# Worker Logs Filters Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 为 `syl-listing-pro-x worker logs` 增加 `--service`、`--tail`、`--since`，支持按容器和时间范围筛选远端日志。

**Architecture:** 保持现有 `worker` 领域分层不变，在命令层解析新参数，在 `internal/domain/worker` 中新增结构化 `LogsInput` 统一拼装远端 `docker compose logs -f` 命令。继续沿用 `RemoteExecutor.Stream(...)` 进行流式输出，避免回退到缓冲式执行。

**Tech Stack:** Go, Cobra, Docker Compose over SSH, go test

---

### Task 1: 为 logs 增加失败测试

**Files:**
- Modify: `internal/domain/worker/deploy_test.go`

**Step 1: Write the failing test**
- 新增测试，断言 `Logs()` 在默认参数下使用 `logs -f --tail=200`。
- 新增测试，断言传入 `service/tail/since` 后，拼装出的命令包含 `--tail=<n>`、`--since <value>` 和服务名。

**Step 2: Run test to verify it fails**
Run: `go test ./internal/domain/worker -run 'TestLogsUsesComposeFallback|TestLogsSupportsFilters' -v`
Expected: FAIL，因为当前 `Logs()` 还不支持结构化参数。

**Step 3: Write minimal implementation**
- 暂不改实现，保持失败。

**Step 4: Run test to verify it fails correctly**
Run: `go test ./internal/domain/worker -run 'TestLogsUsesComposeFallback|TestLogsSupportsFilters' -v`
Expected: FAIL with 命令断言不满足。

### Task 2: 实现 LogsInput 与命令拼装

**Files:**
- Modify: `internal/domain/worker/service.go`

**Step 1: Write the failing test**
- 复用 Task 1 的失败测试。

**Step 2: Run test to verify it fails**
Run: `go test ./internal/domain/worker -run 'TestLogsUsesComposeFallback|TestLogsSupportsFilters' -v`
Expected: FAIL

**Step 3: Write minimal implementation**
- 新增 `LogsInput` 结构：`Server`、`Service`、`Tail`、`Since`。
- 将 `Logs(ctx, serverName string)` 改为 `Logs(ctx, in LogsInput)`。
- 提取 `buildRemoteLogsCommand(server, in)`，统一生成：
  - 默认 `--tail=200`
  - `since` 非空时追加 `--since <value>`
  - `service` 非空时在末尾追加服务名
- 继续使用 `composeFallbackCommand("logs -f ...")`。

**Step 4: Run test to verify it passes**
Run: `go test ./internal/domain/worker -run 'TestLogsUsesComposeFallback|TestLogsSupportsFilters' -v`
Expected: PASS

### Task 3: 接入 Cobra 参数

**Files:**
- Modify: `cmd/worker_logs.go`

**Step 1: Write the failing test**
- 该层当前无命令测试，本次不补 Cobra 单测，保持领域层断言为主。

**Step 2: Write minimal implementation**
- 增加 flags：`--service`、`--tail`、`--since`
- 将参数组装为 `worker.LogsInput`

**Step 3: Run command help check**
Run: `go run . worker logs --help`
Expected: 输出包含新 flags。

### Task 4: 全量验证与真实远端验证

**Files:**
- No code changes expected

**Step 1: Run targeted tests**
Run: `go test ./internal/domain/worker -v`
Expected: PASS

**Step 2: Run full test suite**
Run: `go test ./...`
Expected: PASS

**Step 3: Build binary**
Run: `make`
Expected: 生成 `bin/syl-listing-pro-x`

**Step 4: Real remote verification**
Run: `go run . worker logs --server syl-server --service worker-api --tail 20 --since 10m`
Expected: 仅输出 `worker-api` 最近 10 分钟内的日志，并保持跟随。
