package cmd

import "github.com/spf13/cobra"

func newWorkerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "worker",
		Short: "worker 运维工具",
	}
	cmd.AddCommand(newWorkerDeployCmd())
	cmd.AddCommand(newWorkerPushEnvCmd())
	cmd.AddCommand(newWorkerDiagnoseCmd())
	cmd.AddCommand(newWorkerDiagnoseExternalCmd())
	cmd.AddCommand(newWorkerLogsCmd())
	return cmd
}
