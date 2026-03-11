package cmd

import (
	"github.com/hooziwang/syl-listing-pro-x/internal/domain/worker"
	"github.com/spf13/cobra"
)

func newWorkerDiagnoseCmd() *cobra.Command {
	var server string
	cmd := &cobra.Command{
		Use:   "diagnose",
		Short: "在远端执行内部诊断",
		Long: `通过 SSH 在远端执行内部诊断脚本，检查 worker 运行状态。

当前会验证核心容器是否运行、worker-api /healthz 是否通过、认证换票与规则解析是否正常、Redis 是否返回 PONG，以及 nginx 配置是否有效。`,
		Example: `  syl-listing-pro-x worker diagnose --server syl-server`,
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := worker.Service{
				WorkerRepo: paths.WorkerRepo,
				Servers:    worker.DefaultServers(),
			}
			return svc.Diagnose(cmd.Context(), server)
		},
	}
	cmd.Flags().StringVar(&server, "server", "syl-server", "服务器别名")
	return cmd
}
