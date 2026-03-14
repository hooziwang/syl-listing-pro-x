package config

import (
	"os"
	"os/exec"
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
const defaultWorkspaceRoot = "/Users/wxy/syl-listing-pro"

func DefaultPaths() Paths {
	currentRepo, hasCurrentRepo := detectCurrentKnownRepo()

	root := defaultWorkspaceRoot
	if envRoot := strings.TrimSpace(os.Getenv("SYL_LISTING_PRO_WORKSPACE_ROOT")); envRoot != "" {
		root = envRoot
	} else if hasCurrentRepo {
		root = currentRepo.workspaceRoot
	}

	workerRepo := defaultRepoPath(root, "syl-listing-worker", "worker")
	if envWorker := strings.TrimSpace(os.Getenv("SYL_LISTING_PRO_WORKER_REPO")); envWorker != "" {
		workerRepo = envWorker
	}

	rulesRepo := defaultRepoPath(root, "syl-listing-pro-rules", "rules")
	if envRules := strings.TrimSpace(os.Getenv("SYL_LISTING_PRO_RULES_REPO")); envRules != "" {
		rulesRepo = envRules
	}

	if hasCurrentRepo {
		if currentRepo.isWorker && strings.TrimSpace(os.Getenv("SYL_LISTING_PRO_WORKER_REPO")) == "" {
			workerRepo = currentRepo.topLevel
		}
		if currentRepo.isRules && strings.TrimSpace(os.Getenv("SYL_LISTING_PRO_RULES_REPO")) == "" {
			rulesRepo = currentRepo.topLevel
		}
	}

	workerURL := strings.TrimRight(strings.TrimSpace(os.Getenv("SYL_LISTING_WORKER_URL")), "/")
	if workerURL == "" {
		workerURL = defaultWorkerURL
	}

	if worktreeName, ok := detectCurrentWorktreeName(); ok {
		if candidate := filepath.Join(defaultRepoPath(root, "syl-listing-worker", "worker"), ".worktrees", worktreeName); dirExists(candidate) && strings.TrimSpace(os.Getenv("SYL_LISTING_PRO_WORKER_REPO")) == "" {
			workerRepo = candidate
		}
		if candidate := filepath.Join(defaultRepoPath(root, "syl-listing-pro-rules", "rules"), ".worktrees", worktreeName); dirExists(candidate) && strings.TrimSpace(os.Getenv("SYL_LISTING_PRO_RULES_REPO")) == "" {
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

type currentRepoInfo struct {
	workspaceRoot string
	topLevel      string
	isWorker      bool
	isRules       bool
}

func detectCurrentKnownRepo() (currentRepoInfo, bool) {
	topLevel, ok := gitTrimmedOutput("rev-parse", "--show-toplevel")
	if !ok {
		return currentRepoInfo{}, false
	}
	topLevel = filepath.Clean(topLevel)

	commonDir, ok := gitTrimmedOutput("rev-parse", "--path-format=absolute", "--git-common-dir")
	if !ok {
		return currentRepoInfo{}, false
	}
	commonDir = filepath.Clean(commonDir)
	if filepath.Base(commonDir) != ".git" {
		return currentRepoInfo{}, false
	}

	repoRoot := filepath.Dir(commonDir)
	repoName := filepath.Base(repoRoot)
	info := currentRepoInfo{
		workspaceRoot: filepath.Dir(repoRoot),
		topLevel:      topLevel,
		isWorker:      repoName == "syl-listing-worker" || repoName == "worker",
		isRules:       repoName == "syl-listing-pro-rules" || repoName == "rules",
	}
	if !info.isWorker && !info.isRules && repoName != "syl-listing-pro-x" {
		return currentRepoInfo{}, false
	}
	return info, true
}

func defaultRepoPath(root, preferred, legacy string) string {
	preferredPath := filepath.Join(root, preferred)
	if dirExists(preferredPath) {
		return preferredPath
	}
	legacyPath := filepath.Join(root, legacy)
	if dirExists(legacyPath) {
		return legacyPath
	}
	if filepath.Base(root) == "syl-listing-pro" {
		return legacyPath
	}
	return preferredPath
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

	currentRepo, ok := detectCurrentKnownRepo()
	if !ok {
		return "", false
	}
	mainCLIRepo := filepath.Join(currentRepo.workspaceRoot, "syl-listing-pro-x")
	if filepath.Clean(currentRepo.topLevel) == filepath.Clean(mainCLIRepo) {
		return "", false
	}
	name := strings.TrimSpace(filepath.Base(currentRepo.topLevel))
	if name == "" {
		return "", false
	}
	return name, true
}

func gitTrimmedOutput(args ...string) (string, bool) {
	cmd := exec.Command("git", args...)
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	value := strings.TrimSpace(string(out))
	if value == "" {
		return "", false
	}
	return value, true
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
