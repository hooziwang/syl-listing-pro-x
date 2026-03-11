package cmd

import "github.com/spf13/cobra"

func newWorkerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "worker",
		Short: "worker 运维工具",
		Long: `worker 子命令直接操作 worker/ 仓库或远端 worker 服务，适合部署、诊断、日志排障和版本核对。

这一组命令依赖 SSH 和远端 Docker Compose 环境。默认服务器别名是 syl-server。
其中 deploy / push-env / diagnose / logs 会访问远端机器，check-remote-version / diagnose-external 会访问 worker HTTP 接口。`,
		Example: `  syl-listing-pro-x worker deploy --server syl-server
  syl-listing-pro-x worker diagnose-external --base-url https://worker.example.test --key <SYL_LISTING_KEY>
  syl-listing-pro-x worker check-remote-version --base-url https://worker.example.test --admin-token <ADMIN_TOKEN>`,
	}
	cmd.CompletionOptions.HiddenDefaultCmd = true
	cmd.AddCommand(newWorkerDeployCmd())
	cmd.AddCommand(newWorkerPushEnvCmd())
	cmd.AddCommand(newWorkerDiagnoseCmd())
	cmd.AddCommand(newWorkerDiagnoseExternalCmd())
	cmd.AddCommand(newWorkerCheckRemoteVersionCmd())
	cmd.AddCommand(newWorkerLogsCmd())
	return cmd
}
