# Syl Listing Pro X Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 构建 `syl-listing-pro-x` 独立工程仓库，提供 `rules`、`worker`、`e2e` 三个工程侧子命令，逐步替代现有 Python/bash 工具链。

**Architecture:** 使用单 Go 二进制 + Cobra 子命令。`cmd/` 只负责命令装配，`internal/domain` 负责 `rules/worker/e2e` 业务编排，`internal/platform` 负责 SSH/HTTP/进程/归档等基础能力。首期先完成可运行的命令骨架和 `rules validate/package/publish`、`worker diagnose/diagnose-external`、`e2e list/run` 最小闭环。

**Tech Stack:** Go, Cobra, net/http, os/exec, archive/tar, crypto/sha256, OpenSSL CLI, SSH/SCP

---

### Task 1: 初始化仓库与根命令

**Files:**
- Create: `go.mod`
- Create: `main.go`
- Create: `cmd/root.go`
- Create: `cmd/rules.go`
- Create: `cmd/worker.go`
- Create: `cmd/e2e.go`
- Create: `internal/config/paths.go`
- Create: `internal/config/paths_test.go`

**Step 1: Write the failing test**

验证默认路径解析与仓库根目录固定值。

**Step 2: Run test to verify it fails**

Run: `go test ./...`
Expected: FAIL，因为文件和包还不存在。

**Step 3: Write minimal implementation**

初始化 Go 模块、根命令、三个一级子命令和默认路径配置。

**Step 4: Run test to verify it passes**

Run: `go test ./...`
Expected: PASS

### Task 2: 实现 rules validate/package/publish

**Files:**
- Create: `cmd/rules_validate.go`
- Create: `cmd/rules_package.go`
- Create: `cmd/rules_publish.go`
- Create: `internal/domain/rules/service.go`
- Create: `internal/domain/rules/service_test.go`
- Create: `internal/platform/archive/tarball.go`
- Create: `internal/platform/httpx/client.go`

**Step 1: Write the failing test**

用 fixture 模拟 `tenants/<tenant>/rules`，验证：
- `validate` 能识别缺字段
- `package` 产出 `dist/<tenant>/<version>/rules.tar.gz` 与 `manifest.json`
- `publish` 能向 worker 管理接口发出正确 payload

**Step 2: Run test to verify it fails**

Run: `go test ./internal/domain/rules -v`
Expected: FAIL

**Step 3: Write minimal implementation**

迁移 Python 工具链逻辑到 Go，暂时继续使用 `openssl` 命令做签名与公钥导出。

**Step 4: Run test to verify it passes**

Run: `go test ./internal/domain/rules -v`
Expected: PASS

### Task 3: 实现 worker diagnose/diagnose-external

**Files:**
- Create: `cmd/worker_diagnose.go`
- Create: `cmd/worker_diagnose_external.go`
- Create: `internal/domain/worker/diagnose.go`
- Create: `internal/domain/worker/diagnose_test.go`

**Step 1: Write the failing test**

模拟外部接口，验证健康检查、鉴权交换、规则接口检查、可选生成链路检查。

**Step 2: Run test to verify it fails**

Run: `go test ./internal/domain/worker -v`
Expected: FAIL

**Step 3: Write minimal implementation**

用 Go HTTP 客户端替代现有 `diagnose_external.sh` 主要逻辑。

**Step 4: Run test to verify it passes**

Run: `go test ./internal/domain/worker -v`
Expected: PASS

### Task 4: 实现 worker deploy/push-env 基础能力

**Files:**
- Create: `cmd/worker_deploy.go`
- Create: `cmd/worker_push_env.go`
- Create: `internal/domain/worker/deploy.go`
- Create: `internal/platform/ssh/ssh.go`
- Create: `internal/platform/execx/exec.go`
- Create: `internal/domain/worker/deploy_test.go`

**Step 1: Write the failing test**

验证：
- 服务器别名能解析成固定远端
- `.env` 推送命令与远端重启命令构造正确
- 远程部署命令参数拼装正确

**Step 2: Run test to verify it fails**

Run: `go test ./internal/domain/worker -run 'TestDeploy|TestPushEnv' -v`
Expected: FAIL

**Step 3: Write minimal implementation**

首期先实现 Go 命令编排与 SSH/SCP 调用，不内嵌完整 docker/deploy 所有细节。

**Step 4: Run test to verify it passes**

Run: `go test ./internal/domain/worker -run 'TestDeploy|TestPushEnv' -v`
Expected: PASS

### Task 5: 实现 e2e list/run 骨架

**Files:**
- Create: `cmd/e2e_list.go`
- Create: `cmd/e2e_run.go`
- Create: `internal/domain/e2e/service.go`
- Create: `internal/domain/e2e/service_test.go`
- Create: `internal/domain/e2e/scenarios.go`

**Step 1: Write the failing test**

验证：
- 能列出固定 case
- `run` 能调用已安装 `syl-listing-pro`
- 能收集 stdout/stderr、退出码和 artifacts 目录

**Step 2: Run test to verify it fails**

Run: `go test ./internal/domain/e2e -v`
Expected: FAIL

**Step 3: Write minimal implementation**

先做黑盒命令编排与基础 artifact 收集，不立即实现全部发版用例。

**Step 4: Run test to verify it passes**

Run: `go test ./internal/domain/e2e -v`
Expected: PASS

### Task 6: 全量验证

**Files:**
- Modify: `README.md`
- Create: `.gitignore`
- Create: `Makefile`

**Step 1: Run full verification**

Run: `go test ./...`
Expected: PASS

**Step 2: Build binary**

Run: `go build ./...`
Expected: PASS

**Step 3: Smoke test help output**

Run: `go run . --help`
Expected: 输出根命令及 `rules/worker/e2e`
