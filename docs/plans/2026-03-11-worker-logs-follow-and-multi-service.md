# Worker Logs Follow And Multi Service Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 为 `syl-listing-pro-x worker logs` 增加 `--no-follow` 和多 `--service` 支持，并补充 README 的用法示例。

**Architecture:** 保持现有 `worker` 领域和 Cobra 命令分层不变，在 `LogsInput` 中加入 `Follow` 和 `Services`，统一由 `buildRemoteLogsCommand` 负责远端命令拼装。`README.md` 只补使用示例，不引入额外设计说明。

**Tech Stack:** Go, Cobra, go test, SSH + docker compose logs

---

### Task 1: 先写失败测试

**Files:**
- Modify: `internal/domain/worker/deploy_test.go`

**Step 1: Write the failing test**
- 新增测试，断言 `LogsInput{Follow:false}` 生成的命令不包含 `-f`。
- 新增测试，断言 `LogsInput{Services:["worker-api","nginx"]}` 会按顺序透传两个 service。

**Step 2: Run test to verify it fails**
Run: `go test ./internal/domain/worker -run 'TestLogsUsesComposeFallback|TestLogsSupportsFilters|TestLogsSupportsNoFollowAndMultipleServices' -v`
Expected: FAIL，因为当前只支持单 service，且固定 follow。

**Step 3: Write minimal implementation**
- 暂不改实现，保持失败。

**Step 4: Run test to verify it fails correctly**
Run: `go test ./internal/domain/worker -run 'TestLogsUsesComposeFallback|TestLogsSupportsFilters|TestLogsSupportsNoFollowAndMultipleServices' -v`
Expected: FAIL with 命令断言失败。

### Task 2: 实现领域层参数与命令拼装

**Files:**
- Modify: `internal/domain/worker/service.go`

**Step 1: Write minimal implementation**
- `LogsInput` 增加：`Services []string`、`Follow bool`
- `buildRemoteLogsCommand`：
  - `Follow=false` 时不加 `-f`
  - 多 service 逐个追加
  - 默认 follow 为 true

**Step 2: Run targeted tests**
Run: `go test ./internal/domain/worker -run 'TestLogsUsesComposeFallback|TestLogsSupportsFilters|TestLogsSupportsNoFollowAndMultipleServices' -v`
Expected: PASS

### Task 3: 接入 Cobra flags

**Files:**
- Modify: `cmd/worker_logs.go`

**Step 1: Write minimal implementation**
- `--service` 改为可重复 flag
- 增加 `--no-follow`
- 将参数映射到 `LogsInput`

**Step 2: Verify help output**
Run: `go run . worker logs --help`
Expected: 输出包含 `--no-follow` 和新的 `--service` 描述。

### Task 4: 更新 README 示例

**Files:**
- Modify: `README.md`

**Step 1: Add examples**
- 默认查看全部服务
- 单服务
- 多服务
- `--since`
- `--no-follow`

**Step 2: Manual readback**
Run: `sed -n '1,220p' README.md`
Expected: `worker logs` 示例清晰且不冗余。

### Task 5: 全量验证与真实远端验证

**Files:**
- No code changes expected

**Step 1: Run tests**
Run: `go test ./internal/domain/worker -v && go test ./...`
Expected: PASS

**Step 2: Build binary**
Run: `make`
Expected: 生成 `bin/syl-listing-pro-x`

**Step 3: Real verify**
Run: `go run . worker logs --server syl-server --service worker-api --service nginx --tail 20 --since 10m --no-follow`
Expected: 只输出 `worker-api` 与 `nginx` 的最近日志，并执行一次后退出。
