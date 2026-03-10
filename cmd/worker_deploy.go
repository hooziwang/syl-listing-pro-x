package cmd

import (
	"github.com/hooziwang/syl-listing-pro-x/internal/domain/worker"
	"github.com/spf13/cobra"
)

func newWorkerDeployCmd() *cobra.Command {
	var server string
	var skipDiagnose bool
	var skipBuild bool
	var stopLegacy bool
	var installDocker bool
	var skipWaitHTTPS bool
	var httpsTimeout int
	var httpsInterval int
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "远程部署 worker",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := worker.Service{
				WorkerRepo: paths.WorkerRepo,
				Servers:    worker.DefaultServers(),
			}
			return svc.Deploy(cmd.Context(), worker.DeployInput{
				Server:        server,
				SkipBuild:     skipBuild,
				StopLegacy:    stopLegacy,
				InstallDocker: installDocker,
				SkipWaitHTTPS: skipWaitHTTPS,
				HTTPSTimeout:  httpsTimeout,
				HTTPSInterval: httpsInterval,
				SkipDiagnose:  skipDiagnose,
			})
		},
	}
	cmd.Flags().StringVar(&server, "server", "syl-server", "服务器别名")
	cmd.Flags().BoolVar(&skipBuild, "skip-build", false, "跳过镜像构建")
	cmd.Flags().BoolVar(&stopLegacy, "stop-legacy", false, "停止旧 systemd 服务")
	cmd.Flags().BoolVar(&installDocker, "install-docker", false, "缺少 docker 时自动安装")
	cmd.Flags().BoolVar(&skipWaitHTTPS, "skip-wait-https", false, "跳过 HTTPS 就绪等待")
	cmd.Flags().IntVar(&httpsTimeout, "https-timeout", 240, "HTTPS 等待超时秒数")
	cmd.Flags().IntVar(&httpsInterval, "https-interval", 2, "HTTPS 轮询间隔秒数")
	cmd.Flags().BoolVar(&skipDiagnose, "skip-diagnose", false, "部署后跳过诊断")
	return cmd
}
