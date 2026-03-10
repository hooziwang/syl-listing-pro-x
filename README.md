# syl-listing-pro-x

`syl-listing-pro-x` 是 `syl-listing-pro` 的工程侧工具集合。

当前提供 3 组子命令：

- `rules`：规则校验、打包、发布
- `worker`：worker 部署、诊断、`.env` 下发、日志
- `e2e`：发版前真实 LLM 端到端验收

## 默认路径

程序默认直接操作以下目录：

- `rules`: `/Users/wxy/syl-listing-pro/rules`
- `worker`: `/Users/wxy/syl-listing-pro/worker`

## 构建

```bash
make
```

生成二进制：

```bash
bin/syl-listing-pro-x
```

## 规则命令

```bash
syl-listing-pro-x rules validate --tenant syl
syl-listing-pro-x rules package --tenant syl
syl-listing-pro-x rules publish --tenant syl --admin-token <token>
```

## worker 命令

```bash
syl-listing-pro-x worker deploy --server syl-server
syl-listing-pro-x worker push-env --server syl-server
syl-listing-pro-x worker diagnose --server syl-server
syl-listing-pro-x worker diagnose-external --key <SYL_LISTING_KEY>
syl-listing-pro-x worker logs --server syl-server
syl-listing-pro-x worker logs --server syl-server --service worker-api
syl-listing-pro-x worker logs --server syl-server --service worker-api --service nginx --tail 50 --since 10m
syl-listing-pro-x worker logs --server syl-server --service worker-api --tail 20 --since 1h --no-follow
```

## e2e 命令

```bash
syl-listing-pro-x e2e list
syl-listing-pro-x e2e run --case release-gate --tenant syl --key <SYL_LISTING_KEY> --admin-token <ADMIN_TOKEN> --input /abs/demo.md --out /abs/out
```
