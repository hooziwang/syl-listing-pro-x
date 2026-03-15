# syl-listing-pro-x

`syl-listing-pro-x` 是 `syl-listing-pro` 的工程侧控制台。

它把 3 类原本分散在不同仓库、不同阶段的动作收拢到一个 Go CLI 里：

- `rules`：校验、签名、打包、发布租户规则
- `worker`：唯一正式发布入口
- `e2e`：发版前真实调用规则发布、worker 接口和终端 CLI 的端到端验收

这不是用户侧生成 CLI。  
用户侧生成入口是 `cli/README.md` 里的 `syl-listing-pro`；  
`syl-listing-pro-x` 面向工程维护、发布、排障和验收。

跨仓内容分工：

- 规则结构与规则仓约束见 `rules/README.md`
- worker 运行模型与 API 见 `worker/README.md`
- 最终给运营/业务使用的生成 CLI 见 `cli/README.md`

## 核心定位

`syl-listing-pro-x` 负责把下面几件事串成稳定工作流：

1. 从 `rules` 仓读取租户规则源码并做结构校验。
2. 生成带签名的规则包与 `manifest.json`。
3. 调用 worker 管理接口发布规则。
4. 按版本 tag 发布 `worker` 到远端机器。
5. 通过 e2e 验证真实 worker 与真实 CLI 的完整链路。
6. 触发真实 `syl-listing-pro` CLI 生成，验证完整发布链路。

从代码结构看，它是一个典型的三层组织：

- `cmd/`：Cobra 命令层，负责参数收集和输出
- `internal/domain/rules|worker|e2e`：领域服务层，负责真实业务流程
- `internal/config`：跨仓路径、默认地址和 worktree 解析

## 目录概览

```text
syl-listing-pro-x/
├── cmd/                    # CLI 命令入口
├── internal/config/        # 工作区路径与默认地址解析
├── internal/domain/
│   ├── e2e/                # release-gate / architecture-gate
│   ├── rules/              # 规则校验、打包、签名、发布
│   └── worker/             # release 发布流与远端版本核对
├── artifacts/              # e2e 验收产物目录
├── bin/                    # 编译产物
└── main.go
```

## 路径与环境变量

程序默认直接操作当前工作区下的兄弟仓：

- `workspace root`：`/Users/wxy/syl-listing-pro`
- `rules repo`：`/Users/wxy/syl-listing-pro/rules`
- `worker repo`：`/Users/wxy/syl-listing-pro/worker`
- `worker url`：`https://worker.aelus.tech`

支持以下环境变量覆盖：

- `SYL_LISTING_PRO_WORKSPACE_ROOT`
- `SYL_LISTING_PRO_RULES_REPO`
- `SYL_LISTING_PRO_WORKER_REPO`
- `SYL_LISTING_WORKER_URL`

`internal/config/paths.go` 还有一个很重要的特性：  
如果当前 `syl-listing-pro-x` 运行在 `.worktrees/<name>` 里，并且 `worker/.worktrees/<name>`、`rules/.worktrees/<name>` 存在，那么它会自动切到同名 worktree，而不用手工传 repo 路径。这保证跨仓联调时默认能对齐同一条开发分支。

## 构建

最常用的构建方式：

```bash
cd /Users/wxy/syl-listing-pro/syl-listing-pro-x
make
```

`Makefile` 行为：

- `make test`：执行 `go test ./...`
- `make build`：生成带版本信息的 `bin/syl-listing-pro-x`
- `make install`：安装到 `GOBIN` 或 `GOPATH/bin`
- `make`：先测试、再构建、最后安装

产物路径：

```bash
bin/syl-listing-pro-x
```

查看版本：

```bash
syl-listing-pro-x --version
syl-listing-pro-x version
```

## 命令总览

```bash
syl-listing-pro-x rules ...
syl-listing-pro-x worker ...
syl-listing-pro-x e2e ...
```

