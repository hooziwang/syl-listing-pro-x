package testutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func FindSiblingRepoFromFile(file string, names ...string) (string, bool) {
	if workspaceRoot, ok := repoWorkspaceRootFromFile(file); ok {
		for _, name := range names {
			candidate := filepath.Join(workspaceRoot, name)
			if dirExists(candidate) {
				return candidate, true
			}
		}
	}

	for _, name := range names {
		candidate := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", name))
		if dirExists(candidate) {
			return candidate, true
		}
	}
	return "", false
}

func repoWorkspaceRootFromFile(file string) (string, bool) {
	startDir := filepath.Dir(file)
	topLevel, ok := gitTrimmedOutputFromDir(startDir, "rev-parse", "--show-toplevel")
	if !ok {
		return "", false
	}
	commonDir, ok := gitTrimmedOutputFromDir(startDir, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if !ok {
		return "", false
	}
	if filepath.Base(commonDir) != ".git" {
		return "", false
	}
	repoRoot := filepath.Dir(commonDir)
	if filepath.Clean(topLevel) != filepath.Clean(repoRoot) {
		return filepath.Dir(repoRoot), true
	}
	return filepath.Dir(topLevel), true
}

func gitTrimmedOutputFromDir(dir string, args ...string) (string, bool) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
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
