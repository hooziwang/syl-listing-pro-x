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
	cmd.Flags().StringVar(&server, "server", "syl-server", "服务器别名")
	return cmd
}