### rules 子命令

```bash
syl-listing-pro-x rules validate --tenant syl
syl-listing-pro-x rules package --tenant syl
syl-listing-pro-x rules publish --tenant syl --admin-token <ADMIN_TOKEN>
```

### worker 子命令

```bash
syl-listing-pro-x worker release --server syl-server --version v0.1.3
```

### e2e 子命令

```bash
syl-listing-pro-x e2e list
syl-listing-pro-x e2e single --tenant syl --input /abs/demo.md
syl-listing-pro-x e2e run --case release-gate --tenant syl --key <SYL_LISTING_KEY> --admin-token <ADMIN_TOKEN> --input /abs/demo.md --out /abs/out
syl-listing-pro-x e2e run --case architecture-gate --tenant syl --key <SYL_LISTING_KEY> --admin-token <ADMIN_TOKEN> --private-key /abs/rules.pem --input /abs/demo.md --out /abs/out
syl-listing-pro-x e2e run --case listing-compliance-gate --tenant syl --key <SYL_LISTING_KEY> --admin-token <ADMIN_TOKEN> --out /abs/out
```

## rules：规则工具链

`rules` 领域代码位于 `internal/domain/rules`，核心职责是把租户规则从源码变成可发布的签名包。

### `rules validate`

```bash
syl-listing-pro-x rules validate --tenant syl
```

校验入口会检查：

- `tenants/<tenant>/rules` 目录存在
- `package.yaml` 具备 `required_sections`、`templates`
- `input.yaml` 具备 `file_discovery.marker`、`fields`
- `generation-config.yaml` 具备 `planning`、`judge`、`translation`、`render`、`display_labels`
- `sections/*.yaml` 覆盖 `required_sections` 中声明的 section
- 模板文件存在

除了字段存在性，它还会做结构约束校验：

- `input.fields` 不能为空
- 必须至少定义 `brand`、`keywords`、`category`
- `field.type` 只能是 `scalar` 或 `list`
- `field.capture` 只能是 `inline_label` 或 `heading_section`
- `workflow` 不再承载 DAG/节点编排；section 并发与 handoff 由 worker 的 runtime policy 决定

也就是说，这个校验不是“YAML 能读出来”就算过，而是在工程侧强制规则包满足当前 runtime-native worker 的规则契约。

### `rules package`

```bash
syl-listing-pro-x rules package --tenant syl
```

如果不传 `--version`，会自动生成版本号，格式类似：

```text
rules-syl-20260311-150405-ab12cd
```

打包流程：

1. 先执行完整规则校验。
2. 解析私钥来源。
3. 用 `openssl pkey -pubout` 导出公钥。
4. 生成 `rules.tar.gz`。
5. 计算 archive 的 SHA256。
6. 用私钥分别对 archive 和公钥文件签名。
7. 写出 `manifest.json`。

输出目录：

```text
rules/dist/<tenant>/<version>/
├── manifest.json
├── rules.tar.gz
└── rules_signing_public.pem
```

归档内容不是只打 `rules/` 子目录。  
代码实际会把整个 `tenants/<tenant>` 目录打入压缩包，并把公钥额外挂到 `tenant/rules_signing_public.pem`。

### 私钥解析优先级

`rules package` 和 `rules publish` 共用同一套私钥解析逻辑：

1. `--private-key`
2. `SYL_LISTING_RULES_PRIVATE_KEY`
3. `SIGNING_PRIVATE_KEY_PEM`
4. `SIGNING_PRIVATE_KEY_BASE64`
5. 显式开启本地开发模式后，才允许使用仓库内 `rules/keys/rules_private.pem`

开发模式开关：

```bash
SYL_LISTING_ALLOW_DEV_PRIVATE_KEY=1
```

限制：

