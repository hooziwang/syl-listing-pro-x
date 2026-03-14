package testutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestFindSiblingRepoFromFileUsesGitWorkspaceInsteadOfCurrentCWD(t *testing.T) {
	root := t.TempDir()
	workspaceRoot := filepath.Join(root, "workspace")
	repoRoot := filepath.Join(workspaceRoot, "syl-listing-pro-x")
	rulesRoot := filepath.Join(workspaceRoot, "syl-listing-pro-rules")
	mkdirAllForRepoSiblingTest(t, filepath.Join(repoRoot, "internal", "domain", "e2e"))
	mkdirAllForRepoSiblingTest(t, rulesRoot)

	runGitForRepoSiblingTest(t, repoRoot, "init")
	runGitForRepoSiblingTest(t, repoRoot, "config", "user.email", "test@example.com")
	runGitForRepoSiblingTest(t, repoRoot, "config", "user.name", "tester")
	writeFileForRepoSiblingTest(t, filepath.Join(repoRoot, "README.md"), "repo\n")
	runGitForRepoSiblingTest(t, repoRoot, "add", "README.md")
	runGitForRepoSiblingTest(t, repoRoot, "commit", "-m", "init")

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
	got, ok := FindSiblingRepoFromFile(filePath, "syl-listing-pro-rules", "rules")
	if !ok {
		t.Fatal("FindSiblingRepoFromFile() returned ok=false")
	}
	if got, want := canonicalPathForRepoSiblingTest(t, got), canonicalPathForRepoSiblingTest(t, rulesRoot); got != want {
		t.Fatalf("FindSiblingRepoFromFile()=%q want %q", got, want)
	}
}

func TestFindSiblingRepoFromFileSupportsLinkedWorktree(t *testing.T) {
	root := t.TempDir()
	workspaceRoot := filepath.Join(root, "workspace")
	repoRoot := filepath.Join(workspaceRoot, "syl-listing-pro-x")
	rulesRoot := filepath.Join(workspaceRoot, "syl-listing-pro-rules")
	mkdirAllForRepoSiblingTest(t, filepath.Join(repoRoot, "internal", "domain", "e2e"))
	mkdirAllForRepoSiblingTest(t, rulesRoot)

	runGitForRepoSiblingTest(t, repoRoot, "init")
	runGitForRepoSiblingTest(t, repoRoot, "config", "user.email", "test@example.com")
	runGitForRepoSiblingTest(t, repoRoot, "config", "user.name", "tester")
	writeFileForRepoSiblingTest(t, filepath.Join(repoRoot, "README.md"), "repo\n")
	runGitForRepoSiblingTest(t, repoRoot, "add", "README.md")
	runGitForRepoSiblingTest(t, repoRoot, "commit", "-m", "init")

	worktreePath := filepath.Join(root, "global-worktrees", "syl-listing-pro-x", "fix-rules-root")
	runGitForRepoSiblingTest(t, repoRoot, "worktree", "add", worktreePath, "-b", "fix-rules-root")
	mkdirAllForRepoSiblingTest(t, filepath.Join(worktreePath, "internal", "domain", "e2e"))

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
	got, ok := FindSiblingRepoFromFile(filePath, "syl-listing-pro-rules", "rules")
	if !ok {
		t.Fatal("FindSiblingRepoFromFile() returned ok=false")
	}
	if got, want := canonicalPathForRepoSiblingTest(t, got), canonicalPathForRepoSiblingTest(t, rulesRoot); got != want {
		t.Fatalf("FindSiblingRepoFromFile()=%q want %q", got, want)
	}
}

func TestFindSiblingRepoFromFileFallsBackToSiblingRepoNamesWithoutGit(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "parent")
	filePath := filepath.Join(parent, "syl-listing-pro-x", "internal", "domain", "e2e", "compliance_test.go")
	rulesRoot := filepath.Join(parent, "syl-listing-pro-rules")
	mkdirAllForRepoSiblingTest(t, filepath.Dir(filePath))
	mkdirAllForRepoSiblingTest(t, rulesRoot)

	got, ok := FindSiblingRepoFromFile(filePath, "syl-listing-pro-rules", "rules")
	if !ok {
		t.Fatal("FindSiblingRepoFromFile() returned ok=false")
	}
	if got, want := canonicalPathForRepoSiblingTest(t, got), canonicalPathForRepoSiblingTest(t, rulesRoot); got != want {
		t.Fatalf("FindSiblingRepoFromFile()=%q want %q", got, want)
	}
}

func TestFindSiblingRepoFromFileFallsBackToLegacyRulesDirWithoutGit(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "parent")
	filePath := filepath.Join(parent, "syl-listing-pro-x", "internal", "domain", "e2e", "compliance_test.go")
	rulesRoot := filepath.Join(parent, "rules")
	mkdirAllForRepoSiblingTest(t, filepath.Dir(filePath))
	mkdirAllForRepoSiblingTest(t, rulesRoot)

	got, ok := FindSiblingRepoFromFile(filePath, "syl-listing-pro-rules", "rules")
	if !ok {
		t.Fatal("FindSiblingRepoFromFile() returned ok=false")
	}
	if got, want := canonicalPathForRepoSiblingTest(t, got), canonicalPathForRepoSiblingTest(t, rulesRoot); got != want {
		t.Fatalf("FindSiblingRepoFromFile()=%q want %q", got, want)
	}
}

func runGitForRepoSiblingTest(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v, output=%s", args, err, string(out))
	}
	return string(out)
}

func mkdirAllForRepoSiblingTest(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func writeFileForRepoSiblingTest(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func canonicalPathForRepoSiblingTest(t *testing.T, path string) string {
	t.Helper()
	clean := filepath.Clean(path)
	resolved, err := filepath.EvalSymlinks(clean)
	if err == nil {
		return resolved
	}
	return clean
}
