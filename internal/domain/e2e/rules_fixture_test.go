package e2e

import (
	"os"
	"path/filepath"
	"testing"
)

func writeRulesFixtureRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	rulesDir := filepath.Join(root, "tenants", "syl", "rules")
	mkdirAll(t, filepath.Join(rulesDir, "sections"))
	mkdirAll(t, filepath.Join(rulesDir, "templates"))

	writeFile(t, filepath.Join(rulesDir, "package.yaml"), `required_sections:
  - title
  - bullets
  - description
  - search_terms
  - translation
templates:
  en: templates/en.md.tmpl
  cn: templates/cn.md.tmpl
`)
	writeFile(t, filepath.Join(rulesDir, "templates", "en.md.tmpl"), `# Amazon Listing (EN)

## 分类
{{category_en}}

## 关键词
{{keywords_en}}

## 标题
{{title_en}}

## 五点描述
{{bullets_en}}

## 产品描述
{{description_en}}

## 搜索词
{{search_terms_en}}
`)
	writeFile(t, filepath.Join(rulesDir, "templates", "cn.md.tmpl"), `# 亚马逊 Listing (CN)

## 分类
{{category_cn}}

## 关键词
{{keywords_cn}}

## 标题
{{title_cn}}

## 五点描述
{{bullets_cn}}

## 产品描述
{{description_cn}}

## 搜索词
{{search_terms_cn}}
`)
	writeFile(t, filepath.Join(rulesDir, "sections", "title.yaml"), `section: title
constraints:
  min_chars: 100
  max_chars: 200
`)
	writeFile(t, filepath.Join(rulesDir, "sections", "bullets.yaml"), `section: bullets
constraints:
  line_count: 5
  min_chars_per_line: 240
  max_chars_per_line: 300
  heading_min_words: 2
  heading_max_words: 4
  keyword_embedding:
    lowercase: true
`)
	writeFile(t, filepath.Join(rulesDir, "sections", "description.yaml"), `section: description
constraints:
  min_paragraphs: 2
  max_paragraphs: 2
`)
	writeFile(t, filepath.Join(rulesDir, "sections", "search_terms.yaml"), `section: search_terms
constraints:
  lowercase: true
`)
	writeFile(t, filepath.Join(rulesDir, "sections", "translation.yaml"), `section: translation
constraints: {}
`)
	return root
}

func mkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