- CI 或 GitHub Actions 环境下，开发私钥模式会被强制禁用
- 即使显式传了仓库内默认私钥路径，只要没开启开发模式，也会被拒绝
- `SIGNING_PRIVATE_KEY_PEM` / `SIGNING_PRIVATE_KEY_BASE64` 会被落成临时 PEM 文件，命令结束后自动清理

### `rules publish`

```bash
syl-listing-pro-x rules publish --tenant syl --admin-token <ADMIN_TOKEN>
```

它不是直接上传规则源码，而是：

1. 先跑一遍 `Package`
2. 读取 `dist/<tenant>/<version>/manifest.json`
3. 校验 manifest 里的 `tenant_id`、`rules_version` 与当前参数一致
4. 读取 `rules.tar.gz`
5. base64 编码 archive
6. `POST` 到 `POST /v1/admin/tenant-rules/publish`

默认超时是 2 分钟。  
返回成功后，CLI 会输出最终发布的 `rules_version`。

## worker：正式发布工具

`worker` 领域代码位于 `internal/domain/worker`，现在只保留一个正式入口：`worker release`。

默认服务器定义：

```text
name: syl-server
host: 43.135.112.167
user: ubuntu
port: 22
dir: /opt/syl-listing-worker
```

远端执行仍依赖本机已有：

- `ssh`
- `scp`
- 访问目标机器的权限

### `worker release`

```bash
syl-listing-pro-x worker release --server syl-server --version v0.1.3
```

这是唯一允许的正式发布路径，必须显式传入 `--server` 和 `--version`。

固定流程：

1. 校验本地 `worker` 仓工作区干净。
2. 执行 `npm test`。
3. 校验本地与远端都不存在同名版本 tag。
4. 在当前 `HEAD` 创建并推送 tag。
5. 从该 tag 创建临时干净 worktree。
6. 用这个 tag 对应代码打包、上传、部署远端 worker。
7. 写入远端 `data/runtime/version.json`，其中 `worker_version` 等于该 tag。
8. 调用远端 `/v1/admin/version`，确认远端 `git_commit` 与本地 tag 对应提交一致。

这个约束非常重要：  
发布包与线上 `worker_version` 必须来自同一个 tag，禁止再从当前工作区直接部署。

归档内容和远端部署细节仍由 `internal/domain/worker/deploy.go` 负责：

- 会打入绝大多数 `worker` 源码和部署文件
- 不会打入 `.git`、`node_modules`、`dist`、`.env`、`data`
- 会自动生成 `.compose.env`
- 如果本地 `worker/.env` 存在，会同步到远端
- 默认部署完成后会继续执行内部诊断

常用参数：

- `--skip-build`：远端只 `up -d`，不重建镜像
- `--stop-legacy`：停止旧的 systemd 服务
- `--install-docker`：远端缺 Docker 时自动安装
- `--skip-wait-https`：不等 HTTPS 证书 ready
- `--https-timeout`：等待超时秒数，默认 `240`
- `--https-interval`：HTTPS 就绪检查间隔秒数，默认 `2`
- `--skip-diagnose`：部署后不跑内部诊断

## e2e：真实端到端验收

`e2e` 领域代码位于 `internal/domain/e2e`，它不是 mock 级别测试，而是把规则发布、worker 对外诊断和 `syl-listing-pro` CLI 串起来做验收。

当前只暴露 2 个用例：

- `release-gate`
- `architecture-gate`
- `listing-compliance-gate`
- `single-listing-compliance-gate`

### `e2e list`

```bash
syl-listing-pro-x e2e list
```

直接输出当前支持的 case 名称。

### `e2e run`

```bash
syl-listing-pro-x e2e run \
  --case release-gate \
  --tenant syl \
  --key <SYL_LISTING_KEY> \
  --admin-token <ADMIN_TOKEN> \
  --input /abs/demo.md \
  --out /abs/out
```

固定流程：

