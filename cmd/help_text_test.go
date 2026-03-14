package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func renderHelp(t *testing.T, build func() *cobra.Command, args ...string) string {
	t.Helper()
	cmd := build()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(append(args, "--help"))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	return out.String()
}

func assertContainsAll(t *testing.T, output string, parts ...string) {
	t.Helper()
	for _, part := range parts {
		if !strings.Contains(output, part) {
			t.Fatalf("help output missing %q\noutput:\n%s", part, output)
		}
	}
}

func TestRootHelpIncludesOperationalGuidance(t *testing.T) {
	output := renderHelp(t, func() *cobra.Command { return rootCmd })
	assertContainsAll(t, output,
		"工程侧工具集合",
		"面向开发、运维、发版和工程验收",
		"rules：校验、打包、发布租户规则",
		"worker：部署、诊断、日志与远端版本核对",
		"e2e：真实 worker 和真实 CLI 的发布验收",
	)
}

func TestRulesHelpIncludesWorkflowContext(t *testing.T) {
	output := renderHelp(t, newRulesCmd)
	assertContainsAll(t, output,
		"操作 rules/ 仓库",
		"rules validate -> rules package -> rules publish",
		"签名私钥",
	)
}

func TestWorkerHelpIncludesPrerequisites(t *testing.T) {
	output := renderHelp(t, newWorkerCmd)
	assertContainsAll(t, output,
		"依赖 SSH",
		"Docker Compose",
		"默认服务器别名是 syl-server",
	)
}

func TestE2EHelpIncludesExecutionContext(t *testing.T) {
	output := renderHelp(t, newE2ECmd)
	assertContainsAll(t, output,
		"真实发布验收",
		"依赖 PATH 中可执行的 syl-listing-pro",
		"release-gate",
		"architecture-gate",
		"listing-compliance-gate",
		"single-listing-compliance-gate",
		"single",
	)
}

func TestLeafCommandHelpExplainsBehavior(t *testing.T) {
	cases := []struct {
		name  string
		build func() *cobra.Command
		parts []string
	}{
		{
			name:  "rules-package",
			build: newRulesPackageCmd,
			parts: []string{
				"先执行规则校验，再生成 rules.tar.gz、manifest.json 和公钥文件",
				"stdout 输出产物目录路径",
				"SYL_LISTING_RULES_PRIVATE_KEY",
			},
		},
		{
			name:  "rules-publish",
			build: newRulesPublishCmd,
			parts: []string{
				"会先打包，再调用 worker 管理接口发布",
				"/v1/admin/tenant-rules/publish",
				"--admin-token",
			},
		},
		{
			name:  "worker-deploy",
			build: newWorkerDeployCmd,
			parts: []string{
				"会清理远端目录中除 data 和 .env 以外的内容",
				"同步本地 worker/.env",
				"部署完成后默认执行内部诊断",
			},
		},
		{
			name:  "worker-check-remote-version",
			build: newWorkerCheckRemoteVersionCmd,
			parts: []string{
				"对比本地 worker git commit 与远端 /v1/admin/version",
				"必须显式传入 --base-url",
				"~/.syl-listing-pro-x/.env",
				"远端 rules_versions",
			},
		},
		{
			name:  "e2e-run",
			build: newE2ERunCmd,
			parts: []string{
				"会先发布规则，再诊断 worker，最后调用 syl-listing-pro",
				"stdout 只打印 artifacts 目录路径",
				"可用用例：release-gate, architecture-gate, listing-compliance-gate, single-listing-compliance-gate",
			},
		},
		{
			name:  "e2e-single",
			build: newE2ESingleCmd,
			parts: []string{
				"单文件真实回归验收入口",
				"固定执行 single-listing-compliance-gate",
				"如果不传 --out，会默认落到 syl-listing-pro-x/out/<artifacts-id>",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			output := renderHelp(t, tc.build)
			assertContainsAll(t, output, tc.parts...)
		})
	}
}
