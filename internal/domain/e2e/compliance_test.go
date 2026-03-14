package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/hooziwang/syl-listing-pro-x/internal/testutil"
)

func TestValidateListingCompliance(t *testing.T) {
	rulesRoot := repoRulesRoot(t)

	t.Run("valid english and chinese markdown pass", func(t *testing.T) {
		enPath, cnPath := writeListingMarkdownPair(t, validEnglishMarkdown(), validChineseMarkdown())
		report, err := validateListingCompliance(rulesRoot, "syl", enPath, cnPath)
		if err != nil {
			t.Fatalf("validateListingCompliance() error = %v", err)
		}
		if !report.Passed {
			t.Fatalf("expected pass, violations=%+v", report.Violations)
		}
	})

	t.Run("missing english section fails", func(t *testing.T) {
		enPath, cnPath := writeListingMarkdownPair(t, strings.Replace(validEnglishMarkdown(), "## 产品描述\n", "", 1), validChineseMarkdown())
		report, err := validateListingCompliance(rulesRoot, "syl", enPath, cnPath)
		if err != nil {
			t.Fatalf("validateListingCompliance() error = %v", err)
		}
		if report.Passed {
			t.Fatal("expected missing section failure")
		}
		assertHasViolation(t, report, "英文", "产品描述")
	})

	t.Run("title length violation fails", func(t *testing.T) {
		enPath, cnPath := writeListingMarkdownPair(t, validEnglishMarkdownWithTitle("short title"), validChineseMarkdown())
		report, err := validateListingCompliance(rulesRoot, "syl", enPath, cnPath)
		if err != nil {
			t.Fatalf("validateListingCompliance() error = %v", err)
		}
		if report.Passed {
			t.Fatal("expected title length failure")
		}
		assertHasViolation(t, report, "英文", "标题")
	})

	t.Run("bullet line length violation fails", func(t *testing.T) {
		enPath, cnPath := writeListingMarkdownPair(t, validEnglishMarkdownWithBullets([]string{
			"short bullet",
			bulletText("Second"),
			bulletText("Third"),
			bulletText("Fourth"),
			bulletText("Fifth"),
		}), validChineseMarkdown())
		report, err := validateListingCompliance(rulesRoot, "syl", enPath, cnPath)
		if err != nil {
			t.Fatalf("validateListingCompliance() error = %v", err)
		}
		if report.Passed {
			t.Fatal("expected bullets failure")
		}
		assertHasViolation(t, report, "英文", "五点描述")
	})

	t.Run("bullet heading word count violation fails", func(t *testing.T) {
		enPath, cnPath := writeListingMarkdownPair(t, validEnglishMarkdownWithBullets([]string{
			"Single: **paper lanterns** keep classroom displays bright and ready for daily use while **hanging decor** adds color and **classroom decoration** supports themed corners, bulletin boards, welcome walls, and seasonal party tables with flexible reuse.",
			bulletText("Second"),
			bulletText("Third"),
			bulletText("Fourth"),
			bulletText("Fifth"),
		}), validChineseMarkdown())
		report, err := validateListingCompliance(rulesRoot, "syl", enPath, cnPath)
		if err != nil {
			t.Fatalf("validateListingCompliance() error = %v", err)
		}
		if report.Passed {
			t.Fatal("expected bullet heading failure")
		}
		assertHasViolation(t, report, "英文", "五点描述")
	})

	t.Run("bullet keyword lowercase violation fails", func(t *testing.T) {
		enPath, cnPath := writeListingMarkdownPair(t, validEnglishMarkdownWithBullets([]string{
			"First Feature: **Paper Lanterns** keep classroom displays bright and ready for daily use while **hanging decor** adds color and **classroom decoration** supports themed corners, bulletin boards, welcome walls, and seasonal party tables with flexible reuse.",
			bulletText("Second"),
			bulletText("Third"),
			bulletText("Fourth"),
			bulletText("Fifth"),
		}), validChineseMarkdown())
		report, err := validateListingCompliance(rulesRoot, "syl", enPath, cnPath)
		if err != nil {
			t.Fatalf("validateListingCompliance() error = %v", err)
		}
		if report.Passed {
			t.Fatal("expected bullet lowercase keyword failure")
		}
		assertHasViolation(t, report, "英文", "五点描述")
	})

	t.Run("description paragraph violation fails", func(t *testing.T) {
		enPath, cnPath := writeListingMarkdownPair(t, validEnglishMarkdownWithDescription(paragraphText("single", 820)), validChineseMarkdown())
		report, err := validateListingCompliance(rulesRoot, "syl", enPath, cnPath)
		if err != nil {
			t.Fatalf("validateListingCompliance() error = %v", err)
		}
		if report.Passed {
			t.Fatal("expected description failure")
		}
		assertHasViolation(t, report, "英文", "产品描述")
	})

	t.Run("search terms lowercase violation fails", func(t *testing.T) {
		enPath, cnPath := writeListingMarkdownPair(t, validEnglishMarkdownWithSearchTerms("Paper Lanterns Classroom Decor Party Supplies wedding decorations hanging decor from ceiling"), validChineseMarkdown())
		report, err := validateListingCompliance(rulesRoot, "syl", enPath, cnPath)
		if err != nil {
			t.Fatalf("validateListingCompliance() error = %v", err)
		}
		if report.Passed {
			t.Fatal("expected search terms failure")
		}
		assertHasViolation(t, report, "英文", "搜索词")
	})

	t.Run("missing chinese section fails", func(t *testing.T) {
		enPath, cnPath := writeListingMarkdownPair(t, validEnglishMarkdown(), strings.Replace(validChineseMarkdown(), "## 搜索词\n", "", 1))
		report, err := validateListingCompliance(rulesRoot, "syl", enPath, cnPath)
		if err != nil {
			t.Fatalf("validateListingCompliance() error = %v", err)
		}
		if report.Passed {
			t.Fatal("expected chinese section failure")
		}
		assertHasViolation(t, report, "中文", "搜索词")
	})
}

