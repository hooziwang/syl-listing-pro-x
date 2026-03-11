package cmd

import (
	"github.com/hooziwang/syl-listing-pro-x/internal/config"
	"github.com/spf13/cobra"
)

var paths = config.DefaultPaths()

var rootCmd = newRootCmd()

func Execute() error {
	return rootCmd.Execute()
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "syl-listing-pro-x",
		Short: "syl-listing-pro-x 工程侧工具集合",
		Long: `syl-listing-pro-x 是面向开发、运维、发版和工程验收的工程侧工具集合。

它不是终端用户直接使用的 Listing 生成 CLI，而是用来编排 sibling 仓库和线上 worker 的工程入口：
- rules：校验、打包、发布租户规则
- worker：部署、诊断、日志与远端版本核对
- e2e：真实 worker 和真实 CLI 的发布验收

默认会直接操作 /Users/wxy/syl-listing-pro 下的 rules/、worker/ 和 syl-listing-pro-x/。
如果当前位于 .worktrees/<name> 中，且 sibling 仓库存在同名 worktree，会自动联动切换。`,
		Example: `  syl-listing-pro-x rules validate --tenant syl
  syl-listing-pro-x worker deploy --server syl-server
  syl-listing-pro-x e2e run --case release-gate --tenant syl --key <SYL_LISTING_KEY> --admin-token <ADMIN_TOKEN> --input /abs/demo.md --out /abs/out`,
	}
	cmd.CompletionOptions.HiddenDefaultCmd = true
	cmd.AddCommand(newRulesCmd())
	cmd.AddCommand(newWorkerCmd())
	cmd.AddCommand(newE2ECmd())
	return cmd
}

func init() {
}