1. 创建 artifact 目录
2. 调用 `rules publish`
3. 执行内置的 worker 外部黑盒诊断
4. 查找系统中的 `syl-listing-pro`
5. 执行真实 CLI（始终带 `--verbose`）：
   `syl-listing-pro <input> -o <out> --verbose --log-file <artifact>/cli.verbose.ndjson`
6. 收集输出目录里的 `.md` 和 `.docx`
7. 校验产物数量至少为 4
8. 额外校验英文 markdown 的 `## 搜索词` 第一行必须全小写

命令成功时，标准输出只打印 artifact 目录路径。

### `e2e single`

```bash
syl-listing-pro-x e2e single \
  --tenant syl \
  --input /abs/demo.md
```

这是单文件真实回归的正式入口，内部固定执行 `single-listing-compliance-gate`。
如果没有显式传 `--private-key`，并且当前机器也没有配置签名私钥环境变量，它会自动回退到仓库内开发私钥，同时临时开启本地开发模式。

固定流程：

1. 创建 artifact 目录
2. 如果未传 `--out`，自动推导到 `syl-listing-pro-x/out/<artifacts-id>`
3. 调用 `rules publish`
4. 执行内置的 worker 外部黑盒诊断
5. 查找系统中的 `syl-listing-pro`
6. 执行真实 CLI（始终带 `--verbose`）
7. 检查前台 verbose 日志是否出现非预期错误信号
8. 对单个输入产物执行定向 listing 合规校验

当前这个定向校验重点覆盖：

- 中英文 markdown 和 docx 产物齐全
- 模板要求的中英文 section 都存在
- 英文五点描述每条字符数满足当前硬规则
- 英文五点描述小标题词数满足当前硬规则
- 英文五点描述里的加粗关键词保持小写

### `release-gate`

用途：

- 验证真实规则发布链路
- 验证 worker 对外服务可用
- 验证 `syl-listing-pro` 真实生成能产出完整双语 Markdown + Word
- 验证英文 `search_terms` 保持全小写

### `architecture-gate`

它在 `release-gate` 基础上再验证工程治理内容，尤其适合改这些东西后回归：

- 私钥来源解析
- `--worker` 地址透传
- artifact 完整性
- 验收摘要输出

相对于 `release-gate`，它会额外要求 artifact 目录必须包含：

- `cli.verbose.ndjson`
- `cli.stdout.log`
- `cli.stderr.log`

同时写出：

```text
architecture-summary.json
```

里面会记录：

- `case_name`
- `tenant`
- `worker_url`
- `private_key_path`
- `output_files`

### `listing-compliance-gate`

它把 e2e 的重点从“链路能不能跑通”前移到“真实生成结果是否满足规则约束”。

固定输入来源：

- `syl-listing-pro-x/testdata/e2e/*.md`

它会对每个样例依次执行：

1. 发布当前租户规则
2. 运行内置的 worker 外部黑盒诊断
3. 真实执行 `syl-listing-pro --verbose`
4. 检查 `cli.stderr.log` 是否为空
5. 逐行解析 `cli.verbose.ndjson`
6. 只要发现预期之外错误信号就失败，例如：
   - `level=error`
   - `event` 包含 `failed` / `error`
   - `status=failed`
7. 对英文和中文 markdown 执行规则约束校验

当前首批合规断言包括：

- 中英文 markdown 和 docx 产物齐全
- 模板要求的 section 都存在
- 标题、五点、描述、搜索词满足当前 `rules/tenants/<tenant>/rules/sections/*.yaml` 中的长度/结构约束
- 英文搜索词满足全小写约束

每个输入样例都会在 artifact 目录下生成：

- `cli.verbose.ndjson`
- `cli.stdout.log`
- `cli.stderr.log`
- `compliance-summary.json`

如果不传 `--artifacts-id`，artifact 目录名默认使用 UTC 时间戳：

```text
YYYYMMDD-HHMMSS
```

默认根目录：

```text
syl-listing-pro-x/artifacts/
```

## 推荐工作流

### 规则改动后

