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
	var httpsCheckInterval int
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "远程部署 worker",
		Long: `把本地 worker/ 仓库打包上传到远端并执行 docker compose 部署。

命令依赖 SSH，默认服务器别名是 syl-server。部署时会清理远端目录中除 data 和 .env 以外的内容，会根据本地 worker.config.json 生成 .compose.env；如果本地存在 worker/.env，也会同步本地 worker/.env。
部署完成后默认执行内部诊断；除非显式传 --skip-diagnose。`,
		Example: `  syl-listing-pro-x worker deploy --server syl-server
  syl-listing-pro-x worker deploy --server syl-server --skip-build
  syl-listing-pro-x worker deploy --server syl-server --install-docker --stop-legacy`,
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := worker.Service{
				WorkerRepo: paths.WorkerRepo,
				Servers:    worker.DefaultServers(),
			}
			return svc.Deploy(cmd.Context(), worker.DeployInput{
				Server:             server,
				SkipBuild:          skipBuild,
				StopLegacy:         stopLegacy,
				InstallDocker:      installDocker,
				SkipWaitHTTPS:      skipWaitHTTPS,
				HTTPSTimeout:       httpsTimeout,
				HTTPSCheckInterval: httpsCheckInterval,
				SkipDiagnose:       skipDiagnose,
			})
		},
	}
	cmd.Flags().StringVar(&server, "server", "syl-server", "服务器别名")
	cmd.Flags().BoolVar(&skipBuild, "skip-build", false, "跳过镜像构建")
	cmd.Flags().BoolVar(&stopLegacy, "stop-legacy", false, "停止旧 systemd 服务")
	cmd.Flags().BoolVar(&installDocker, "install-docker", false, "缺少 docker 时自动安装")
	cmd.Flags().BoolVar(&skipWaitHTTPS, "skip-wait-https", false, "跳过 HTTPS 就绪等待")
	cmd.Flags().IntVar(&httpsTimeout, "https-timeout", 240, "HTTPS 等待超时秒数")
	cmd.Flags().IntVar(&httpsCheckInterval, "https-interval", 2, "HTTPS 就绪检查间隔秒数")
	cmd.Flags().BoolVar(&skipDiagnose, "skip-diagnose", false, "部署后跳过诊断")
	return cmd
}
