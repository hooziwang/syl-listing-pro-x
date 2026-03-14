package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestDefaultPaths(t *testing.T) {
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	neutralWD := t.TempDir()
	if err := os.Chdir(neutralWD); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldWD)
	})
	cfg := DefaultPaths()
	if cfg.WorkspaceRoot != "/Users/wxy/syl-listing-pro" {
		t.Fatalf("WorkspaceRoot=%q", cfg.WorkspaceRoot)
	}
	if cfg.WorkerRepo != "/Users/wxy/syl-listing-pro/worker" {
		t.Fatalf("WorkerRepo=%q", cfg.WorkerRepo)
	}
	if cfg.RulesRepo != "/Users/wxy/syl-listing-pro/rules" {
		t.Fatalf("RulesRepo=%q", cfg.RulesRepo)
	}
	if cfg.WorkerURL != "https://worker.aelus.tech" {
		t.Fatalf("WorkerURL=%q", cfg.WorkerURL)
	}
}

func TestDefaultPathsUsesEnvOverride(t *testing.T) {
	t.Setenv("SYL_LISTING_PRO_WORKSPACE_ROOT", "/tmp/syl-listing-pro-test")
	cfg := DefaultPaths()
	if cfg.WorkspaceRoot != "/tmp/syl-listing-pro-test" {
		t.Fatalf("WorkspaceRoot=%q", cfg.WorkspaceRoot)
	}
	if cfg.WorkerRepo != "/tmp/syl-listing-pro-test/syl-listing-worker" {
		t.Fatalf("WorkerRepo=%q", cfg.WorkerRepo)
	}
	if cfg.RulesRepo != "/tmp/syl-listing-pro-test/syl-listing-pro-rules" {
		t.Fatalf("RulesRepo=%q", cfg.RulesRepo)
	}
}

func TestDefaultPathsIgnoresBlankEnvOverride(t *testing.T) {
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	neutralWD := t.TempDir()
	if err := os.Chdir(neutralWD); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldWD)
	})
	if err := os.Setenv("SYL_LISTING_PRO_WORKSPACE_ROOT", "   "); err != nil {
		t.Fatalf("Setenv error = %v", err)
	}
	t.Cleanup(func() {
		_ = os.Unsetenv("SYL_LISTING_PRO_WORKSPACE_ROOT")
	})
	cfg := DefaultPaths()
	if cfg.WorkspaceRoot != "/Users/wxy/syl-listing-pro" {
		t.Fatalf("WorkspaceRoot=%q", cfg.WorkspaceRoot)
	}
}

func TestDefaultPathsUsesPerRepoOverrides(t *testing.T) {
	t.Setenv("SYL_LISTING_PRO_WORKER_REPO", "/tmp/custom-worker")
	t.Setenv("SYL_LISTING_PRO_RULES_REPO", "/tmp/custom-rules")
	t.Setenv("SYL_LISTING_WORKER_URL", "https://worker.example.test/")
	cfg := DefaultPaths()
	if cfg.WorkerRepo != "/tmp/custom-worker" {
		t.Fatalf("WorkerRepo=%q", cfg.WorkerRepo)
	}
	if cfg.RulesRepo != "/tmp/custom-rules" {
		t.Fatalf("RulesRepo=%q", cfg.RulesRepo)
	}
	if cfg.WorkerURL != "https://worker.example.test" {
		t.Fatalf("WorkerURL=%q", cfg.WorkerURL)
	}
}

