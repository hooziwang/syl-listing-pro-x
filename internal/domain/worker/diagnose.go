package worker

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

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

func (s Service) buildRemoteDiagnoseCommand(server Server) string {
	cfg, err := s.loadWorkerConfig()
	if err != nil {
		return fmt.Sprintf("echo %q >&2; exit 1", err.Error())
	}
	healthCheckJS := `const run=async()=>{const res=await fetch("http://127.0.0.1:8080/healthz");const raw=await res.text();const data=JSON.parse(raw);if(![200,503].includes(res.status)||data.llm?.deepseek?.ok!==true){throw new Error(raw)}};run().catch(e=>{console.error(e.message);process.exit(1);});`
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
	if err := s.requestJSONWithStatus(ctx, http.MethodGet, baseURL+"/healthz", "", nil, &health, map[int]struct{}{
		http.StatusServiceUnavailable: {},
	}); err != nil {
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
	return providerHealthy(h.LLM.Deepseek)
}
