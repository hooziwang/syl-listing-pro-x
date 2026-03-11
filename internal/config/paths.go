package config

import (
	"os"
	"path/filepath"
	"strings"
)

type Paths struct {
	WorkspaceRoot string
	WorkerRepo    string
	RulesRepo     string
	WorkerURL     string
}

const defaultWorkerURL = "https://worker.aelus.tech"

func DefaultPaths() Paths {
	root := "/Users/wxy/syl-listing-pro"
	if envRoot := strings.TrimSpace(os.Getenv("SYL_LISTING_PRO_WORKSPACE_ROOT")); envRoot != "" {
		root = envRoot
	}
	workerRepo := filepath.Join(root, "worker")
	if envWorker := strings.TrimSpace(os.Getenv("SYL_LISTING_PRO_WORKER_REPO")); envWorker != "" {
		workerRepo = envWorker
	}
	rulesRepo := filepath.Join(root, "rules")
	if envRules := strings.TrimSpace(os.Getenv("SYL_LISTING_PRO_RULES_REPO")); envRules != "" {
		rulesRepo = envRules
	}
	workerURL := strings.TrimRight(strings.TrimSpace(os.Getenv("SYL_LISTING_WORKER_URL")), "/")
	if workerURL == "" {
		workerURL = defaultWorkerURL
	}
	if worktreeName, ok := detectCurrentWorktreeName(); ok {
		if candidate := filepath.Join(root, "worker", ".worktrees", worktreeName); dirExists(candidate) && strings.TrimSpace(os.Getenv("SYL_LISTING_PRO_WORKER_REPO")) == "" {
			workerRepo = candidate
		}
		if candidate := filepath.Join(root, "rules", ".worktrees", worktreeName); dirExists(candidate) && strings.TrimSpace(os.Getenv("SYL_LISTING_PRO_RULES_REPO")) == "" {
			rulesRepo = candidate
		}
	}
	return Paths{
		WorkspaceRoot: root,
		WorkerRepo:    workerRepo,
		RulesRepo:     rulesRepo,
		WorkerURL:     workerURL,
	}
}

func detectCurrentWorktreeName() (string, bool) {
	wd, err := os.Getwd()
	if err != nil {
		return "", false
	}
	clean := filepath.Clean(wd)
	parts := strings.Split(clean, string(filepath.Separator))
	for i := 0; i < len(parts)-1; i++ {
		if parts[i] != ".worktrees" {
			continue
		}
		name := strings.TrimSpace(parts[i+1])
		if name == "" {
			return "", false
		}
		return name, true
	}
	return "", false
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
