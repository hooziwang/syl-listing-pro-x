package rules

import (
	"context"
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

func TestServiceValidateWorkflowNodesRequired(t *testing.T) {
	root := t.TempDir()
	writeTenantFixture(t, root, "demo")
	writeFile(t, filepath.Join(root, "tenants", "demo", "rules", "workflow.yaml"), `planning:
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
	err := svc.Validate("demo")
	if err == nil {
		t.Fatal("Validate() expected error")
	}
	if !strings.Contains(err.Error(), "workflow.yaml 缺少字段: nodes") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestServiceValidateWorkflowNodeDependsOnExistingNode(t *testing.T) {
	root := t.TempDir()
	writeTenantFixture(t, root, "demo")
	writeFile(t, filepath.Join(root, "tenants", "demo", "rules", "workflow.yaml"), `planning:
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
nodes:
  - id: title_en
    type: generate
    section: title
    output_to: title_en
  - id: render_en
    type: render
    depends_on: [missing_node]
    inputs:
      title_en: title_en
    template: en
    output_to: en_markdown
`)
	svc := Service{Root: root}
	err := svc.Validate("demo")
	if err == nil {
		t.Fatal("Validate() expected error")
	}
	if !strings.Contains(err.Error(), "workflow.nodes[render_en].depends_on 引用了不存在的节点: missing_node") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestServiceValidateWorkflowNodeOutputToMustBeUnique(t *testing.T) {
	root := t.TempDir()
	writeTenantFixture(t, root, "demo")
	writeFile(t, filepath.Join(root, "tenants", "demo", "rules", "workflow.yaml"), `planning:
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
nodes:
  - id: title_en
    type: generate
    section: title
    output_to: shared_slot
  - id: title_cn
    type: translate
    input_from: title_en
    output_to: shared_slot
`)
	svc := Service{Root: root}
	err := svc.Validate("demo")
	if err == nil {
		t.Fatal("Validate() expected error")
	}
	if !strings.Contains(err.Error(), "workflow.nodes[title_cn].output_to 与 [title_en] 冲突: shared_slot") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestServiceValidateRenderNodeInputsRequired(t *testing.T) {
	root := t.TempDir()
	writeTenantFixture(t, root, "demo")
	writeFile(t, filepath.Join(root, "tenants", "demo", "rules", "workflow.yaml"), `planning:
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
nodes:
  - id: title_en
    type: generate
    section: title
    output_to: title_en
  - id: render_en
    type: render
    depends_on: [title_en]
    template: en
    output_to: en_markdown
`)
	svc := Service{Root: root}
	err := svc.Validate("demo")
	if err == nil {
		t.Fatal("Validate() expected error")
	}
	if !strings.Contains(err.Error(), "workflow.nodes[render_en].inputs 非法") {
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
nodes:
  - id: category_cn
    type: translate
    input_from: category
    output_to: category_cn
  - id: keywords_cn
    type: translate
    input_from: keywords
    output_to: keywords_cn
  - id: title_en
    type: generate
    section: title
    output_to: title_en
  - id: bullets_en
    type: generate
    section: bullets
    output_to: bullets_en
  - id: description_en
    type: generate
    section: description
    output_to: description_en
  - id: search_terms_en
    type: derive
    section: search_terms
    output_to: search_terms_en
  - id: quality_judge
    type: judge
    depends_on: [title_en, bullets_en, description_en, search_terms_en]
    inputs:
      title: title_en
      bullets: bullets_en
      description: description_en
      search_terms: search_terms_en
    output_to: judge_report_1
  - id: title_cn
    type: translate
    depends_on: [title_en]
    input_from: title_en
    output_to: title_cn
  - id: bullets_cn
    type: translate
    depends_on: [bullets_en]
    input_from: bullets_en
    output_to: bullets_cn
  - id: description_cn
    type: translate
    depends_on: [description_en]
    input_from: description_en
    output_to: description_cn
  - id: search_terms_cn
    type: translate
    depends_on: [search_terms_en]
    input_from: search_terms_en
    output_to: search_terms_cn
  - id: render_en
    type: render
    depends_on: [title_en, bullets_en, description_en, search_terms_en]
    inputs:
      brand: brand
      category_en: category
      keywords_en: keywords
      title_en: title_en
      bullets_en: bullets_en
      description_en: description_en
      search_terms_en: search_terms_en
    template: en
    output_to: en_markdown
  - id: render_cn
    type: render
    depends_on: [category_cn, keywords_cn, title_cn, bullets_cn, description_cn, search_terms_cn]
    inputs:
      brand: brand
      category_cn: category_cn
      keywords_cn: keywords_cn
      title_cn: title_cn
      bullets_cn: bullets_cn
      description_cn: description_cn
      search_terms_cn: search_terms_cn
    template: cn
    output_to: cn_markdown
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
