package rules

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

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
brand:
  labels: ["品牌名"]
  fallback: UnknownBrand
keywords:
  heading_aliases: ["关键词库"]
category:
  heading_aliases: ["分类"]
`)
	writeFile(t, filepath.Join(base, "workflow.yaml"), `planning:
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
