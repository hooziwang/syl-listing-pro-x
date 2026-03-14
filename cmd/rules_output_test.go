package cmd

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRulesPackageCmdKeepsStderrQuietByDefault(t *testing.T) {
	withPathsForTest(t, func() {
		root := t.TempDir()
		writeTenantFixtureForRulesCmdTest(t, root, "demo")
		keyPath := generatePrivateKeyForRulesCmdTest(t, root)
		paths.WorkspaceRoot = "/tmp/workspace"
		paths.RulesRepo = root

		cmd := newRulesPackageCmd()
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetArgs([]string{
			"--tenant", "demo",
			"--version", "rules-demo-20260314-000000-abcd12",
			"--private-key", keyPath,
		})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		wantPackageDir := filepath.Join(root, "dist", "demo", "rules-demo-20260314-000000-abcd12")
		if got := stdout.String(); got != wantPackageDir+"\n" {
			t.Fatalf("stdout = %q, want %q", got, wantPackageDir+"\\n")
		}
		if got := stderr.String(); got != "" {
			t.Fatalf("stderr = %q, want empty", got)
		}
	})
}

func TestRulesPackageCmdPrintsPathContextWhenRequested(t *testing.T) {
	withPathsForTest(t, func() {
		root := t.TempDir()
		writeTenantFixtureForRulesCmdTest(t, root, "demo")
		keyPath := generatePrivateKeyForRulesCmdTest(t, root)
		paths.WorkspaceRoot = "/tmp/workspace"
		paths.RulesRepo = root

		cmd := newRulesPackageCmd()
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetArgs([]string{
			"--tenant", "demo",
			"--version", "rules-demo-20260314-000000-abcd12",
			"--private-key", keyPath,
			"--print-path-context",
		})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		wantPackageDir := filepath.Join(root, "dist", "demo", "rules-demo-20260314-000000-abcd12")
		if got := stdout.String(); got != wantPackageDir+"\n" {
			t.Fatalf("stdout = %q, want %q", got, wantPackageDir+"\\n")
		}
		for _, part := range []string{
			"[rules package] 路径上下文",
			"WorkspaceRoot=/tmp/workspace",
			fmt.Sprintf("RulesRepo=%s", root),
			fmt.Sprintf("PackageDir=%s", wantPackageDir),
		} {
			if !strings.Contains(stderr.String(), part) {
				t.Fatalf("stderr missing %q\nstderr:\n%s", part, stderr.String())
			}
		}
	})
}

func TestRulesPublishCmdKeepsStderrQuietByDefault(t *testing.T) {
	withPathsForTest(t, func() {
		root := t.TempDir()
		writeTenantFixtureForRulesCmdTest(t, root, "demo")
		keyPath := generatePrivateKeyForRulesCmdTest(t, root)
		paths.WorkspaceRoot = "/tmp/workspace"
		paths.RulesRepo = root

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got, want := r.URL.Path, "/v1/admin/tenant-rules/publish"; got != want {
				t.Fatalf("path = %q, want %q", got, want)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"rules_version":"rules-demo-20260314-000000-abcd12"}`))
		}))
		defer server.Close()

		cmd := newRulesPublishCmd()
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetArgs([]string{
			"--tenant", "demo",
			"--version", "rules-demo-20260314-000000-abcd12",
			"--private-key", keyPath,
			"--worker", server.URL,
			"--admin-token", "demo-token",
		})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		if got := stdout.String(); got != "rules-demo-20260314-000000-abcd12\n" {
			t.Fatalf("stdout = %q", got)
		}
		if got := stderr.String(); got != "" {
			t.Fatalf("stderr = %q, want empty", got)
		}
	})
}

func TestRulesPublishCmdPrintsPathContextWhenRequested(t *testing.T) {
	withPathsForTest(t, func() {
		root := t.TempDir()
		writeTenantFixtureForRulesCmdTest(t, root, "demo")
		keyPath := generatePrivateKeyForRulesCmdTest(t, root)
		paths.WorkspaceRoot = "/tmp/workspace"
		paths.RulesRepo = root

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got, want := r.URL.Path, "/v1/admin/tenant-rules/publish"; got != want {
				t.Fatalf("path = %q, want %q", got, want)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"rules_version":"rules-demo-20260314-000000-abcd12"}`))
		}))
		defer server.Close()

		cmd := newRulesPublishCmd()
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetArgs([]string{
			"--tenant", "demo",
			"--version", "rules-demo-20260314-000000-abcd12",
			"--private-key", keyPath,
			"--worker", server.URL,
			"--admin-token", "demo-token",
			"--print-path-context",
		})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		wantPackageDir := filepath.Join(root, "dist", "demo", "rules-demo-20260314-000000-abcd12")
		if got := stdout.String(); got != "rules-demo-20260314-000000-abcd12\n" {
			t.Fatalf("stdout = %q", got)
		}
		for _, part := range []string{
			"[rules publish] 路径上下文",
			"WorkspaceRoot=/tmp/workspace",
			fmt.Sprintf("RulesRepo=%s", root),
			fmt.Sprintf("PackageDir=%s", wantPackageDir),
			"RulesVersion=rules-demo-20260314-000000-abcd12",
		} {
			if !strings.Contains(stderr.String(), part) {
				t.Fatalf("stderr missing %q\nstderr:\n%s", part, stderr.String())
			}
		}
	})
}

func writeTenantFixtureForRulesCmdTest(t *testing.T, root, tenant string) {
	t.Helper()
	base := filepath.Join(root, "tenants", tenant, "rules")
	mkdirAllForRulesCmdTest(t, filepath.Join(base, "sections"))
	mkdirAllForRulesCmdTest(t, filepath.Join(base, "templates"))

	writeFileForRulesCmdTest(t, filepath.Join(base, "package.yaml"), `required_sections:
  - title
  - bullets
  - description
  - search_terms
  - translation
templates:
  en: templates/en.md.tmpl
  cn: templates/cn.md.tmpl
`)
	writeFileForRulesCmdTest(t, filepath.Join(base, "input.yaml"), `file_discovery:
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
	writeFileForRulesCmdTest(t, filepath.Join(base, "generation-config.yaml"), `planning:
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
	writeFileForRulesCmdTest(t, filepath.Join(base, "templates", "en.md.tmpl"), "# EN\n{{title_en}}\n")
	writeFileForRulesCmdTest(t, filepath.Join(base, "templates", "cn.md.tmpl"), "# CN\n{{title_cn}}\n")

	for _, name := range []string{"title", "bullets", "description", "search_terms", "translation"} {
		writeFileForRulesCmdTest(t, filepath.Join(base, "sections", name+".yaml"), "section: "+name+"\nlanguage: en\ninstruction: ok\nconstraints: {}\nexecution:\n  retries: 2\noutput:\n  format: text\n")
	}
}

func generatePrivateKeyForRulesCmdTest(t *testing.T, root string) string {
	t.Helper()
	keyPath := filepath.Join(root, "rules_private.pem")
	cmd := exec.Command("openssl", "genrsa", "-out", keyPath, "2048")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generate private key: %v output=%s", err, string(out))
	}
	return keyPath
}

func mkdirAllForRulesCmdTest(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func writeFileForRulesCmdTest(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
