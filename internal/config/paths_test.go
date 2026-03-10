package config

import "testing"

func TestDefaultPaths(t *testing.T) {
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
}
