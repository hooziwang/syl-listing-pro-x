package cmd

import "github.com/spf13/cobra"

func newRulesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rules",
		Short: "规则工具链",
	Long: `rules 子命令直接操作 rules/ 仓库，负责租户规则的校验、打包和发布。

建议流程是 rules validate -> rules package -> rules publish。
如果命令涉及签名私钥，优先读取 --private-key，其次读取环境变量；本地命令行未显式配置时，会自动回退到 rules 仓库内开发私钥并临时开启开发模式。`,
		Example: `  syl-listing-pro-x rules validate --tenant syl
  syl-listing-pro-x rules package --tenant syl --private-key /abs/rules.pem
  syl-listing-pro-x rules publish --tenant syl
  syl-listing-pro-x rules publish --tenant syl --worker https://worker.example.test`,
	}
	cmd.CompletionOptions.HiddenDefaultCmd = true
	cmd.AddCommand(newRulesValidateCmd())
	cmd.AddCommand(newRulesPackageCmd())
	cmd.AddCommand(newRulesPublishCmd())
	return cmd
}