func TestDefaultPathsUsesSiblingWorktreesWhenRunningInsideWorktree(t *testing.T) {
	root := t.TempDir()
	worktreeName := "workflow-engine-refactor"
	cliWD := filepath.Join(root, "syl-listing-pro-x", ".worktrees", worktreeName)
	workerWD := filepath.Join(root, "worker", ".worktrees", worktreeName)
	rulesWD := filepath.Join(root, "rules", ".worktrees", worktreeName)
	for _, dir := range []string{cliWD, workerWD, rulesWD} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll(%q) error = %v", dir, err)
		}
	}

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(cliWD); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldWD)
	})
	t.Setenv("SYL_LISTING_PRO_WORKSPACE_ROOT", root)
	t.Setenv("SYL_LISTING_PRO_WORKER_REPO", "")
	t.Setenv("SYL_LISTING_PRO_RULES_REPO", "")

	cfg := DefaultPaths()
	if cfg.WorkerRepo != workerWD {
		t.Fatalf("WorkerRepo=%q want %q", cfg.WorkerRepo, workerWD)
	}
	if cfg.RulesRepo != rulesWD {
		t.Fatalf("RulesRepo=%q want %q", cfg.RulesRepo, rulesWD)
	}
}

func TestDefaultPathsUsesSiblingReposFromGitCommonDirWhenRunningInsideGlobalWorktree(t *testing.T) {
	root := t.TempDir()
	workspaceRoot := filepath.Join(root, "workspace")
	mainRepo := filepath.Join(workspaceRoot, "syl-listing-pro-x")
	workerRepo := filepath.Join(workspaceRoot, "syl-listing-worker")
	rulesRepo := filepath.Join(workspaceRoot, "syl-listing-pro-rules")
	for _, dir := range []string{mainRepo, workerRepo, rulesRepo} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll(%q) error = %v", dir, err)
		}
	}
	runGitForPathsTest(t, mainRepo, "init")
	runGitForPathsTest(t, mainRepo, "config", "user.email", "test@example.com")
	runGitForPathsTest(t, mainRepo, "config", "user.name", "tester")
	if err := os.WriteFile(filepath.Join(mainRepo, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitForPathsTest(t, mainRepo, "add", "README.md")
	runGitForPathsTest(t, mainRepo, "commit", "-m", "init")

	globalWorktree := filepath.Join(root, "global-worktrees", "syl-listing-pro-x", "fix-paths")
	runGitForPathsTest(t, mainRepo, "worktree", "add", globalWorktree, "-b", "fix-paths")

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(globalWorktree); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldWD)
	})
	t.Setenv("SYL_LISTING_PRO_WORKSPACE_ROOT", "")
	t.Setenv("SYL_LISTING_PRO_WORKER_REPO", "")
	t.Setenv("SYL_LISTING_PRO_RULES_REPO", "")

	cfg := DefaultPaths()
	if got, want := canonicalPathForPathsTest(t, cfg.WorkspaceRoot), canonicalPathForPathsTest(t, workspaceRoot); got != want {
		t.Fatalf("WorkspaceRoot=%q want %q", got, want)
	}
	if got, want := canonicalPathForPathsTest(t, cfg.WorkerRepo), canonicalPathForPathsTest(t, workerRepo); got != want {
		t.Fatalf("WorkerRepo=%q want %q", got, want)
	}
	if got, want := canonicalPathForPathsTest(t, cfg.RulesRepo), canonicalPathForPathsTest(t, rulesRepo); got != want {
		t.Fatalf("RulesRepo=%q want %q", got, want)
	}
}

func runGitForPathsTest(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v, output=%s", args, err, string(out))
	}
	return string(bytesTrimSpaceForPathsTest(out))
}

func bytesTrimSpaceForPathsTest(in []byte) []byte {
	for len(in) > 0 && (in[len(in)-1] == '\n' || in[len(in)-1] == '\r' || in[len(in)-1] == ' ' || in[len(in)-1] == '\t') {
		in = in[:len(in)-1]
	}
	for len(in) > 0 && (in[0] == '\n' || in[0] == '\r' || in[0] == ' ' || in[0] == '\t') {
		in = in[1:]
	}
	return in
}

func canonicalPathForPathsTest(t *testing.T, path string) string {
	t.Helper()
	clean := filepath.Clean(path)
	resolved, err := filepath.EvalSymlinks(clean)
	if err == nil {
		return resolved
	}
	return clean
}
