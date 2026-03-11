package cmd

import (
	"fmt"
	"sort"

	"github.com/hooziwang/syl-listing-pro-x/internal/domain/worker"
	"github.com/spf13/cobra"
)

func newWorkerCheckRemoteVersionCmd() *cobra.Command {
	var baseURL string
	var adminToken string
	cmd := &cobra.Command{
		Use:   "check-remote-version",
		Short: "检查远端 worker 是否已部署为本地最新版本",
		Long: `对比本地 worker git commit 与远端 /v1/admin/version，确认远端是否已部署为本地最新版本。

成功时会打印本地 commit、远端 commit、远端 build_time、deployed_at 和远端 rules_versions。
未传 --admin-token 时，会回退读取 ~/.syl-listing-pro-x/.env 中的 ADMIN_TOKEN。`,
		Example: `  syl-listing-pro-x worker check-remote-version --base-url https://worker.example.test --admin-token <ADMIN_TOKEN>
  syl-listing-pro-x worker check-remote-version --base-url https://worker.example.test`,
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := worker.Service{
				WorkerRepo: paths.WorkerRepo,
			}
			result, err := svc.CheckRemoteVersion(cmd.Context(), worker.CheckRemoteVersionInput{
				BaseURL:    baseURL,
				AdminToken: adminToken,
			})
			if result.LocalGitCommit != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "本地 git commit: %s\n", result.LocalGitCommit)
			}
			if result.Remote.GitCommit != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "远端 git commit: %s\n", result.Remote.GitCommit)
			}
			if result.Remote.BuildTime != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "远端 build_time: %s\n", result.Remote.BuildTime)
			}
			if result.Remote.DeployedAt != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "远端 deployed_at: %s\n", result.Remote.DeployedAt)
			}
			if len(result.Remote.RulesVersions) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "远端 rules_versions:")
				keys := make([]string, 0, len(result.Remote.RulesVersions))
				for tenantID := range result.Remote.RulesVersions {
					keys = append(keys, tenantID)
				}
				sort.Strings(keys)
				for _, tenantID := range keys {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s=%s\n", tenantID, result.Remote.RulesVersions[tenantID])
				}
			}
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "远端 worker 已是最新版本")
			return nil
		},
	}
	cmd.Flags().StringVar(&baseURL, "base-url", paths.WorkerURL, "worker 地址")
	cmd.Flags().StringVar(&adminToken, "admin-token", "", "ADMIN_TOKEN，默认从 ~/.syl-listing-pro-x/.env 读取")
	return cmd
}