func TestRepoRulesRootFromFileUsesGitWorkspaceInsteadOfCurrentCWD(t *testing.T) {
	root := t.TempDir()
	workspaceRoot := filepath.Join(root, "workspace")
	repoRoot := filepath.Join(workspaceRoot, "syl-listing-pro-x")
	rulesRoot := filepath.Join(workspaceRoot, "syl-listing-pro-rules")
	mkdirAllForCompliancePathTest(t, filepath.Join(repoRoot, "internal", "domain", "e2e"))
	mkdirAllForCompliancePathTest(t, rulesRoot)

	runGitForCompliancePathTest(t, repoRoot, "init")
	runGitForCompliancePathTest(t, repoRoot, "config", "user.email", "test@example.com")
	runGitForCompliancePathTest(t, repoRoot, "config", "user.name", "tester")
	writeFileForCompliancePathTest(t, filepath.Join(repoRoot, "README.md"), "repo\n")
	runGitForCompliancePathTest(t, repoRoot, "add", "README.md")
	runGitForCompliancePathTest(t, repoRoot, "commit", "-m", "init")

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldWD)
	})

	filePath := filepath.Join(repoRoot, "internal", "domain", "e2e", "compliance_test.go")
	got := mustFindRulesRootFromFile(t, filePath)
	if got, want := canonicalPathForCompliancePathTest(t, got), canonicalPathForCompliancePathTest(t, rulesRoot); got != want {
		t.Fatalf("repoRulesRootFromFile()=%q want %q", got, rulesRoot)
	}
}

func TestRepoRulesRootFromFileSupportsLinkedWorktree(t *testing.T) {
	root := t.TempDir()
	workspaceRoot := filepath.Join(root, "workspace")
	repoRoot := filepath.Join(workspaceRoot, "syl-listing-pro-x")
	rulesRoot := filepath.Join(workspaceRoot, "syl-listing-pro-rules")
	mkdirAllForCompliancePathTest(t, filepath.Join(repoRoot, "internal", "domain", "e2e"))
	mkdirAllForCompliancePathTest(t, rulesRoot)

	runGitForCompliancePathTest(t, repoRoot, "init")
	runGitForCompliancePathTest(t, repoRoot, "config", "user.email", "test@example.com")
	runGitForCompliancePathTest(t, repoRoot, "config", "user.name", "tester")
	writeFileForCompliancePathTest(t, filepath.Join(repoRoot, "README.md"), "repo\n")
	runGitForCompliancePathTest(t, repoRoot, "add", "README.md")
	runGitForCompliancePathTest(t, repoRoot, "commit", "-m", "init")

	worktreePath := filepath.Join(root, "global-worktrees", "syl-listing-pro-x", "fix-rules-root")
	runGitForCompliancePathTest(t, repoRoot, "worktree", "add", worktreePath, "-b", "fix-rules-root")
	mkdirAllForCompliancePathTest(t, filepath.Join(worktreePath, "internal", "domain", "e2e"))

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldWD)
	})

	filePath := filepath.Join(worktreePath, "internal", "domain", "e2e", "compliance_test.go")
	got := mustFindRulesRootFromFile(t, filePath)
	if got, want := canonicalPathForCompliancePathTest(t, got), canonicalPathForCompliancePathTest(t, rulesRoot); got != want {
		t.Fatalf("repoRulesRootFromFile()=%q want %q", got, rulesRoot)
	}
}

