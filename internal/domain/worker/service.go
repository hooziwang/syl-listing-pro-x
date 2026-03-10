package worker

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Server struct {
	Name string
	Host string
	User string
	Port int
	Dir  string
}

type Service struct {
	HTTPClient *http.Client
	WorkerRepo string
	Remote     RemoteExecutor
	Servers    map[string]Server
}

type DiagnoseExternalInput struct {
	BaseURL      string
	SYLKey       string
	WithGenerate bool
	Timeout      time.Duration
	Interval     time.Duration
}

type PushEnvInput struct {
	Server string
}

type LogsInput struct {
	Server   string
	Services []string
	Tail     int
	Since    string
	NoFollow bool
}

type DeployInput struct {
	Server        string
	SkipBuild     bool
	StopLegacy    bool
	InstallDocker bool
	SkipWaitHTTPS bool
	HTTPSTimeout  int
	HTTPSInterval int
	SkipDiagnose  bool
}

type RemoteExecutor interface {
	Copy(ctx context.Context, server Server, src, dst string) error
	Run(ctx context.Context, server Server, cmd string) error
	Stream(ctx context.Context, server Server, cmd string) error
}

type healthResponse struct {
	OK  bool `json:"ok"`
	LLM struct {
		Fluxcode providerHealth `json:"fluxcode"`
		Deepseek providerHealth `json:"deepseek"`
	} `json:"llm"`
}

type providerHealth struct {
	OK       bool  `json:"ok"`
	Required *bool `json:"required"`
}

type authExchangeResponse struct {
	TenantID    string `json:"tenant_id"`
	AccessToken string `json:"access_token"`
}

type resolveRulesResponse struct {
	RulesVersion string `json:"rules_version"`
	DownloadURL  string `json:"download_url"`
}

type generateResponse struct {
	JobID string `json:"job_id"`
}

type jobStatusResponse struct {
	Status string `json:"status"`
	Error  string `json:"error"`
}

type jobResultResponse struct {
	ENMarkdown string `json:"en_markdown"`
	CNMarkdown string `json:"cn_markdown"`
}

type workerConfig struct {
	Server struct {
		Domain           string `json:"domain"`
		LetsencryptEmail string `json:"letsencrypt_email"`
	} `json:"server"`
}

func DefaultServers() map[string]Server {
	return map[string]Server{
		"syl-server": {
			Name: "syl-server",
			Host: "43.135.112.167",
			User: "ubuntu",
			Port: 22,
			Dir:  "/opt/syl-listing-worker",
		},
	}
}

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
	tmpArchive, err := s.createWorkerArchive()
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
	cmd := s.buildRemoteDeployCommand(server, remoteTmp, in)
	if err := remote.Run(ctx, server, cmd); err != nil {
		return err
	}
	if in.SkipDiagnose {
		return nil
	}
	return s.Diagnose(ctx, in.Server)
}

func (s Service) Diagnose(ctx context.Context, serverName string) error {
	server, err := s.resolveServer(serverName)
	if err != nil {
		return err
	}
	cmd := s.buildRemoteDiagnoseCommand(server)
	return s.remote().Run(ctx, server, cmd)
}

func (s Service) Logs(ctx context.Context, in LogsInput) error {
	server, err := s.resolveServer(in.Server)
	if err != nil {
		return err
	}
	cmd := s.buildRemoteLogsCommand(server, in)
	return s.remote().Stream(ctx, server, cmd)
}

