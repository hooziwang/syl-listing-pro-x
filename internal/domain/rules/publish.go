package rules

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (s Service) Publish(ctx context.Context, in PublishInput) (PublishResponse, error) {
	if strings.TrimSpace(in.Tenant) == "" {
		return PublishResponse{}, fmt.Errorf("缺少 tenant")
	}
	if strings.TrimSpace(in.Version) == "" {
		return PublishResponse{}, fmt.Errorf("缺少 rules version")
	}
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
	if manifest.TenantID != in.Tenant || manifest.RulesVersion != in.Version {
		return PublishResponse{}, fmt.Errorf("manifest 与发布参数不一致: tenant=%s version=%s", manifest.TenantID, manifest.RulesVersion)
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
