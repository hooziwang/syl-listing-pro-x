package cmd

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

func loadUserEnvValue(appDir string, key string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	envPath := filepath.Join(home, appDir, ".env")
	f, err := os.Open(envPath)
	if err != nil {
		return ""
	}
	defer f.Close()

	prefix := key + "="
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(line, prefix))
		value = strings.Trim(value, `"'`)
		if value != "" {
			return value
		}
	}
	return ""
}