func (s Service) buildRemoteDeployCommand(server Server, remoteTmp string, in DeployInput) string {
	timeout := in.HTTPSTimeout
	if timeout <= 0 {
		timeout = 240
	}
	interval := in.HTTPSInterval
	if interval <= 0 {
		interval = 2
	}
	cfg, err := s.loadWorkerConfig()
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

func (s Service) buildRemoteDiagnoseCommand(server Server) string {
	cfg, err := s.loadWorkerConfig()
	if err != nil {
		return fmt.Sprintf("echo %q >&2; exit 1", err.Error())
	}
	healthCheckJS := `const ok=(p)=>p?.required===false||p?.ok===true;const run=async()=>{const res=await fetch("http://127.0.0.1:8080/healthz");const raw=await res.text();const data=JSON.parse(raw);if(!res.ok||!ok(data.llm?.fluxcode)||!ok(data.llm?.deepseek)){throw new Error(raw)}};run().catch(e=>{console.error(e.message);process.exit(1);});`
	return strings.Join([]string{
		"set -euo pipefail",
		fmt.Sprintf("cd %s", server.Dir),
		fmt.Sprintf("running=\"$(%s)\"", composeFallbackCommand("ps --services --status running")),
		"for svc in redis worker-api worker-runner nginx certbot; do echo \"$running\" | grep -qx \"$svc\" || { echo \"服务未运行: $svc\" >&2; exit 1; }; done",
		composeFallbackCommand(fmt.Sprintf("exec -T worker-api node -e %s", shellQuoteArg(healthCheckJS))),
		"SYL_KEY=$(grep -E '^SYL_LISTING_KEYS=' .env | tail -n 1 | cut -d'=' -f2- | cut -d',' -f1 | cut -d':' -f2-)",
		"[ -n \"$SYL_KEY\" ]",
		composeFallbackCommand("exec -T -e SYL_KEY=\"$SYL_KEY\" worker-api node -e \"const run=async()=>{const exchange=await fetch('http://127.0.0.1:8080/v1/auth/exchange',{method:'POST',headers:{Authorization:'Bearer ' + process.env.SYL_KEY}});if(!exchange.ok) throw new Error('auth exchange failed');const j=await exchange.json();const rules=await fetch('http://127.0.0.1:8080/v1/rules/resolve?current=',{headers:{Authorization:'Bearer ' + j.access_token}});if(!rules.ok) throw new Error('rules resolve failed');const r=await rules.json();if(!r.rules_version) throw new Error('rules_version missing')};run().catch(e=>{console.error(e.message);process.exit(1);});\""),
		fmt.Sprintf("%s | tr -d '\\r' | grep -qx PONG", composeFallbackCommand("exec -T redis redis-cli ping")),
		fmt.Sprintf("%s >/dev/null", composeFallbackCommand("exec -T nginx nginx -t")),
		composeFallbackCommand(fmt.Sprintf("exec -T nginx sh -lc 'test -s /etc/letsencrypt/live/%s/fullchain.pem && test -s /etc/letsencrypt/live/%s/privkey.pem || true'", shellSingleQuote(cfg.Server.Domain), shellSingleQuote(cfg.Server.Domain))),
	}, "; ")
}

func (s Service) buildRemoteLogsCommand(server Server, in LogsInput) string {
	tail := in.Tail
	if tail <= 0 {
		tail = 200
	}
	args := []string{
		"logs",
		fmt.Sprintf("--tail=%d", tail),
	}
	if !in.NoFollow {
		args = append(args[:1], append([]string{"-f"}, args[1:]...)...)
	}
	if strings.TrimSpace(in.Since) != "" {
		args = append(args, "--since", shellQuoteArg(in.Since))
	}
	for _, service := range in.Services {
		if strings.TrimSpace(service) == "" {
			continue
		}
		args = append(args, shellQuoteArg(service))
	}
	return fmt.Sprintf("set -euo pipefail; cd %s; %s", server.Dir, composeFallbackCommand(strings.Join(args, " ")))
}

func composeFallbackCommand(args string) string {
	return fmt.Sprintf(
		"if docker compose --env-file .compose.env %s 2>/dev/null; then :; elif sudo -n docker compose --env-file .compose.env %s; then :; else exit 1; fi",
		args,
		args,
	)
}

func (s Service) DiagnoseExternal(ctx context.Context, in DiagnoseExternalInput) error {
	baseURL := strings.TrimRight(strings.TrimSpace(in.BaseURL), "/")
	if baseURL == "" {
		return fmt.Errorf("缺少 base-url")
	}
	if strings.TrimSpace(in.SYLKey) == "" {
		return fmt.Errorf("缺少 SYL_LISTING_KEY")
	}
	if in.Timeout <= 0 {
		in.Timeout = 5 * time.Minute
	}
	if in.Interval <= 0 {
		in.Interval = 2 * time.Second
	}

	var health healthResponse
	if err := s.requestJSON(ctx, http.MethodGet, baseURL+"/healthz", "", nil, &health); err != nil {
		return fmt.Errorf("healthz 检查失败: %w", err)
	}
	if !healthAcceptable(health) {
		return fmt.Errorf("healthz 返回异常")
	}

	var auth authExchangeResponse
	if err := s.requestJSON(ctx, http.MethodPost, baseURL+"/v1/auth/exchange", in.SYLKey, nil, &auth); err != nil {
		return fmt.Errorf("auth exchange 失败: %w", err)
	}
	if strings.TrimSpace(auth.AccessToken) == "" {
		return fmt.Errorf("auth exchange 缺少 access_token")
	}

	var resolve resolveRulesResponse
	if err := s.requestJSON(ctx, http.MethodGet, baseURL+"/v1/rules/resolve?current=", auth.AccessToken, nil, &resolve); err != nil {
		return fmt.Errorf("rules resolve 失败: %w", err)
	}
	if strings.TrimSpace(resolve.RulesVersion) == "" || strings.TrimSpace(resolve.DownloadURL) == "" {
		return fmt.Errorf("rules resolve 缺少关键字段")
	}
	if err := s.requestJSON(ctx, http.MethodPost, baseURL+"/v1/rules/refresh", auth.AccessToken, nil, &map[string]any{}); err != nil {
		return fmt.Errorf("rules refresh 失败: %w", err)
	}
	if err := s.requestDownload(ctx, resolve.DownloadURL, auth.AccessToken); err != nil {
		return fmt.Errorf("rules download 失败: %w", err)
	}

	if !in.WithGenerate {
		return nil
	}

	body := map[string]any{
		"input_markdown": "===Listing Requirements===\n\n# 基础信息\n品牌名: E2EBrand\n\n# 关键词库\nkeyword one\nkeyword two\nkeyword three\nkeyword four\nkeyword five\nkeyword six\nkeyword seven\nkeyword eight\nkeyword nine\nkeyword ten\nkeyword eleven\nkeyword twelve\nkeyword thirteen\nkeyword fourteen\nkeyword fifteen\n\n# 分类\nHome & Kitchen > Decor\n",
	}
	var gen generateResponse
	if err := s.requestJSON(ctx, http.MethodPost, baseURL+"/v1/generate", auth.AccessToken, body, &gen); err != nil {
		return fmt.Errorf("generate 失败: %w", err)
	}
	if strings.TrimSpace(gen.JobID) == "" {
		return fmt.Errorf("generate 缺少 job_id")
	}

	deadline := time.Now().Add(in.Timeout)
	for time.Now().Before(deadline) {
		var st jobStatusResponse
		if err := s.requestJSON(ctx, http.MethodGet, baseURL+"/v1/jobs/"+gen.JobID, auth.AccessToken, nil, &st); err != nil {
			return fmt.Errorf("查询任务失败: %w", err)
		}
		switch strings.ToLower(strings.TrimSpace(st.Status)) {
		case "queued", "running", "":
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(in.Interval):
			}
		case "succeeded":
			var result jobResultResponse
			if err := s.requestJSON(ctx, http.MethodGet, baseURL+"/v1/jobs/"+gen.JobID+"/result", auth.AccessToken, nil, &result); err != nil {
				return fmt.Errorf("读取结果失败: %w", err)
			}
			if strings.TrimSpace(result.ENMarkdown) == "" || strings.TrimSpace(result.CNMarkdown) == "" {
				return fmt.Errorf("结果为空")
			}
			return nil
		case "failed":
			return fmt.Errorf("生成失败: %s", st.Error)
		case "cancelled":
			return fmt.Errorf("生成被取消")
		default:
			return fmt.Errorf("未知任务状态: %s", st.Status)
		}
	}
	return fmt.Errorf("生成轮询超时")
}

