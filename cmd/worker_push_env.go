package cmd

import (
	"github.com/hooziwang/syl-listing-pro-x/internal/domain/worker"
	"github.com/spf13/cobra"
)

func newWorkerPushEnvCmd() *cobra.Command {
	var server string
	cmd := &cobra.Command{
		Use:   "push-env",
		Short: "下发 .env 并重启 worker",
		Long: `把本地 worker/.env 同步到远端 worker 目录，并重启 worker-api 与 worker-runner。

适合只更新运行参数、不改代码或镜像的场景。命令依赖 SSH，必须显式传入 --server。`,
		Example: `  syl-listing-pro-x worker push-env --server syl-server`,
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := worker.Service{
				WorkerRepo: paths.WorkerRepo,
				Servers:    worker.DefaultServers(),
			}
			return svc.PushEnv(cmd.Context(), worker.PushEnvInput{
				Server: server,
			})
		},
	}
	cmd.Flags().StringVar(&server, "server", "", "服务器别名")
	_ = cmd.MarkFlagRequired("server")
	return cmd
}
