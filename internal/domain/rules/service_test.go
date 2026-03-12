package rules

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestGenerateVersionFormat(t *testing.T) {
	version := GenerateVersion("syl")
	pattern := regexp.MustCompile(`^rules-syl-\d{8}-\d{6}-[0-9a-z]{6}$`)
	if !pattern.MatchString(version) {
		t.Fatalf("version format invalid: %q", version)
	}
	suffix := version[strings.LastIndex(version, "-")+1:]
	if !regexp.MustCompile(`[a-z]`).MatchString(suffix) {
		t.Fatalf("version suffix should contain letters, got %q", version)
	}
}

func TestServiceValidate(t *testing.T) {
	root := t.TempDir()
	writeTenantFixture(t, root, "demo")

	svc := Service{Root: root}
	if err := svc.Validate("demo"); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestServicePackage(t *testing.T) {
	root := t.TempDir()
	writeTenantFixture(t, root, "demo")
	keyPath := generatePrivateKey(t, root)

	svc := Service{Root: root}
	out, err := svc.Package("demo", "rules-demo-20260310-000000-aaaaaa", keyPath)
	if err != nil {
		t.Fatalf("Package() error = %v", err)
	}
	if _, err := os.Stat(out.ArchivePath); err != nil {
		t.Fatalf("archive missing: %v", err)
	}
	if _, err := os.Stat(out.ManifestPath); err != nil {
		t.Fatalf("manifest missing: %v", err)
	}
	data, err := os.ReadFile(out.ManifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}
	if manifest.TenantID != "demo" {
		t.Fatalf("manifest tenant = %q", manifest.TenantID)
	}
	if manifest.RulesVersion != "rules-demo-20260310-000000-aaaaaa" {
		t.Fatalf("manifest version = %q", manifest.RulesVersion)
	}
	if manifest.SignatureBase64 == "" {
		t.Fatal("manifest signature empty")
	}
	if manifest.SigningPublicKeyPathInArchive != "tenant/rules_signing_public.pem" {
		t.Fatalf("manifest signing key path = %q", manifest.SigningPublicKeyPathInArchive)
	}
}

func TestServicePackageRejectsBundledDevKeyWithoutExplicitOptIn(t *testing.T) {
	root := t.TempDir()
	writeTenantFixture(t, root, "demo")
	keyPath := filepath.Join(root, "keys", "rules_private.pem")
	mkdirAll(t, filepath.Dir(keyPath))
	writeFile(t, keyPath, mustReadFile(t, generatePrivateKey(t, root)))

	svc := Service{Root: root}
	_, err := svc.Package("demo", "rules-demo-20260310-000000-devkey", keyPath)
	if err == nil {
		t.Fatal("Package() expected error")
	}
	if !strings.Contains(err.Error(), "开发模式") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestServicePackageRejectsBundledDevKeySymlinkWithoutExplicitOptIn(t *testing.T) {
	root := t.TempDir()
	writeTenantFixture(t, root, "demo")
	keyPath := filepath.Join(root, "keys", "rules_private.pem")
	mkdirAll(t, filepath.Dir(keyPath))
	writeFile(t, keyPath, mustReadFile(t, generatePrivateKey(t, root)))

	symlinkPath := filepath.Join(root, "external-rules-private.pem")
	if err := os.Symlink(keyPath, symlinkPath); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	svc := Service{Root: root}
	_, err := svc.Package("demo", "rules-demo-20260310-000000-devlink", symlinkPath)
	if err == nil {
		t.Fatal("Package() expected error")
	}
	if !strings.Contains(err.Error(), "开发模式") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestServicePackageAllowsBundledDevKeyWithExplicitOptIn(t *testing.T) {
	root := t.TempDir()
	writeTenantFixture(t, root, "demo")
	keyPath := filepath.Join(root, "keys", "rules_private.pem")
	mkdirAll(t, filepath.Dir(keyPath))
	writeFile(t, keyPath, mustReadFile(t, generatePrivateKey(t, root)))
	t.Setenv("SYL_LISTING_ALLOW_DEV_PRIVATE_KEY", "1")

	svc := Service{Root: root}
	out, err := svc.Package("demo", "rules-demo-20260310-000000-devoptin", keyPath)
	if err != nil {
		t.Fatalf("Package() error = %v", err)
	}
	if _, err := os.Stat(out.ManifestPath); err != nil {
		t.Fatalf("manifest missing: %v", err)
	}
}

func TestServicePackageRejectsBundledDevKeyInCI(t *testing.T) {
	root := t.TempDir()
	writeTenantFixture(t, root, "demo")
	keyPath := filepath.Join(root, "keys", "rules_private.pem")
	mkdirAll(t, filepath.Dir(keyPath))
	writeFile(t, keyPath, mustReadFile(t, generatePrivateKey(t, root)))
	t.Setenv("SYL_LISTING_ALLOW_DEV_PRIVATE_KEY", "1")
	t.Setenv("GITHUB_ACTIONS", "true")

	svc := Service{Root: root}
	_, err := svc.Package("demo", "rules-demo-20260310-000000-devci", keyPath)
	if err == nil {
		t.Fatal("Package() expected error")
	}
	if !strings.Contains(err.Error(), "GitHub Actions / CI") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestServicePackageUsesPrivateKeyFromEnv(t *testing.T) {
	root := t.TempDir()
	writeTenantFixture(t, root, "demo")
	keyPath := generatePrivateKey(t, root)
	t.Setenv("SYL_LISTING_RULES_PRIVATE_KEY", keyPath)

	svc := Service{Root: root}
	out, err := svc.Package("demo", "rules-demo-20260310-000000-envkey", "")
	if err != nil {
		t.Fatalf("Package() error = %v", err)
	}
	if _, err := os.Stat(out.ManifestPath); err != nil {
		t.Fatalf("manifest missing: %v", err)
	}
}

func TestServicePackageUsesPrivateKeyPEMFromEnv(t *testing.T) {
	root := t.TempDir()
	writeTenantFixture(t, root, "demo")
	keyPath := generatePrivateKey(t, root)
	t.Setenv("SIGNING_PRIVATE_KEY_PEM", mustReadFile(t, keyPath))

	svc := Service{Root: root}
	out, err := svc.Package("demo", "rules-demo-20260310-000000-envpem", "")
	if err != nil {
		t.Fatalf("Package() error = %v", err)
	}
	if _, err := os.Stat(out.ManifestPath); err != nil {
		t.Fatalf("manifest missing: %v", err)
	}
}

func TestServicePackageUsesPrivateKeyBase64FromEnv(t *testing.T) {
	root := t.TempDir()
	writeTenantFixture(t, root, "demo")
	keyPath := generatePrivateKey(t, root)
	t.Setenv("SIGNING_PRIVATE_KEY_BASE64", base64.StdEncoding.EncodeToString([]byte(mustReadFile(t, keyPath))))

	svc := Service{Root: root}
	out, err := svc.Package("demo", "rules-demo-20260310-000000-envb64", "")
	if err != nil {
		t.Fatalf("Package() error = %v", err)
	}
	if _, err := os.Stat(out.ManifestPath); err != nil {
		t.Fatalf("manifest missing: %v", err)
	}
}

func TestServicePackageMissingPrivateKeyMessageIncludesResolutionOrder(t *testing.T) {
	root := t.TempDir()
	writeTenantFixture(t, root, "demo")

	svc := Service{Root: root}
	_, err := svc.Package("demo", "rules-demo-20260310-000000-nokey", "")
	if err == nil {
		t.Fatal("Package() expected error")
	}
	for _, part := range []string{
		"--private-key",
		"SYL_LISTING_RULES_PRIVATE_KEY",
		"SIGNING_PRIVATE_KEY_PEM",
		"SIGNING_PRIVATE_KEY_BASE64",
		"本地开发模式",
	} {
		if !strings.Contains(err.Error(), part) {
			t.Fatalf("error %q missing %q", err, part)
		}
	}
}

func TestServicePublish(t *testing.T) {
	root := t.TempDir()
	writeTenantFixture(t, root, "demo")
	keyPath := generatePrivateKey(t, root)

	svc := Service{Root: root}
	pkg, err := svc.Package("demo", "rules-demo-20260310-000001-bbbbbb", keyPath)
	if err != nil {
		t.Fatalf("Package() error = %v", err)
	}

	var gotAuth string
	var gotPayload publishPayload
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if r.Method != http.MethodPost {
			t.Fatalf("method=%s", r.Method)
		}
		if r.URL.Path != "/v1/admin/tenant-rules/publish" {
			t.Fatalf("path=%s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"tenant_id":"demo","rules_version":"rules-demo-20260310-000001-bbbbbb"}`))
	}))
	defer ts.Close()

	resp, err := svc.Publish(context.Background(), PublishInput{
		Tenant:     "demo",
		Version:    "rules-demo-20260310-000001-bbbbbb",
		WorkerURL:  ts.URL,
		AdminToken: "token-123",
	})
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if !resp.OK {
		t.Fatalf("Publish() ok=false")
	}
	if gotAuth != "Bearer token-123" {
		t.Fatalf("Authorization=%q", gotAuth)
	}
	if gotPayload.TenantID != "demo" {
		t.Fatalf("payload tenant=%q", gotPayload.TenantID)
	}
	if gotPayload.ManifestSHA256 == "" || gotPayload.ArchiveBase64 == "" {
		t.Fatalf("payload missing required fields: %+v", gotPayload)
	}
	if pkg.ManifestPath == "" {
		t.Fatal("package output should not be empty")
	}
}

func TestServicePublishRejectsManifestTenantVersionMismatch(t *testing.T) {
	root := t.TempDir()
	writeTenantFixture(t, root, "demo")
	keyPath := generatePrivateKey(t, root)

	svc := Service{Root: root}
	pkg, err := svc.Package("demo", "rules-demo-20260310-000002-cccccc", keyPath)
	if err != nil {
		t.Fatalf("Package() error = %v", err)
	}

	data, err := os.ReadFile(pkg.ManifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}
	manifest.RulesVersion = "rules-demo-20260310-999999-wrong"
	updated, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	if err := os.WriteFile(pkg.ManifestPath, updated, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	var called bool
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"tenant_id":"demo","rules_version":"rules-demo-20260310-000002-cccccc"}`))
	}))
	defer ts.Close()

	_, err = svc.Publish(context.Background(), PublishInput{
		Tenant:     "demo",
		Version:    "rules-demo-20260310-000002-cccccc",
		WorkerURL:  ts.URL,
		AdminToken: "token-123",
	})
	if err == nil {
		t.Fatal("Publish() expected error")
	}
	if !strings.Contains(err.Error(), "manifest") {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Fatal("Publish() should fail before remote request")
	}
}

func TestServiceValidateMissingSectionFails(t *testing.T) {
	root := t.TempDir()
	writeTenantFixture(t, root, "demo")
	if err := os.Remove(filepath.Join(root, "tenants", "demo", "rules", "sections", "description.yaml")); err != nil {
		t.Fatalf("remove section: %v", err)
	}
	svc := Service{Root: root}
	err := svc.Validate("demo")
	if err == nil {
		t.Fatal("Validate() expected error")
	}
	if !strings.Contains(err.Error(), "缺少 section") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestServiceValidateGenerationConfigWithoutNodesPasses(t *testing.T) {
	root := t.TempDir()
	writeTenantFixture(t, root, "demo")
	writeFile(t, filepath.Join(root, "tenants", "demo", "rules", "generation-config.yaml"), `planning:
  system_prompt: p
  user_prompt: p
judge:
  system_prompt: j
  user_prompt: j
  ignore_messages: ["OK"]
  skip_sections: ["search_terms"]
translation:
  system_prompt: t
render:
  keywords_item_template: "{{item}}"
  bullets_item_template: "{{item}}"
  bullets_separator: "\n\n"
display_labels:
  title: 标题
  bullets: 五点描述
  description: 产品描述
  search_terms: 搜索词
  category: 分类
  keywords: 关键词
`)
	svc := Service{Root: root}
	if err := svc.Validate("demo"); err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}
}

func writeTenantFixture(t *testing.T, root, tenant string) {
	t.Helper()
	base := filepath.Join(root, "tenants", tenant, "rules")
	mkdirAll(t, filepath.Join(base, "sections"))
	mkdirAll(t, filepath.Join(base, "templates"))

	writeFile(t, filepath.Join(base, "package.yaml"), `required_sections:
  - title
  - bullets
  - description
  - search_terms
  - translation
templates:
  en: templates/en.md.tmpl
  cn: templates/cn.md.tmpl
`)
	writeFile(t, filepath.Join(base, "input.yaml"), `file_discovery:
  marker: ===Listing Requirements===
fields:
  - key: brand
    type: scalar
    capture: inline_label
    labels: ["品牌名"]
    fallback: UnknownBrand
    fallback_from_h1_first_token: true
  - key: keywords
    type: list
    capture: heading_section
    heading_aliases: ["关键词库"]
    min_count: 15
    unique_required: true
  - key: category
    type: scalar
    capture: heading_section
    heading_aliases: ["分类"]
`)
	writeFile(t, filepath.Join(base, "generation-config.yaml"), `planning:
  system_prompt: p
  user_prompt: p
judge:
  system_prompt: j
  user_prompt: j
  ignore_messages: ["OK"]
  skip_sections: ["search_terms"]
translation:
  system_prompt: t
render:
  keywords_item_template: "{{item}}"
  bullets_item_template: "{{item}}"
  bullets_separator: "\n\n"
display_labels:
  title: 标题
  bullets: 五点描述
  description: 产品描述
  search_terms: 搜索词
  category: 分类
  keywords: 关键词
`)
	writeFile(t, filepath.Join(base, "templates", "en.md.tmpl"), "# EN\n{{title_en}}\n")
	writeFile(t, filepath.Join(base, "templates", "cn.md.tmpl"), "# CN\n{{title_cn}}\n")

	for _, name := range []string{"title", "bullets", "description", "search_terms", "translation"} {
		writeFile(t, filepath.Join(base, "sections", name+".yaml"), "section: "+name+"\nlanguage: en\ninstruction: ok\nconstraints: {}\nexecution:\n  retries: 2\noutput:\n  format: text\n")
	}
}

func generatePrivateKey(t *testing.T, root string) string {
	t.Helper()
	keyPath := filepath.Join(root, "rules_private.pem")
	cmd := exec.Command("openssl", "genrsa", "-out", keyPath, "2048")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generate private key: %v output=%s", err, string(out))
	}
	return keyPath
}

func mkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func mustReadFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