func providerHealthy(p providerHealth) bool {
	if p.Required != nil && !*p.Required {
		return true
	}
	return p.OK
}

func healthAcceptable(h healthResponse) bool {
	return providerHealthy(h.LLM.Fluxcode) && providerHealthy(h.LLM.Deepseek)
}

func (s Service) requestDownload(ctx context.Context, url, bearer string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+bearer)
	resp, err := s.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s", strings.TrimSpace(string(data)))
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return fmt.Errorf("下载为空")
	}
	return nil
}

func (s Service) requestJSON(ctx context.Context, method, url, bearer string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = strings.NewReader(string(data))
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return err
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := s.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s", strings.TrimSpace(string(data)))
	}
	if out == nil || len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, out)
}

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

func (s Service) createWorkerArchive() (string, error) {
	repo := s.workerRepo()
	cfg, err := s.loadWorkerConfig()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(cfg.Server.Domain) == "" {
		return "", fmt.Errorf("config.server.domain 不能为空")
	}

	tmpFile, err := os.CreateTemp("", "syl-listing-worker-*.tar.gz")
	if err != nil {
		return "", err
	}
	tmpPath := tmpFile.Name()
	defer tmpFile.Close()

	gz := gzip.NewWriter(tmpFile)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	composeEnv := fmt.Sprintf("DOMAIN=%s\nLETSENCRYPT_EMAIL=%s\n", cfg.Server.Domain, cfg.Server.LetsencryptEmail)
	if err := addBytesToTar(tw, ".compose.env", []byte(composeEnv), 0o644); err != nil {
		return "", err
	}

	err = filepath.Walk(repo, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(repo, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		name := filepath.ToSlash(rel)
		if shouldSkipWorkerPath(name, info.IsDir()) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = name
		if info.IsDir() {
			header.Name += "/"
		}
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(tw, f)
		return err
	})
	if err != nil {
		return "", err
	}
	return tmpPath, nil
}

func (s Service) loadWorkerConfig() (workerConfig, error) {
	var cfg workerConfig
	cfgData, err := os.ReadFile(filepath.Join(s.workerRepo(), "worker.config.json"))
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(cfgData, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func shellSingleQuote(input string) string {
	return strings.ReplaceAll(input, "'", `'\''`)
}

func shellQuoteArg(input string) string {
	return "'" + shellSingleQuote(strings.TrimSpace(input)) + "'"
}

func shouldSkipWorkerPath(name string, isDir bool) bool {
	parts := strings.Split(name, "/")
	if len(parts) == 0 {
		return false
	}
	switch parts[0] {
	case ".git", "node_modules", "dist":
		return true
	}
	if strings.HasPrefix(name, "data/") && isDir {
		return true
	}
	return false
}

func addBytesToTar(tw *tar.Writer, name string, data []byte, mode int64) error {
	header := &tar.Header{
		Name:    name,
		Mode:    mode,
		Size:    int64(len(data)),
		ModTime: time.Now(),
	}
	if err := tw.WriteHeader(header); err != nil {
		return err
	}
	_, err := tw.Write(data)
	return err
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
