package worker

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (s Service) PushEnv(ctx context.Context, in PushEnvInput) error {
	server, err := s.resolveServer(in.Server)
	if err != nil {
		return err
	}
	envPath := filepath.Join(s.workerRepo(), ".env")
	if _, err := os.Stat(envPath); err != nil {
		return fmt.Errorf("未找到 %s", envPath)
	}
	remote := s.remote()
	if err := remote.Copy(ctx, server, envPath, "/tmp/syl-listing-worker.env.tmp"); err != nil {
		return err
	}
	cmd := fmt.Sprintf(
		"set -euo pipefail; cp /tmp/syl-listing-worker.env.tmp %[1]s/.env; rm -f /tmp/syl-listing-worker.env.tmp; cd %[1]s; %[2]s",
		server.Dir,
		composeFallbackCommand("up -d --no-deps --force-recreate worker-api worker-runner"),
	)
	return remote.Run(ctx, server, cmd)
}

func (s Service) Deploy(ctx context.Context, in DeployInput) error {
	server, err := s.resolveServer(in.Server)
	if err != nil {
		return err
	}
	repo := s.workerRepo()
	if _, err := os.Stat(filepath.Join(repo, "worker.config.json")); err != nil {
		return fmt.Errorf("缺少 worker.config.json")
	}
	tmpArchive, err := s.createWorkerArchiveForServer(server)
	if err != nil {
		return err
	}
	defer os.Remove(tmpArchive)
	remoteTmp := fmt.Sprintf("/tmp/syl-listing-worker-%d.tar.gz", time.Now().UnixNano())
	remote := s.remote()
	if err := remote.Run(ctx, server, fmt.Sprintf("set -euo pipefail; mkdir -p %s; find %s -mindepth 1 -maxdepth 1 ! -name data ! -name .env -exec rm -rf {} +", server.Dir, server.Dir)); err != nil {
		return err
	}
	if err := remote.Copy(ctx, server, tmpArchive, remoteTmp); err != nil {
		return err
	}
	envPath := filepath.Join(s.workerRepo(), ".env")
	if _, err := os.Stat(envPath); err == nil {
		if err := remote.Copy(ctx, server, envPath, "/tmp/syl-listing-worker.env.tmp"); err != nil {
			return err
		}
		if err := remote.Run(ctx, server, fmt.Sprintf("set -euo pipefail; cp /tmp/syl-listing-worker.env.tmp %s/.env; rm -f /tmp/syl-listing-worker.env.tmp", server.Dir)); err != nil {
			return err
		}
	}
	versionMeta, err := s.buildRuntimeVersionMetadata()
	if err != nil {
		return err
	}
	cmd := s.buildRemoteDeployCommand(server, remoteTmp, in, versionMeta)
	if err := remote.Run(ctx, server, cmd); err != nil {
		return err
	}
	if in.SkipDiagnose {
		return nil
	}
	return s.Diagnose(ctx, in.Server)
}

func (s Service) buildRemoteDeployCommand(server Server, remoteTmp string, in DeployInput, versionMeta string) string {
	timeout := in.HTTPSTimeout
	if timeout <= 0 {
		timeout = 240
	}
	interval := in.HTTPSCheckInterval
	if interval <= 0 {
		interval = 2
	}
	cfg, err := s.resolveWorkerConfig(server)
	if err != nil {
		return fmt.Sprintf("echo %q >&2; exit 1", err.Error())
	}
	composeUp := composeFallbackCommand("up -d --build")
	if in.SkipBuild {
		composeUp = composeFallbackCommand("up -d")
	}
	parts := []string{
		"set -euo pipefail",
		fmt.Sprintf("cd %s", server.Dir),
		fmt.Sprintf("tar -xzf %s -C %s", remoteTmp, server.Dir),
		fmt.Sprintf("rm -f %s", remoteTmp),
		"mkdir -p data/runtime",
		fmt.Sprintf("printf %%s %s > data/runtime/version.json", shellQuoteArg(versionMeta)),
	}
	if in.InstallDocker {
		parts = append(parts, "if ! command -v docker >/dev/null 2>&1; then sudo apt-get update -y && sudo apt-get install -y docker.io docker-compose-v2 python3 && sudo systemctl enable --now docker; fi")
	}
	if in.StopLegacy {
		parts = append(parts, "if command -v systemctl >/dev/null 2>&1; then sudo systemctl stop syl-listing-worker-api.service syl-listing-worker-runner.service nginx || true; sudo systemctl disable syl-listing-worker-api.service syl-listing-worker-runner.service nginx || true; fi")
	}
	parts = append(parts, composeUp)
	if !in.SkipWaitHTTPS {
		parts = append(parts, fmt.Sprintf("deadline=$(( $(date +%%s) + %d ))", timeout))
		parts = append(parts, fmt.Sprintf(
			"while true; do if %s; then break; fi; if [ $(date +%%s) -ge $deadline ]; then echo '等待 HTTPS 就绪超时' >&2; exit 1; fi; sleep %d; done",
			composeFallbackCommand(fmt.Sprintf("exec -T nginx sh -lc 'test -s /etc/letsencrypt/live/%s/fullchain.pem && test -s /etc/letsencrypt/live/%s/privkey.pem'", shellSingleQuote(cfg.Server.Domain), shellSingleQuote(cfg.Server.Domain))),
			interval,
		))
	}
	parts = append(parts, composeFallbackCommand("ps"))
	return strings.Join(parts, "; ")
}
