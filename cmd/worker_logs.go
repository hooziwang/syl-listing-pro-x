package cmd

import (
	"github.com/hooziwang/syl-listing-pro-x/internal/domain/worker"
	"github.com/spf13/cobra"
)

func newWorkerLogsCmd() *cobra.Command {
	var server string
	var services []string
	var tail int
	var since string
	var noFollow bool
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "查看远端 worker 日志",
		Long: `通过 SSH 到远端执行 docker compose logs，实时查看 worker 日志。

必须显式传入 --server。默认会持续跟随输出，除非传 --no-follow。可以用 --service 重复过滤容器，用 --since 和 --tail 控制时间范围和最近行数。`,
		Example: `  syl-listing-pro-x worker logs --server syl-server
  syl-listing-pro-x worker logs --server syl-server --service worker-api
  syl-listing-pro-x worker logs --server syl-server --service worker-api --service nginx --tail 50 --since 10m
  syl-listing-pro-x worker logs --server syl-server --service worker-api --tail 20 --since 1h --no-follow`,
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := worker.Service{
				WorkerRepo: paths.WorkerRepo,
				Servers:    worker.DefaultServers(),
			}
			return svc.Logs(cmd.Context(), worker.LogsInput{
				Server:   server,
				Services: services,
				Tail:     tail,
				Since:    since,
				NoFollow: noFollow,
			})
		},
	}
	cmd.Flags().StringVar(&server, "server", "", "服务器别名")
	_ = cmd.MarkFlagRequired("server")
	cmd.Flags().StringArrayVar(&services, "service", nil, "仅查看指定容器，可重复传入")
	cmd.Flags().IntVar(&tail, "tail", 200, "启动时先输出最近多少行")
	cmd.Flags().StringVar(&since, "since", "", "仅查看某个时间点之后的日志，例如 10m、1h、2026-03-10T23:00:00")
	cmd.Flags().BoolVar(&noFollow, "no-follow", false, "只输出一次后退出，不持续跟随")
	return cmd
}
