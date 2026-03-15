# Syl Listing Pro X Version Metadata Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 为 `syl-listing-pro-x` 增加和 `cli` 对齐的版本信息注入与命令输出能力。

**Architecture:** 在 `cmd` 层新增共享版本变量与输出函数，根命令复用 `--version` 早返回逻辑，并增加 `version` 子命令。构建层通过 `Makefile` 的 `-ldflags` 注入 `VERSION`、`COMMIT`、`BUILD_TIME`，让 `make/build/install` 产物一致携带版本元数据。

**Tech Stack:** Go, Cobra, GNU Make

---

### Task 1: 版本输出测试

**Files:**
- Modify: `cmd/help_text_test.go`
- Create: `cmd/version_test.go`
- Create: `cmd/root_version_test.go`

1. 写失败测试，覆盖 `versionText()`、`printVersion()`、`version` 子命令和 `--version` 标志。
2. 运行 `go test ./cmd -run 'TestVersion|TestRoot'`，确认先失败。

### Task 2: 命令实现

**Files:**
- Modify: `cmd/root.go`
- Create: `cmd/version.go`
- Create: `cmd/version_cmd.go`

1. 增加 `Version/Commit/BuildTime` 默认值与版本输出函数。
2. 根命令加入 `--version/-v` 并在无其他参数时短路输出版本。
3. 注册 `version` 子命令。

### Task 3: 构建注入与文档

**Files:**
- Modify: `Makefile`
- Modify: `README.md`

1. 给 `Makefile` 增加 `VERSION/COMMIT/BUILD_TIME/LDFLAGS`。
2. 让 `build/install` 产物都携带版本注入。
3. README 补充版本构建行为说明。

### Task 4: 验证

**Files:**
- Verify only

1. 运行 `go test ./...`。
2. 运行 `make`。
3. 运行 `bin/syl-listing-pro-x --version`。
4. 运行 `/Users/wxy/go/bin/syl-listing-pro-x version`。
