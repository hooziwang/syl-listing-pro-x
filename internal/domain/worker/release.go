package worker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func (s Service) Release(ctx context.Context, in ReleaseInput) error {
	version := strings.TrimSpace(in.Version)
	if version == "" {
		return fmt.Errorf("缺少 version")
	}
	if err := s.ensureGitClean(ctx); err != nil {
		return err
	}
	if err := s.runWorkerTests(ctx); err != nil {
		return err
	}
	if err := s.ensureTagDoesNotExist(ctx, version); err != nil {
		return err
	}
	if err := s.createTag(ctx, version); err != nil {
		return err
	}
	if err := s.pushTag(ctx, version); err != nil {
		return err
	}

	worktreePath, cleanup, err := s.createTaggedWorktree(ctx, version)
	if err != nil {
		return err
	}
	defer cleanup()

	releaseSvc := Service{
		HTTPClient: s.HTTPClient,
		WorkerRepo: worktreePath,
		Remote:     s.Remote,
		Servers:    s.Servers,
	}
	if err := releaseSvc.Deploy(ctx, DeployInput{
		Server:             in.Server,
		SkipBuild:          in.SkipBuild,
		StopLegacy:         in.StopLegacy,
		InstallDocker:      in.InstallDocker,
		SkipWaitHTTPS:      in.SkipWaitHTTPS,
		HTTPSTimeout:       in.HTTPSTimeout,
		HTTPSCheckInterval: in.HTTPSCheckInterval,
		SkipDiagnose:       in.SkipDiagnose,
	}); err != nil {
		return err
	}

	baseURL, err := releaseSvc.releaseBaseURL(in)
	if err != nil {
		return err
	}
	if _, err := releaseSvc.CheckRemoteVersion(ctx, CheckRemoteVersionInput{
		BaseURL:    baseURL,
		AdminToken: in.AdminToken,
	}); err != nil {
		return err
	}
	return nil
}

func (s Service) releaseBaseURL(in ReleaseInput) (string, error) {
	if baseURL := strings.TrimRight(strings.TrimSpace(in.BaseURL), "/"); baseURL != "" {
		return baseURL, nil
	}
	server, err := s.resolveServer(in.Server)
	if err != nil {
		return "", err
	}
	cfg, err := s.resolveWorkerConfig(server)
	if err != nil {
		return "", err
	}
	domain := strings.TrimSpace(server.Domain)
	if domain == "" {
		domain = strings.TrimSpace(cfg.Server.Domain)
	}
	if domain == "" {
		return "", fmt.Errorf("无法推导远端 worker 地址：server.domain 与 worker.config.json 中的 server.domain 都为空")
	}
	return "https://" + domain, nil
}

func (s Service) ensureGitClean(ctx context.Context) error {
	out, err := s.runCommand(ctx, s.workerRepo(), "git", "status", "--short")
	if err != nil {
		return err
	}
	if strings.TrimSpace(out) != "" {
		return fmt.Errorf("worker 工作区不干净：请先提交或清理改动后再执行 release")
	}
	return nil
}

func (s Service) runWorkerTests(ctx context.Context) error {
	_, err := s.runCommand(ctx, s.workerRepo(), "npm", "test")
	if err != nil {
		return fmt.Errorf("worker npm test 失败: %w", err)
	}
	return nil
}

func (s Service) ensureTagDoesNotExist(ctx context.Context, version string) error {
	if _, err := s.runCommand(ctx, s.workerRepo(), "git", "rev-parse", "-q", "--verify", "refs/tags/"+version); err == nil {
		return fmt.Errorf("worker 版本 tag 已存在: %s", version)
	}
	if _, err := s.runCommand(ctx, s.workerRepo(), "git", "ls-remote", "--exit-code", "--tags", "origin", "refs/tags/"+version); err == nil {
		return fmt.Errorf("远端 worker 版本 tag 已存在: %s", version)
	}
	return nil
}

func (s Service) createTag(ctx context.Context, version string) error {
	if _, err := s.runCommand(ctx, s.workerRepo(), "git", "tag", "--no-sign", version); err != nil {
		return fmt.Errorf("创建 worker tag 失败: %w", err)
	}
	return nil
}

func (s Service) pushTag(ctx context.Context, version string) error {
	if _, err := s.runCommand(ctx, s.workerRepo(), "git", "push", "origin", version); err != nil {
		return fmt.Errorf("推送 worker tag 失败: %w", err)
	}
	return nil
}

func (s Service) createTaggedWorktree(ctx context.Context, version string) (string, func(), error) {
	tmpDir, err := os.MkdirTemp("", "syl-worker-release-*")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() {
		_, _ = s.runCommand(context.Background(), s.workerRepo(), "git", "worktree", "remove", "--force", tmpDir)
		_ = os.RemoveAll(tmpDir)
	}
	if _, err := s.runCommand(ctx, s.workerRepo(), "git", "worktree", "add", "--detach", tmpDir, version); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("创建 worker 发布 worktree 失败: %w", err)
	}
	return filepath.Clean(tmpDir), cleanup, nil
}

func (s Service) runCommand(ctx context.Context, dir, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	trimmed := strings.TrimSpace(string(out))
	if err != nil {
		if trimmed == "" {
			return "", err
		}
		return "", fmt.Errorf("%w: %s", err, trimmed)
	}
	return trimmed, nil
}
