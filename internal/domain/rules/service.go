package rules

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Service struct {
	Root       string
	HTTPClient *http.Client
}

type PackageOutput struct {
	PackageDir   string
	ArchivePath  string
	ManifestPath string
}

type Manifest struct {
	TenantID                        string `json:"tenant_id"`
	RulesVersion                    string `json:"rules_version"`
	ManifestSHA256                  string `json:"manifest_sha256"`
	SignatureBase64                 string `json:"signature_base64"`
	SignatureAlgo                   string `json:"signature_algo"`
	SigningPublicKeyPathInArchive   string `json:"signing_public_key_path_in_archive"`
	SigningPublicKeySignatureBase64 string `json:"signing_public_key_signature_base64"`
	SigningPublicKeySignatureAlgo   string `json:"signing_public_key_signature_algo"`
	Archive                         string `json:"archive"`
}

type PublishInput struct {
	Tenant     string
	Version    string
	WorkerURL  string
	AdminToken string
}

type PublishResponse struct {
	OK           bool   `json:"ok"`
	TenantID     string `json:"tenant_id"`
	RulesVersion string `json:"rules_version"`
}

type publishPayload struct {
	TenantID                        string `json:"tenant_id"`
	RulesVersion                    string `json:"rules_version"`
	ManifestSHA256                  string `json:"manifest_sha256"`
	ArchiveBase64                   string `json:"archive_base64"`
	SignatureBase64                 string `json:"signature_base64"`
	SignatureAlgo                   string `json:"signature_algo"`
	SigningPublicKeyPathInArchive   string `json:"signing_public_key_path_in_archive"`
	SigningPublicKeySignatureBase64 string `json:"signing_public_key_signature_base64"`
	SigningPublicKeySignatureAlgo   string `json:"signing_public_key_signature_algo"`
}

func GenerateVersion(tenant string) string {
	now := time.Now().UTC()
	return fmt.Sprintf("rules-%s-%s-%06d", normalizeTenant(tenant), now.Format("20060102-150405"), now.Nanosecond()%1_000_000)
}

