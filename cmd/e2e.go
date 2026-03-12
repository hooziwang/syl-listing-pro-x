package cmd

import "github.com/spf13/cobra"

func newE2ECmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "e2e",
		Short: "端到端验收工具",
	Long: `e2e 子命令用于真实发布验收，会联动 rules 发布、worker 诊断和真实 CLI 执行。

它依赖 PATH 中可执行的 syl-listing-pro，并会把过程日志和产物写入 artifacts 目录。
当前可用用例只有 release-gate、architecture-gate、listing-compliance-gate 和 single-listing-compliance-gate。`,
		Example: `  syl-listing-pro-x e2e list
  syl-listing-pro-x e2e single --tenant syl --input /abs/demo.md
  syl-listing-pro-x e2e run --case release-gate --tenant syl --key <SYL_LISTING_KEY> --admin-token <ADMIN_TOKEN> --input /abs/demo.md --out /abs/out
  syl-listing-pro-x e2e run --case architecture-gate --tenant syl --key <SYL_LISTING_KEY> --admin-token <ADMIN_TOKEN> --private-key /abs/rules.pem --input /abs/demo.md --out /abs/out
  syl-listing-pro-x e2e run --case listing-compliance-gate --tenant syl --key <SYL_LISTING_KEY> --admin-token <ADMIN_TOKEN> --out /abs/out
  syl-listing-pro-x e2e run --case single-listing-compliance-gate --tenant syl --input /abs/demo.md --out /abs/out`,
	}
	cmd.CompletionOptions.HiddenDefaultCmd = true
	cmd.AddCommand(newE2EListCmd())
	cmd.AddCommand(newE2ERunCmd())
	cmd.AddCommand(newE2ESingleCmd())
	return cmd
}
