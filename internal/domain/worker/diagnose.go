package worker

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	keySelection := "SYL_KEYS_RAW=$(grep -E '^SYL_LISTING_KEYS=' .env | tail -n 1 | cut -d'=' -f2-); " +
		"SYL_KEY=$(printf '%s\\n' \"$SYL_KEYS_RAW\" | tr ',' '\\n' | awk -F: '$1==\"syl\"{print substr($0, index($0,\":\")+1); found=1; exit} END{if(!found && NF){print substr($0, index($0,\":\")+1)}}')"
	return strings.Join([]string{
		"set -euo pipefail",
		fmt.Sprintf("cd %s", server.Dir),
		fmt.Sprintf("running=\"$(%s)\"", composeFallbackCommand("ps --services --status running")),
		"for svc in redis worker-api worker-runner nginx certbot; do echo \"$running\" | grep -qx \"$svc\" || { echo \"服务未运行: $svc\" >&2; exit 1; }; done",
		composeFallbackCommand(fmt.Sprintf("exec -T worker-api node -e %s", shellQuoteArg(healthCheckJS))),
		keySelection,
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
		"input_markdown": "===Listing Requirements===\n\n# 基础信息\n\n品牌名: E2EBrand\n数量/包装:12 pack\n核心材质:paper and metal frame\n颜色:Colorful plaid\n尺寸:10 x 10 in\n重量:330g\n\n# 功能卖点\n\n1. classroom hanging decoration\n2. diy friendly setup\n3. reusable foldable structure\n4. bright colorful plaid look\n5. wide party use\n\n# 产品细节信息\n\n包装内含:12 paper lanterns with metal frames\n适用场景:classroom ceiling hanging decoration for back to school displays\n设计特点:colorful plaid round lanterns with collapsible frame\n质量/安全认证:lightweight paper decor for indoor party display\n\n# 关键词库（按权重排序，共15-20个）\n\npaper lanterns\npaper lanterns decorative\ncolorful paper lanterns\nhanging paper lanterns\nhanging decor\npaper hanging decorations\nceiling hanging decor\nhanging ceiling decor\nclassroom decoration\nhanging classroom decoration\nceiling hanging classroom decor\nwedding decorations\nchinese lanterns\nball lanterns lamps\nsummer party decorations\nhanging decor from ceiling\nbaby shower decorations\npaper lamp\nwedding shower decorations\n\n# 分类\n\nTools & Home Improvement>Lighting & Ceiling Fans>Novelty Lighting>Paper Lanterns\n\n# 特殊关键要求\n\n适用场景最主要是教室悬挂装饰\n\n五点内容：\n\n套装包含（尺寸）\n材质（可重复使用）\n颜色\n组装（轻松安装）\n用途广泛\n",
	}
	var gen generateResponse
	if err := s.requestJSON(ctx, http.MethodPost, baseURL+"/v1/generate", auth.AccessToken, body, &gen); err != nil {
		return fmt.Errorf("generate 失败: %w", err)
	}
	if strings.TrimSpace(gen.JobID) == "" {
		return fmt.Errorf("generate 缺少 job_id")
	}

	streamCtx, cancel := context.WithTimeout(ctx, in.Timeout)
	defer cancel()

	status, err := s.waitForJobTerminalStatus(streamCtx, baseURL, auth.AccessToken, gen.JobID)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("生成流式等待超时")
		}
		return fmt.Errorf("订阅任务事件失败: %w", err)
	}
	switch strings.ToLower(strings.TrimSpace(status.Status)) {
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
		return fmt.Errorf("生成失败: %s", status.Error)
	case "cancelled":
		return fmt.Errorf("生成被取消")
	default:
		return fmt.Errorf("未知任务状态: %s", status.Status)
	}
}

func (s Service) waitForJobTerminalStatus(
	ctx context.Context,
	baseURL, bearer, jobID string,
) (jobEventStatusResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/v1/jobs/"+jobID+"/events", nil)
	if err != nil {
		return jobEventStatusResponse{}, err
	}
	req.Header.Set("Authorization", "Bearer "+bearer)
	req.Header.Set("Accept", "text/event-stream")

	baseClient := s.httpClient()
	streamClient := *baseClient
	streamClient.Timeout = 0
	resp, err := streamClient.Do(req)
	if err != nil {
		return jobEventStatusResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return jobEventStatusResponse{}, fmt.Errorf("%s", strings.TrimSpace(string(data)))
	}

	reader := bufio.NewReader(resp.Body)
	var eventName string
	var dataLines []string
	flush := func() (jobEventStatusResponse, bool, error) {
		if len(dataLines) == 0 {
			eventName = ""
			return jobEventStatusResponse{}, false, nil
		}
		defer func() {
			eventName = ""
			dataLines = dataLines[:0]
		}()
		if eventName != "status" {
			return jobEventStatusResponse{}, false, nil
		}
		var payload jobEventStatusResponse
		if err := json.Unmarshal([]byte(strings.Join(dataLines, "\n")), &payload); err != nil {
			return jobEventStatusResponse{}, false, err
		}
		switch strings.ToLower(strings.TrimSpace(payload.Status)) {
		case "succeeded", "failed", "cancelled":
			return payload, true, nil
		default:
			return jobEventStatusResponse{}, false, nil
		}
	}

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				if payload, ok, flushErr := flush(); flushErr != nil {
					return jobEventStatusResponse{}, flushErr
				} else if ok {
					return payload, nil
				}
				return jobEventStatusResponse{}, fmt.Errorf("事件流在终态前结束")
			}
			return jobEventStatusResponse{}, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			if payload, ok, flushErr := flush(); flushErr != nil {
				return jobEventStatusResponse{}, flushErr
			} else if ok {
				return payload, nil
			}
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		switch {
		case strings.HasPrefix(line, "event:"):
			eventName = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
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