```bash
cd /Users/wxy/syl-listing-pro/syl-listing-pro-x
make
bin/syl-listing-pro-x rules validate --tenant syl
bin/syl-listing-pro-x rules package --tenant syl
bin/syl-listing-pro-x rules publish --tenant syl --admin-token <ADMIN_TOKEN>
```

### worker 改动后

```bash
cd /Users/wxy/syl-listing-pro/syl-listing-pro-x
make
bin/syl-listing-pro-x worker release --server syl-server --version v0.1.3
```

### 发版前完整验收

```bash
cd /Users/wxy/syl-listing-pro/syl-listing-pro-x
make

bin/syl-listing-pro-x e2e single \
  --tenant syl \
  --input /Users/wxy/Downloads/test/12个装10寸开学季灯笼.md

bin/syl-listing-pro-x e2e run \
  --case release-gate \
  --tenant syl \
  --key <SYL_LISTING_KEY> \
  --admin-token <ADMIN_TOKEN> \
  --input /abs/demo.md \
  --out /abs/out

bin/syl-listing-pro-x e2e run \
  --case architecture-gate \
  --tenant syl \
  --key <SYL_LISTING_KEY> \
  --admin-token <ADMIN_TOKEN> \
  --private-key /abs/rules.pem \
  --input /abs/demo.md \
  --out /abs/out

bin/syl-listing-pro-x e2e run \
  --case listing-compliance-gate \
  --tenant syl \
  --key <SYL_LISTING_KEY> \
  --admin-token <ADMIN_TOKEN> \
  --out /abs/out
```

## 常见注意点

- `rules publish` 成功的前提是对应版本的 `manifest.json` 和 `rules.tar.gz` 已经存在且内容一致。
- `rules package` 依赖本机有可用的 `openssl`。
- `worker release` 依赖本机有 `ssh`、`scp`、`git`、`npm`。
- `worker release` 必须显式传入 `--server` 和 `--version`。
- `worker release` 会先检查本地 `worker` 工作区是否干净；有未提交改动时会直接失败。
- `worker release` 会从 tag 对应代码部署，不会再直接发布当前工作区。
- `e2e run` / `e2e single` 都依赖系统 PATH 中存在可执行的 `syl-listing-pro`，否则会在 `exec.LookPath("syl-listing-pro")` 时报错。
- `release-gate` / `architecture-gate` 只会收集输出目录根层级的 `.md` / `.docx` 文件，不会递归子目录。
- `listing-compliance-gate` 会为 `testdata/e2e` 下的每个输入样例创建独立输出子目录。
- `e2e single` 如果不传 `--out`，会自动落到 `syl-listing-pro-x/out/<artifacts-id>`。
- `diagnose-external` 对 provider 健康度采用 required-aware 策略：`required=false` 的 provider 即使 `ok=false` 也不会直接判死。

## 源码入口

如果要继续深入看实现，最值得先读这些文件：

- `main.go`
- `cmd/root.go`
- `internal/config/paths.go`
- `internal/domain/rules/validate.go`
- `internal/domain/rules/package.go`
- `internal/domain/rules/publish.go`
- `cmd/worker_release.go`
- `internal/domain/worker/release.go`
- `internal/domain/worker/deploy.go`
- `internal/domain/worker/version.go`
- `internal/domain/e2e/service.go`

## 当前回归基线

从 README 所描述的现状看，当前最小可信回归命令是：

```bash
cd /Users/wxy/syl-listing-pro/syl-listing-pro-x && go test ./...
cd /Users/wxy/syl-listing-pro/syl-listing-pro-x && make build
```

如果涉及跨仓联动，再加上：

```bash
SYL_LISTING_PRO_RULES_REPO=/Users/wxy/syl-listing-pro/rules \
/Users/wxy/syl-listing-pro/syl-listing-pro-x/bin/syl-listing-pro-x rules validate --tenant syl
```
