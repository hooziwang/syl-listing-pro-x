package cmd

import "github.com/spf13/cobra"

func newWorkerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "worker",
		Short: "worker 正式发布入口",
		Long: `worker 子命令只保留正式发布入口。

发布流程统一走 worker release：
1. 校验本地 worker 仓工作区干净。
2. 执行 npm test。
3. 创建并推送版本 tag。
4. 从 tag 对应代码部署远端 worker。
5. 核对远端运行版本。

正式发布时必须显式传入 --server 和 --version。`,
		Example: `  syl-listing-pro-x worker release --server syl-server --version v0.1.3`,
	}
	cmd.CompletionOptions.HiddenDefaultCmd = true
	cmd.AddCommand(newWorkerReleaseCmd())
	return cmd
}
