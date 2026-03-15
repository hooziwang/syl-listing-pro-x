package cmd

import (
	"github.com/hooziwang/syl-listing-pro-x/internal/config"
	"github.com/spf13/cobra"
)

var paths = config.DefaultPaths()
var showVersion bool

var rootCmd = newRootCmd()

func Execute() error {
	return rootCmd.Execute()
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "syl-listing-pro-x",
		Short: "syl-listing-pro-x 工程侧工具集合",
		Args:  cobra.ArbitraryArgs,
		Long: `syl-listing-pro-x 是面向开发、运维、发版和工程验收的工程侧工具集合。

		它不是终端用户直接使用的 Listing 生成 CLI，而是用来编排 sibling 仓库和线上 worker 的工程入口：
		- rules：校验、打包、发布租户规则
		- worker：唯一正式发布入口
		- e2e：真实 worker 和真实 CLI 的发布验收

		默认会直接操作 /Users/wxy/syl-listing-pro 下的 rules/、worker/ 和 syl-listing-pro-x/。
		如果当前位于 .worktrees/<name> 中，且 sibling 仓库存在同名 worktree，会自动联动切换。`,
		Example: `  syl-listing-pro-x rules validate --tenant syl
	  syl-listing-pro-x worker release --server syl-server --version v0.1.3
	  syl-listing-pro-x e2e run --case release-gate --tenant syl --key <SYL_LISTING_KEY> --admin-token <ADMIN_TOKEN> --input /abs/demo.md --out /abs/out`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if showVersion {
				printVersion(cmd.OutOrStdout())
				return nil
			}
			return cmd.Help()
		},
	}
	cmd.CompletionOptions.HiddenDefaultCmd = true
	cmd.PersistentFlags().BoolVarP(&showVersion, "version", "v", false, "显示版本信息")
	cmd.AddCommand(newRulesCmd())
	cmd.AddCommand(newWorkerCmd())
	cmd.AddCommand(newE2ECmd())
	cmd.AddCommand(newVersionCmd())
	return cmd
}

func init() {
}
