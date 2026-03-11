package config

import (
	"os"
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
	if cfg.WorkerRepo != "/tmp/syl-listing-pro-test/worker" {
		t.Fatalf("WorkerRepo=%q", cfg.WorkerRepo)
	}
	if cfg.RulesRepo != "/tmp/syl-listing-pro-test/rules" {
		t.Fatalf("RulesRepo=%q", cfg.RulesRepo)
	}
}

func TestDefaultPathsIgnoresBlankEnvOverride(t *testing.T) {
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
