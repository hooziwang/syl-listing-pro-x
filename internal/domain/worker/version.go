package worker

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func (s Service) CheckRemoteVersion(ctx context.Context, in CheckRemoteVersionInput) (CheckRemoteVersionResult, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(in.BaseURL), "/")
	if baseURL == "" {
		return CheckRemoteVersionResult{}, fmt.Errorf("缺少 base-url")
	}
	adminToken := strings.TrimSpace(in.AdminToken)
	if adminToken == "" {
		var err error
		adminToken, err = loadAdminTokenFromUserEnv()
		if err != nil {
			return CheckRemoteVersionResult{}, err
		}
	}
	if adminToken == "" {
		return CheckRemoteVersionResult{}, fmt.Errorf("缺少 ADMIN_TOKEN")
	}
	localCommit, err := s.localGitCommitShort()
	if err != nil {
		return CheckRemoteVersionResult{}, err
	}
	var remote RemoteVersionInfo
	if err := s.requestJSON(ctx, http.MethodGet, baseURL+"/v1/admin/version", adminToken, nil, &remote); err != nil {
		return CheckRemoteVersionResult{}, fmt.Errorf("读取远端版本失败: %w", err)
	}
	result := CheckRemoteVersionResult{
		LocalGitCommit: localCommit,
		Remote:         remote,
		UpToDate:       strings.TrimSpace(remote.GitCommit) == localCommit,
	}
	if !result.UpToDate {
		return result, fmt.Errorf("远端 worker 不是最新版本：本地 %s，远端 %s", localCommit, strings.TrimSpace(remote.GitCommit))
	}
	return result, nil
}

func loadAdminTokenFromUserEnv() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("读取用户目录失败: %w", err)
	}
	envPath := filepath.Join(home, ".syl-listing-pro-x", ".env")
	f, err := os.Open(envPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("缺少 ADMIN_TOKEN，且未找到 %s", envPath)
		}
		return "", fmt.Errorf("读取 %s 失败: %w", envPath, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.HasPrefix(line, "ADMIN_TOKEN=") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(line, "ADMIN_TOKEN="))
		value = strings.Trim(value, `"'`)
		if value != "" {
			return value, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("读取 %s 失败: %w", envPath, err)
	}
	return "", fmt.Errorf("缺少 ADMIN_TOKEN，且 %s 未定义 ADMIN_TOKEN", envPath)
}

func (s Service) localGitCommitShort() (string, error) {
	cmd := exec.Command("git", "-C", s.workerRepo(), "rev-parse", "--short", "HEAD")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("读取本地 worker git commit 失败: %s", strings.TrimSpace(string(out)))
	}
	commit := strings.TrimSpace(string(out))
	if commit == "" {
		return "", fmt.Errorf("本地 worker git commit 为空")
	}
	return commit, nil
}

func (s Service) buildRuntimeVersionMetadata() (string, error) {
	commit, err := s.localGitCommitShort()
	if err != nil {
		return "", err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	payload := map[string]string{
		"service":     "syl-listing-worker",
		"git_commit":  commit,
		"build_time":  now,
		"deployed_at": now,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
