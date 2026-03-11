package worker

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

func (s Service) httpClient() *http.Client {
	if s.HTTPClient != nil {
		return s.HTTPClient
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func (s Service) workerRepo() string {
	if strings.TrimSpace(s.WorkerRepo) != "" {
		return s.WorkerRepo
	}
	return "/Users/wxy/syl-listing-pro/worker"
}

func (s Service) remote() RemoteExecutor {
	if s.Remote != nil {
		return s.Remote
	}
	return shellRemoteExecutor{}
}

func (s Service) resolveServer(name string) (Server, error) {
	servers := s.Servers
	if len(servers) == 0 {
		servers = DefaultServers()
	}
	server, ok := servers[name]
	if !ok {
		return Server{}, fmt.Errorf("未知服务器: %s", name)
	}
	return server, nil
}

type shellRemoteExecutor struct{}

func (shellRemoteExecutor) Copy(ctx context.Context, server Server, src, dst string) error {
	target := fmt.Sprintf("%s@%s:%s", server.User, server.Host, dst)
	cmd := exec.CommandContext(ctx, "scp", "-P", fmt.Sprintf("%d", server.Port), src, target)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("scp 失败: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func (shellRemoteExecutor) Run(ctx context.Context, server Server, cmdText string) error {
	target := fmt.Sprintf("%s@%s", server.User, server.Host)
	cmd := exec.CommandContext(ctx, "ssh", "-p", fmt.Sprintf("%d", server.Port), target, cmdText)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ssh 失败: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func (shellRemoteExecutor) Stream(ctx context.Context, server Server, cmdText string) error {
	target := fmt.Sprintf("%s@%s", server.User, server.Host)
	cmd := exec.CommandContext(ctx, "ssh", "-p", fmt.Sprintf("%d", server.Port), target, cmdText)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ssh 失败: %w", err)
	}
	return nil
}