func TestRepoRulesRootFromFileFallsBackToSiblingRepoNamesWithoutGit(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "parent")
	filePath := filepath.Join(parent, "syl-listing-pro-x", "internal", "domain", "e2e", "compliance_test.go")
	rulesRoot := filepath.Join(parent, "syl-listing-pro-rules")
	mkdirAllForCompliancePathTest(t, filepath.Dir(filePath))
	mkdirAllForCompliancePathTest(t, rulesRoot)

	got := mustFindRulesRootFromFile(t, filePath)
	if got, want := canonicalPathForCompliancePathTest(t, got), canonicalPathForCompliancePathTest(t, rulesRoot); got != want {
		t.Fatalf("repoRulesRootFromFile()=%q want %q", got, rulesRoot)
	}
}

func TestRepoRulesRootFromFileFallsBackToLegacyRulesDirWithoutGit(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "parent")
	filePath := filepath.Join(parent, "syl-listing-pro-x", "internal", "domain", "e2e", "compliance_test.go")
	rulesRoot := filepath.Join(parent, "rules")
	mkdirAllForCompliancePathTest(t, filepath.Dir(filePath))
	mkdirAllForCompliancePathTest(t, rulesRoot)

	got := mustFindRulesRootFromFile(t, filePath)
	if got, want := canonicalPathForCompliancePathTest(t, got), canonicalPathForCompliancePathTest(t, rulesRoot); got != want {
		t.Fatalf("repoRulesRootFromFile()=%q want %q", got, rulesRoot)
	}
}

func repoRulesRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return mustFindRulesRootFromFile(t, file)
}

func mustFindRulesRootFromFile(t *testing.T, file string) string {
	t.Helper()
	path, ok := testutil.FindSiblingRepoFromFile(file, "syl-listing-pro-rules", "rules")
	if !ok {
		t.Fatal("未找到 rules 仓目录")
	}
	return path
}

func runGitForCompliancePathTest(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v, output=%s", args, err, string(out))
	}
	return string(out)
}

