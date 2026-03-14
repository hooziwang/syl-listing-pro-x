# syl-listing-pro-x

`syl-listing-pro-x` 是 `syl-listing-pro` 的工程侧控制台。

它把 3 类原本分散在不同仓库、不同阶段的动作收拢到一个 Go CLI 里：

- `rules`：校验、签名、打包、发布租户规则
- `worker`：远端部署、诊断、日志、版本核对
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
4. 将 `worker` 仓打包上传到远端机器并完成部署。
5. 对远端 worker 做内部诊断、外部诊断和日志查看。
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
│   └── worker/             # 部署、日志、诊断、版本核对
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
- `make build`：生成 `bin/syl-listing-pro-x`
- `make`：先测试，再构建

产物路径：

```bash
bin/syl-listing-pro-x
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
syl-listing-pro-x worker deploy --server syl-server
syl-listing-pro-x worker push-env --server syl-server
syl-listing-pro-x worker diagnose --server syl-server
syl-listing-pro-x worker diagnose-external --key <SYL_LISTING_KEY>
syl-listing-pro-x worker check-remote-version
syl-listing-pro-x worker logs --server syl-server --service worker-api --tail 50 --since 10m
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

## worker：远端运维工具

`worker` 领域代码位于 `internal/domain/worker`，它把 `worker` 仓和远端机器连起来。

默认服务器定义：

```text
name: syl-server
host: 43.135.112.167
user: ubuntu
port: 22
dir: /opt/syl-listing-worker
```

远端执行依赖本机已有：

- `ssh`
- `scp`
- 访问目标机器的权限

### `worker deploy`

```bash
syl-listing-pro-x worker deploy --server syl-server
```

部署流程来自 `internal/domain/worker/deploy.go`：

1. 读取 `worker.config.json`
2. 将 `worker` 仓打成临时 tar.gz
3. 清理远端目录中除 `data/` 和 `.env` 之外的内容
4. 上传归档到远端 `/tmp/...tar.gz`
5. 如果本地 `worker/.env` 存在，则同步到远端
6. 写入 `data/runtime/version.json`
7. 通过 `docker compose --env-file .compose.env up -d [--build]` 启动
8. 可选等待 HTTPS 证书就绪
9. 默认部署完成后继续执行内部诊断

会自动打进归档的内容：

- `worker` 仓绝大多数源码和部署文件
- 自动生成的 `.compose.env`

不会打进归档的内容：

- `.git`
- `node_modules`
- `dist`
- `.env`
- `data`

`.compose.env` 由 `worker.config.json` 生成，当前包含：

- `DOMAIN`
- `LETSENCRYPT_EMAIL`

常用参数：

- `--skip-build`：远端只 `up -d`，不重建镜像
- `--stop-legacy`：停止旧的 systemd 服务
- `--install-docker`：远端缺 Docker 时自动安装
- `--skip-wait-https`：不等 HTTPS 证书 ready
- `--https-timeout`：等待超时秒数，默认 `240`
- `--https-interval`：HTTPS 就绪检查间隔秒数，默认 `2`
- `--skip-diagnose`：部署后不跑内部诊断

### `worker push-env`

```bash
syl-listing-pro-x worker push-env --server syl-server
```

用途很单纯：

1. 从本地 `worker/.env` 读取环境变量
2. 上传到远端 `/tmp/syl-listing-worker.env.tmp`
3. 覆盖远端 `${server.Dir}/.env`
4. 仅重启 `worker-api` 和 `worker-runner`

这个命令不会重传代码，适合只改密钥或配置。

### `worker diagnose`

```bash
syl-listing-pro-x worker diagnose --server syl-server
```

这是“远端内部诊断”，通过 SSH 到服务器上执行检查脚本。  
它当前会验证：

- `redis`、`worker-api`、`worker-runner`、`nginx`、`certbot` 都在运行
- `worker-api` 容器内 `http://127.0.0.1:8080/healthz` 正常
- healthz 对 optional provider 采取 required-aware 判断
- `.env` 里能解析出 `SYL_LISTING_KEYS`
- 能完成 `/v1/auth/exchange`
- 能完成 `/v1/rules/resolve`
- Redis `PING` 返回 `PONG`
- `nginx -t` 通过
- Let’s Encrypt 证书文件存在或至少路径可检查

