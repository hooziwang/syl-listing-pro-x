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