func (s Service) Validate(tenant string) error {
	rulesDir := filepath.Join(s.Root, "tenants", tenant, "rules")
	if _, err := os.Stat(rulesDir); err != nil {
		return fmt.Errorf("rules 目录不存在: %s", rulesDir)
	}

	packageDoc, err := readYAMLMap(filepath.Join(rulesDir, "package.yaml"))
	if err != nil {
		return err
	}
	inputDoc, err := readYAMLMap(filepath.Join(rulesDir, "input.yaml"))
	if err != nil {
		return err
	}
	workflowDoc, err := readYAMLMap(filepath.Join(rulesDir, "workflow.yaml"))
	if err != nil {
		return err
	}

	if err := requireMapKeys(packageDoc, "package.yaml", "required_sections", "templates"); err != nil {
		return err
	}
	if err := requireMapKeys(inputDoc, "input.yaml", "file_discovery", "brand", "keywords", "category"); err != nil {
		return err
	}
	if err := requireNestedKeys(inputDoc, "input.yaml", "file_discovery", "marker"); err != nil {
		return err
	}
	if err := requireNestedKeys(inputDoc, "input.yaml", "brand", "labels", "fallback"); err != nil {
		return err
	}
	if err := requireNestedKeys(inputDoc, "input.yaml", "keywords", "heading_aliases"); err != nil {
		return err
	}
	if err := requireNestedKeys(inputDoc, "input.yaml", "category", "heading_aliases"); err != nil {
		return err
	}
	if err := requireMapKeys(workflowDoc, "workflow.yaml", "planning", "judge", "translation", "render", "display_labels"); err != nil {
		return err
	}
	if err := requireNestedKeys(workflowDoc, "workflow.yaml", "planning", "system_prompt", "user_prompt"); err != nil {
		return err
	}
	if err := requireNestedKeys(workflowDoc, "workflow.yaml", "judge", "system_prompt", "user_prompt", "ignore_messages", "skip_sections"); err != nil {
		return err
	}
	if err := requireNestedKeys(workflowDoc, "workflow.yaml", "translation", "system_prompt"); err != nil {
		return err
	}
	if err := requireNestedKeys(workflowDoc, "workflow.yaml", "render", "keywords_item_template", "bullets_item_template", "bullets_separator"); err != nil {
		return err
	}
	if err := requireNestedKeys(workflowDoc, "workflow.yaml", "display_labels", "title", "bullets", "description", "search_terms", "category", "keywords"); err != nil {
		return err
	}

	requiredSections, err := stringListFromMap(packageDoc, "required_sections", "package.yaml")
	if err != nil {
		return err
	}
	templates, ok := packageDoc["templates"].(map[string]any)
	if !ok {
		return fmt.Errorf("package.yaml templates 非法")
	}
	for _, key := range []string{"en", "cn"} {
		path, _ := templates[key].(string)
		if path == "" {
			return fmt.Errorf("package.yaml templates.%s 非法", key)
		}
		if _, err := os.Stat(filepath.Join(rulesDir, path)); err != nil {
			return fmt.Errorf("模板文件不存在: %s", filepath.Join(rulesDir, path))
		}
	}

	sectionDir := filepath.Join(rulesDir, "sections")
	entries, err := os.ReadDir(sectionDir)
	if err != nil {
		return fmt.Errorf("sections 目录不存在: %s", sectionDir)
	}
	found := map[string]struct{}{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		path := filepath.Join(sectionDir, entry.Name())
		doc, err := readYAMLMap(path)
		if err != nil {
			return err
		}
		if err := requireMapKeys(doc, filepath.Base(path), "section", "language", "instruction", "constraints", "execution", "output"); err != nil {
			return err
		}
		section, _ := doc["section"].(string)
		if strings.TrimSpace(section) == "" {
			return fmt.Errorf("%s instruction 不能为空", path)
		}
		if err := requireNestedKeys(doc, filepath.Base(path), "execution", "retries"); err != nil {
			return err
		}
		found[section] = struct{}{}
	}

	missing := make([]string, 0)
	for _, section := range requiredSections {
		if _, ok := found[section]; !ok {
			missing = append(missing, section)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("缺少 section: %v", missing)
	}
	return nil
}

func (s Service) Package(tenant, version, privateKeyPath string) (PackageOutput, error) {
	if err := s.Validate(tenant); err != nil {
		return PackageOutput{}, err
	}
	if strings.TrimSpace(privateKeyPath) == "" {
		return PackageOutput{}, fmt.Errorf("缺少私钥：请传 --private-key")
	}
	if _, err := os.Stat(privateKeyPath); err != nil {
		return PackageOutput{}, fmt.Errorf("签名私钥不存在: %s", privateKeyPath)
	}

	src := filepath.Join(s.Root, "tenants", tenant)
	outDir := filepath.Join(s.Root, "dist", tenant, version)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return PackageOutput{}, err
	}

	publicKeyPath := filepath.Join(outDir, "rules_signing_public.pem")
	if err := runOpenSSL("pkey", "-in", privateKeyPath, "-pubout", "-out", publicKeyPath); err != nil {
		return PackageOutput{}, err
	}

	archivePath := filepath.Join(outDir, "rules.tar.gz")
	if err := createTarGz(archivePath, map[string]string{
		src:           "tenant",
		publicKeyPath: "tenant/rules_signing_public.pem",
	}); err != nil {
		return PackageOutput{}, err
	}

	archiveSHA, err := sha256File(archivePath)
	if err != nil {
		return PackageOutput{}, err
	}
	signature, err := signFile(privateKeyPath, archivePath)
	if err != nil {
		return PackageOutput{}, err
	}
	publicKeySig, err := signFile(privateKeyPath, publicKeyPath)
	if err != nil {
		return PackageOutput{}, err
	}

	manifest := Manifest{
		TenantID:                        tenant,
		RulesVersion:                    version,
		ManifestSHA256:                  archiveSHA,
		SignatureBase64:                 signature,
		SignatureAlgo:                   "rsa-sha256",
		SigningPublicKeyPathInArchive:   "tenant/rules_signing_public.pem",
		SigningPublicKeySignatureBase64: publicKeySig,
		SigningPublicKeySignatureAlgo:   "rsa-sha256",
		Archive:                         "rules.tar.gz",
	}
	manifestPath := filepath.Join(outDir, "manifest.json")
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return PackageOutput{}, err
	}
	if err := os.WriteFile(manifestPath, data, 0o644); err != nil {
		return PackageOutput{}, err
	}

	return PackageOutput{
		PackageDir:   outDir,
		ArchivePath:  archivePath,
		ManifestPath: manifestPath,
	}, nil
}