底层执行统一使用 compose fallback：

```text
先尝试 docker compose --env-file .compose.env ...
失败后再尝试 sudo -n docker compose --env-file .compose.env ...
```

### `worker diagnose-external`

```bash
syl-listing-pro-x worker diagnose-external --base-url https://worker.aelus.tech --key <SYL_LISTING_KEY>
```

必须显式传入 `--base-url`，不会再回退到默认 worker 地址。

这是“外部黑盒诊断”，直接打公网接口，不走 SSH。  
它按顺序检查：

1. `GET /healthz`
2. `POST /v1/auth/exchange`
3. `GET /v1/rules/resolve?current=`
4. `POST /v1/rules/refresh`
5. 下载 `download_url`

如果带上 `--with-generate`，还会额外：

6. `POST /v1/generate`
7. 订阅 `GET /v1/jobs/:jobId/events`
8. 读取 `GET /v1/jobs/:jobId/result`
9. 确认 `en_markdown` 和 `cn_markdown` 都非空

常用参数：

- `--key`：必填
- `--base-url`：必填，明确指定 worker 对外地址
- `--with-generate`：额外验证真实生成链路
- `--timeout`：生成事件流超时，默认 `5m`

### `worker check-remote-version`

```bash
syl-listing-pro-x worker check-remote-version --base-url https://worker.aelus.tech
```

它会：

1. 读取本地 `worker` 仓 `git rev-parse --short HEAD`
2. 调用 `GET /v1/admin/version`
3. 对比远端 `git_commit`
4. 输出远端的 `build_time`、`deployed_at`、`rules_versions`

如果没有传 `--admin-token`，会回退读取：

```text
~/.syl-listing-pro-x/.env
```

要求其中存在：

```bash
ADMIN_TOKEN=...
```

只要远端 commit 与本地 commit 不一致，命令就会返回错误。

### `worker logs`

```bash
syl-listing-pro-x worker logs --server syl-server
syl-listing-pro-x worker logs --server syl-server --service worker-api
syl-listing-pro-x worker logs --server syl-server --service worker-api --service nginx --tail 50 --since 10m
syl-listing-pro-x worker logs --server syl-server --no-follow
```

这是远端实时日志透传，底层会走 `ssh` + `docker compose logs`。

参数语义：

- `--service`：可重复传多个容器名
- `--tail`：默认 `200`
- `--since`：支持 `10m`、`1h`、RFC3339 时间
- `--no-follow`：只拉一次，不持续跟随

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
3. 调用 `worker diagnose-external`
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
4. 调用 `worker diagnose-external`
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
2. 运行 `worker diagnose-external`
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
cd /Users/wxy/syl-listing-pro/worker
make test
make build

cd /Users/wxy/syl-listing-pro/syl-listing-pro-x
bin/syl-listing-pro-x worker deploy --server syl-server
bin/syl-listing-pro-x worker diagnose --server syl-server
bin/syl-listing-pro-x worker check-remote-version
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
- `worker deploy` / `push-env` / `diagnose` / `logs` 都依赖本机有 `ssh`、`scp`。
- `worker deploy` 会保留远端 `data/` 和 `.env`，其余目录会被清空后再同步。
- `worker diagnose-external` 必须显式传入 `--base-url`，避免误连默认环境。
- `worker check-remote-version` 会把本地 `worker` 仓 HEAD 与远端 `/v1/admin/version` 对比，不是比较 `syl-listing-pro-x` 自己的 commit。
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
- `internal/domain/worker/deploy.go`
- `internal/domain/worker/diagnose.go`
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