func mkdirAllForCompliancePathTest(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func writeFileForCompliancePathTest(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func canonicalPathForCompliancePathTest(t *testing.T, path string) string {
	t.Helper()
	clean := filepath.Clean(path)
	resolved, err := filepath.EvalSymlinks(clean)
	if err == nil {
		return resolved
	}
	return clean
}

func writeListingMarkdownPair(t *testing.T, english string, chinese string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	enPath := filepath.Join(dir, "listing_demo_en.md")
	cnPath := filepath.Join(dir, "listing_demo_cn.md")
	if err := os.WriteFile(enPath, []byte(english), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cnPath, []byte(chinese), 0o644); err != nil {
		t.Fatal(err)
	}
	return enPath, cnPath
}

func validEnglishMarkdown() string {
	return validEnglishMarkdownWithValues(
		titleText(120),
		[]string{
			bulletText("First"),
			bulletText("Second"),
			bulletText("Third"),
			bulletText("Fourth"),
			bulletText("Fifth"),
		},
		strings.Join([]string{
			paragraphText("first", 390),
			paragraphText("second", 390),
		}, "\n\n"),
		searchTermsText(140),
	)
}

func validEnglishMarkdownWithTitle(title string) string {
	return validEnglishMarkdownWithValues(
		title,
		[]string{
			bulletText("First"),
			bulletText("Second"),
			bulletText("Third"),
			bulletText("Fourth"),
			bulletText("Fifth"),
		},
		strings.Join([]string{
			paragraphText("first", 390),
			paragraphText("second", 390),
		}, "\n\n"),
		searchTermsText(140),
	)
}

func validEnglishMarkdownWithBullets(lines []string) string {
	return validEnglishMarkdownWithValues(
		titleText(120),
		lines,
		strings.Join([]string{
			paragraphText("first", 390),
			paragraphText("second", 390),
		}, "\n\n"),
		searchTermsText(140),
	)
}

func validEnglishMarkdownWithDescription(description string) string {
	return validEnglishMarkdownWithValues(
		titleText(120),
		[]string{
			bulletText("First"),
			bulletText("Second"),
			bulletText("Third"),
			bulletText("Fourth"),
			bulletText("Fifth"),
		},
		description,
		searchTermsText(140),
	)
}

func validEnglishMarkdownWithSearchTerms(searchTerms string) string {
	return validEnglishMarkdownWithValues(
		titleText(120),
		[]string{
			bulletText("First"),
			bulletText("Second"),
			bulletText("Third"),
			bulletText("Fourth"),
			bulletText("Fifth"),
		},
		strings.Join([]string{
			paragraphText("first", 390),
			paragraphText("second", 390),
		}, "\n\n"),
		searchTerms,
	)
}

func validEnglishMarkdownWithValues(title string, bullets []string, description string, searchTerms string) string {
	return strings.Join([]string{
		"# Amazon Listing (EN)",
		"",
		"## 分类",
		"Home & Kitchen > Decor",
		"",
		"## 关键词",
		"paper lanterns",
		"",
		"## 标题",
		title,
		"",
		"## 五点描述",
		strings.Join(bullets, "\n"),
		"",
		"## 产品描述",
		description,
		"",
		"## 搜索词",
		searchTerms,
		"",
	}, "\n")
}

func validChineseMarkdown() string {
	return strings.Join([]string{
		"# 亚马逊 Listing (CN)",
		"",
		"## 分类",
		"家居用品 > 装饰",
		"",
		"## 关键词",
		"纸灯笼 教室装饰 派对用品",
		"",
		"## 标题",
		"这是一个满足结构要求的中文标题内容，长度足够并且不为空。",
		"",
		"## 五点描述",
		"第一点：中文卖点内容完整。",
		"第二点：中文卖点内容完整。",
		"第三点：中文卖点内容完整。",
		"第四点：中文卖点内容完整。",
		"第五点：中文卖点内容完整。",
		"",
		"## 产品描述",
		"第一段中文描述内容完整。",
		"",
		"第二段中文描述内容完整。",
		"",
		"## 搜索词",
		"纸灯笼 教室装饰 派对用品",
		"",
	}, "\n")
}

func titleText(length int) string {
	return repeatedWords("brand paper lanterns classroom decor", length)
}

func bulletText(prefix string) string {
	heading := strings.TrimSpace(prefix)
	if !strings.Contains(heading, " ") {
		heading += " Feature"
	}
	return heading + ": " + strings.TrimSpace(
		"**paper lanterns** keep classroom displays bright and easy to refresh for lessons, parties, and welcome days. "+
			"**hanging decor** adds cheerful color without tools, while **classroom decoration** supports bulletin boards, reading corners, ceiling displays, and reusable seasonal setups.",
	)
}

func paragraphText(prefix string, length int) string {
	return repeatedWords(prefix+" durable paper lanterns classroom decoration hanging decor wedding decorations", length)
}

func searchTermsText(length int) string {
	return repeatedWords("paper lanterns classroom decor party supplies hanging decorations", length)
}

func repeatedWords(seed string, minLength int) string {
	base := strings.TrimSpace(seed)
	if len(base) >= minLength {
		return base[:minLength]
	}
	var b strings.Builder
	for b.Len() < minLength {
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(base)
	}
	out := b.String()
	if len(out) > minLength {
		return strings.TrimSpace(out[:minLength])
	}
	return out
}

func assertHasViolation(t *testing.T, report listingComplianceReport, language string, section string) {
	t.Helper()
	for _, violation := range report.Violations {
		if strings.Contains(violation.Language, language) && strings.Contains(violation.Section, section) {
			return
		}
	}
	t.Fatalf("expected violation for %s %s, got %+v", language, section, report.Violations)
}