func (s Service) Publish(ctx context.Context, in PublishInput) (PublishResponse, error) {
	if strings.TrimSpace(in.WorkerURL) == "" {
		return PublishResponse{}, fmt.Errorf("缺少 worker 地址")
	}
	if strings.TrimSpace(in.AdminToken) == "" {
		return PublishResponse{}, fmt.Errorf("缺少 ADMIN_TOKEN")
	}
	outDir := filepath.Join(s.Root, "dist", in.Tenant, in.Version)
	manifestPath := filepath.Join(outDir, "manifest.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return PublishResponse{}, err
	}
	var manifest Manifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return PublishResponse{}, err
	}
	archiveData, err := os.ReadFile(filepath.Join(outDir, "rules.tar.gz"))
	if err != nil {
		return PublishResponse{}, err
	}
	payload := publishPayload{
		TenantID:                        manifest.TenantID,
		RulesVersion:                    manifest.RulesVersion,
		ManifestSHA256:                  manifest.ManifestSHA256,
		ArchiveBase64:                   base64.StdEncoding.EncodeToString(archiveData),
		SignatureBase64:                 manifest.SignatureBase64,
		SignatureAlgo:                   manifest.SignatureAlgo,
		SigningPublicKeyPathInArchive:   manifest.SigningPublicKeyPathInArchive,
		SigningPublicKeySignatureBase64: manifest.SigningPublicKeySignatureBase64,
		SigningPublicKeySignatureAlgo:   manifest.SigningPublicKeySignatureAlgo,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return PublishResponse{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(in.WorkerURL, "/")+"/v1/admin/tenant-rules/publish", strings.NewReader(string(body)))
	if err != nil {
		return PublishResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+in.AdminToken)
	resp, err := s.httpClient().Do(req)
	if err != nil {
		return PublishResponse{}, err
	}
	defer resp.Body.Close()
	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return PublishResponse{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return PublishResponse{}, fmt.Errorf("%s", strings.TrimSpace(string(respData)))
	}
	var out PublishResponse
	if err := json.Unmarshal(respData, &out); err != nil {
		return PublishResponse{}, err
	}
	return out, nil
}

func (s Service) httpClient() *http.Client {
	if s.HTTPClient != nil {
		return s.HTTPClient
	}
	return &http.Client{Timeout: 2 * time.Minute}
}

func requireMapKeys(doc map[string]any, file string, keys ...string) error {
	for _, key := range keys {
		if _, ok := doc[key]; !ok {
			return fmt.Errorf("%s 缺少字段: %s", file, key)
		}
	}
	return nil
}

func requireNestedKeys(doc map[string]any, file, key string, keys ...string) error {
	node, ok := doc[key].(map[string]any)
	if !ok {
		return fmt.Errorf("%s %s 结构非法", file, key)
	}
	for _, child := range keys {
		if _, ok := node[child]; !ok {
			return fmt.Errorf("%s 缺少字段: %s.%s", file, key, child)
		}
	}
	return nil
}

func readYAMLMap(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, fmt.Errorf("%s 根节点必须是对象", path)
	}
	return doc, nil
}

func stringListFromMap(doc map[string]any, key, file string) ([]string, error) {
	raw, ok := doc[key].([]any)
	if !ok || len(raw) == 0 {
		return nil, fmt.Errorf("%s %s 非法", file, key)
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		value := strings.TrimSpace(fmt.Sprint(item))
		if value != "" {
			out = append(out, value)
		}
	}
	return out, nil
}

func createTarGz(outPath string, mappings map[string]string) error {
	file, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer file.Close()
	gz := gzip.NewWriter(file)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	keys := make([]string, 0, len(mappings))
	for src := range mappings {
		keys = append(keys, src)
	}
	sort.Strings(keys)

	for _, src := range keys {
		dst := mappings[src]
		if err := filepath.Walk(src, func(path string, info os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			relPath := dst
			if path != src {
				rel, err := filepath.Rel(src, path)
				if err != nil {
					return err
				}
				relPath = filepath.ToSlash(filepath.Join(dst, rel))
			}
			header, err := tar.FileInfoHeader(info, "")
			if err != nil {
				return err
			}
			header.Name = relPath
			if info.IsDir() {
				header.Name += "/"
			}
			if err := tw.WriteHeader(header); err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()
			_, err = io.Copy(tw, f)
			return err
		}); err != nil {
			return err
		}
	}
	return nil
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func signFile(privateKeyPath, path string) (string, error) {
	cmd := exec.Command("openssl", "dgst", "-sha256", "-sign", privateKeyPath, path)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("%s", strings.TrimSpace(string(ee.Stderr)))
		}
		return "", err
	}
	return base64.StdEncoding.EncodeToString(out), nil
}

func runOpenSSL(args ...string) error {
	cmd := exec.Command("openssl", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

func normalizeTenant(raw string) string {
	tenant := strings.ToLower(strings.TrimSpace(raw))
	tenant = regexp.MustCompile(`[^a-z0-9-]+`).ReplaceAllString(tenant, "-")
	tenant = regexp.MustCompile(`-+`).ReplaceAllString(tenant, "-")
	tenant = strings.Trim(tenant, "-")
	if tenant == "" {
		return "tenant"
	}
	return tenant
}
