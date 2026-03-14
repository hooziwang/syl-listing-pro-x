package cmd

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hooziwang/syl-listing-pro-x/internal/domain/e2e"
	"github.com/spf13/cobra"
)

func withPathsForTest(t *testing.T, fn func()) {
	t.Helper()
	oldPaths := paths
	t.Cleanup(func() {
		paths = oldPaths
	})
	fn()
}

func withE2ERunnerFactoryForTest(t *testing.T, factory func(stderr io.Writer) e2eRunner, fn func()) {
	t.Helper()
	oldFactory := newE2ERunner
	newE2ERunner = factory
	t.Cleanup(func() {
		newE2ERunner = oldFactory
	})
	fn()
}

type fakeE2ERunner struct {
	listCasesResult []string
	runResult       e2e.RunResult
	runErr          error
	lastRunInput    e2e.RunInput
	lastStderr      io.Writer
}

func (f *fakeE2ERunner) ListCases() []string {
	return f.listCasesResult
}

func (f *fakeE2ERunner) Run(_ context.Context, in e2e.RunInput) (e2e.RunResult, error) {
	f.lastRunInput = in
	return f.runResult, f.runErr
}

func TestE2ERunCmdUsesPathsWorkerURLDefault(t *testing.T) {
	withPathsForTest(t, func() {
		paths.WorkerURL = "https://worker.from.paths"
		cmd := newE2ERunCmd()
		if got := cmd.Flag("worker").DefValue; got != paths.WorkerURL {
			t.Fatalf("worker default = %q, want %q", got, paths.WorkerURL)
		}
	})
}

func TestE2ESingleCmdUsesPathsWorkerURLDefault(t *testing.T) {
	withPathsForTest(t, func() {
		paths.WorkerURL = "https://worker.from.paths"
		cmd := newE2ESingleCmd()
		if got := cmd.Flag("worker").DefValue; got != paths.WorkerURL {
			t.Fatalf("worker default = %q, want %q", got, paths.WorkerURL)
		}
	})
}

func TestE2ERunCmdExposesPrintPathContextFlag(t *testing.T) {
	withPathsForTest(t, func() {
		cmd := newE2ERunCmd()
		flag := cmd.Flag("print-path-context")
		if flag == nil {
			t.Fatal("print-path-context flag missing")
		}
		if got := flag.DefValue; got != "false" {
			t.Fatalf("print-path-context default = %q, want false", got)
		}
	})
}

func TestE2ESingleCmdExposesPrintPathContextFlag(t *testing.T) {
	withPathsForTest(t, func() {
		cmd := newE2ESingleCmd()
		flag := cmd.Flag("print-path-context")
		if flag == nil {
			t.Fatal("print-path-context flag missing")
		}
		if got := flag.DefValue; got != "false" {
			t.Fatalf("print-path-context default = %q, want false", got)
		}
	})
}

func TestE2ERunCmdPassesPrintPathContextAndErrWriter(t *testing.T) {
	withPathsForTest(t, func() {
		runner := &fakeE2ERunner{runResult: e2e.RunResult{ArtifactsDir: "/tmp/artifacts-run"}}
		withE2ERunnerFactoryForTest(t, func(stderr io.Writer) e2eRunner {
			runner.lastStderr = stderr
			return runner
		}, func() {
			cmd := newE2ERunCmd()
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)
			cmd.SetArgs([]string{
				"--case", "release-gate",
				"--tenant", "syl",
				"--key", "key",
				"--admin-token", "admin",
				"--input", "/tmp/demo.md",
				"--out", "/tmp/out",
				"--print-path-context",
			})

			if err := cmd.Execute(); err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if !runner.lastRunInput.PrintPathContext {
				t.Fatal("PrintPathContext=false, want true")
			}
			if runner.lastStderr != &stderr {
				t.Fatal("stderr writer was not forwarded to e2e service")
			}
			if got := stdout.String(); got != "/tmp/artifacts-run\n" {
				t.Fatalf("stdout = %q", got)
			}
			if got := stderr.String(); got != "" {
				t.Fatalf("stderr = %q, want empty", got)
			}
		})
	})
}

func TestE2ESingleCmdPassesPrintPathContextAndErrWriter(t *testing.T) {
	withPathsForTest(t, func() {
		runner := &fakeE2ERunner{runResult: e2e.RunResult{ArtifactsDir: "/tmp/artifacts-single"}}
		withE2ERunnerFactoryForTest(t, func(stderr io.Writer) e2eRunner {
			runner.lastStderr = stderr
			return runner
		}, func() {
			cmd := newE2ESingleCmd()
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)
			cmd.SetArgs([]string{
				"--tenant", "syl",
				"--key", "key",
				"--admin-token", "admin",
				"--input", "/tmp/demo.md",
				"--print-path-context",
			})

			if err := cmd.Execute(); err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if !runner.lastRunInput.PrintPathContext {
				t.Fatal("PrintPathContext=false, want true")
			}
			if runner.lastStderr != &stderr {
				t.Fatal("stderr writer was not forwarded to e2e service")
			}
			if got := stdout.String(); got != "/tmp/artifacts-single\n" {
				t.Fatalf("stdout = %q", got)
			}
			if got := stderr.String(); got != "" {
				t.Fatalf("stderr = %q, want empty", got)
			}
		})
	})
}

func TestRulesPublishCmdUsesPathsWorkerURLDefault(t *testing.T) {
	withPathsForTest(t, func() {
		paths.WorkerURL = "https://worker.from.paths"
		cmd := newRulesPublishCmd()
		if got := cmd.Flag("worker").DefValue; got != paths.WorkerURL {
			t.Fatalf("worker default = %q, want %q", got, paths.WorkerURL)
		}
	})
}

func TestRulesPackageCmdDoesNotDefaultToBundledPrivateKey(t *testing.T) {
	withPathsForTest(t, func() {
		cmd := newRulesPackageCmd()
		if got := cmd.Flag("private-key").DefValue; got != "" {
			t.Fatalf("private-key default = %q, want empty", got)
		}
	})
}

func TestRulesPublishCmdDoesNotDefaultToBundledPrivateKey(t *testing.T) {
	withPathsForTest(t, func() {
		cmd := newRulesPublishCmd()
		if got := cmd.Flag("private-key").DefValue; got != "" {
			t.Fatalf("private-key default = %q, want empty", got)
		}
	})
}

func TestWorkerCheckRemoteVersionCmdUsesPathsWorkerURLDefault(t *testing.T) {
	withPathsForTest(t, func() {
		paths.WorkerURL = "https://worker.from.paths"
		cmd := newWorkerCheckRemoteVersionCmd()
		if got := cmd.Flag("base-url").DefValue; got != paths.WorkerURL {
			t.Fatalf("base-url default = %q, want %q", got, paths.WorkerURL)
		}
	})
}

func TestWorkerDiagnoseExternalCmdUsesPathsWorkerURLDefault(t *testing.T) {
	withPathsForTest(t, func() {
		paths.WorkerURL = "https://worker.from.paths"
		cmd := newWorkerDiagnoseExternalCmd()
		if got := cmd.Flag("base-url").DefValue; got != paths.WorkerURL {
			t.Fatalf("base-url default = %q, want %q", got, paths.WorkerURL)
		}
	})
}

func TestE2ERunCmdLoadsKeyAndAdminTokenDefaultsFromUserEnv(t *testing.T) {
	withPathsForTest(t, func() {
		homeDir := t.TempDir()
		t.Setenv("HOME", homeDir)

		if err := os.MkdirAll(filepath.Join(homeDir, ".syl-listing-pro"), 0o755); err != nil {
			t.Fatalf("MkdirAll key env dir error = %v", err)
		}
		if err := os.MkdirAll(filepath.Join(homeDir, ".syl-listing-pro-x"), 0o755); err != nil {
			t.Fatalf("MkdirAll admin env dir error = %v", err)
		}
		if err := os.WriteFile(
			filepath.Join(homeDir, ".syl-listing-pro", ".env"),
			[]byte("SYL_LISTING_KEY=key-from-home-env\n"),
			0o644,
		); err != nil {
			t.Fatalf("WriteFile key env error = %v", err)
		}
		if err := os.WriteFile(
			filepath.Join(homeDir, ".syl-listing-pro-x", ".env"),
			[]byte("ADMIN_TOKEN=admin-from-home-env\n"),
			0o644,
		); err != nil {
			t.Fatalf("WriteFile admin env error = %v", err)
		}

		cmd := newE2ERunCmd()
		if got := cmd.Flag("key").DefValue; got != "key-from-home-env" {
			t.Fatalf("key default = %q, want %q", got, "key-from-home-env")
		}
		if got := cmd.Flag("admin-token").DefValue; got != "admin-from-home-env" {
			t.Fatalf("admin-token default = %q, want %q", got, "admin-from-home-env")
		}
	})
}

func TestE2ESingleCmdLoadsKeyAndAdminTokenDefaultsFromUserEnv(t *testing.T) {
	withPathsForTest(t, func() {
		homeDir := t.TempDir()
		t.Setenv("HOME", homeDir)

		if err := os.MkdirAll(filepath.Join(homeDir, ".syl-listing-pro"), 0o755); err != nil {
			t.Fatalf("MkdirAll key env dir error = %v", err)
		}
		if err := os.MkdirAll(filepath.Join(homeDir, ".syl-listing-pro-x"), 0o755); err != nil {
			t.Fatalf("MkdirAll admin env dir error = %v", err)
		}
		if err := os.WriteFile(
			filepath.Join(homeDir, ".syl-listing-pro", ".env"),
			[]byte("SYL_LISTING_KEY=key-from-home-env\n"),
			0o644,
		); err != nil {
			t.Fatalf("WriteFile key env error = %v", err)
		}
		if err := os.WriteFile(
			filepath.Join(homeDir, ".syl-listing-pro-x", ".env"),
			[]byte("ADMIN_TOKEN=admin-from-home-env\n"),
			0o644,
		); err != nil {
			t.Fatalf("WriteFile admin env error = %v", err)
		}

		cmd := newE2ESingleCmd()
		if got := cmd.Flag("key").DefValue; got != "key-from-home-env" {
			t.Fatalf("key default = %q, want %q", got, "key-from-home-env")
		}
		if got := cmd.Flag("admin-token").DefValue; got != "admin-from-home-env" {
			t.Fatalf("admin-token default = %q, want %q", got, "admin-from-home-env")
		}
	})
}

func TestE2ERunCmdLeavesKeyAndAdminTokenDefaultsEmptyWithoutUserEnv(t *testing.T) {
	withPathsForTest(t, func() {
		t.Setenv("HOME", t.TempDir())

		cmd := newE2ERunCmd()
		if got := cmd.Flag("key").DefValue; got != "" {
			t.Fatalf("key default = %q, want empty", got)
		}
		if got := cmd.Flag("admin-token").DefValue; got != "" {
			t.Fatalf("admin-token default = %q, want empty", got)
		}
	})
}

func TestE2ESingleCmdLeavesKeyAndAdminTokenDefaultsEmptyWithoutUserEnv(t *testing.T) {
	withPathsForTest(t, func() {
		t.Setenv("HOME", t.TempDir())

		cmd := newE2ESingleCmd()
		if got := cmd.Flag("key").DefValue; got != "" {
			t.Fatalf("key default = %q, want empty", got)
		}
		if got := cmd.Flag("admin-token").DefValue; got != "" {
			t.Fatalf("admin-token default = %q, want empty", got)
		}
	})
}

func TestE2ERunCmdDoesNotMarkEnvBackedFlagsAsRequired(t *testing.T) {
	withPathsForTest(t, func() {
		cmd := newE2ERunCmd()
		if _, ok := cmd.Flag("key").Annotations[cobra.BashCompOneRequiredFlag]; ok {
			t.Fatalf("key flag should not be marked required when env fallback exists")
		}
		if _, ok := cmd.Flag("admin-token").Annotations[cobra.BashCompOneRequiredFlag]; ok {
			t.Fatalf("admin-token flag should not be marked required when env fallback exists")
		}
	})
}

func TestE2ESingleCmdDoesNotMarkEnvBackedFlagsAsRequired(t *testing.T) {
	withPathsForTest(t, func() {
		cmd := newE2ESingleCmd()
		if _, ok := cmd.Flag("key").Annotations[cobra.BashCompOneRequiredFlag]; ok {
			t.Fatalf("key flag should not be marked required when env fallback exists")
		}
		if _, ok := cmd.Flag("admin-token").Annotations[cobra.BashCompOneRequiredFlag]; ok {
			t.Fatalf("admin-token flag should not be marked required when env fallback exists")
		}
	})
}

func TestResolveSingleE2EPathsDefaultsOutputDirFromArtifactsID(t *testing.T) {
	withPathsForTest(t, func() {
		paths.WorkspaceRoot = "/tmp/workspace-root"
		artifactsID, outputDir := resolveSingleE2EPaths("single-listing-regression-fixed", "")
		if artifactsID != "single-listing-regression-fixed" {
			t.Fatalf("artifacts id = %q", artifactsID)
		}
		want := filepath.Join(paths.WorkspaceRoot, "syl-listing-pro-x", "out", artifactsID)
		if outputDir != want {
			t.Fatalf("output dir = %q, want %q", outputDir, want)
		}
	})
}

func TestResolveSingleE2EPathsGeneratesArtifactsIDWhenMissing(t *testing.T) {
	withPathsForTest(t, func() {
		paths.WorkspaceRoot = "/tmp/workspace-root"
		artifactsID, outputDir := resolveSingleE2EPaths("", "")
		if !strings.HasPrefix(artifactsID, "single-listing-regression-") {
			t.Fatalf("artifacts id = %q", artifactsID)
		}
		wantPrefix := filepath.Join(paths.WorkspaceRoot, "syl-listing-pro-x", "out", "single-listing-regression-")
		if !strings.HasPrefix(outputDir, wantPrefix) {
			t.Fatalf("output dir = %q, want prefix %q", outputDir, wantPrefix)
		}
	})
}

func TestResolveSingleE2EPrivateKeyUsesRepoDefaultAndEnablesDevMode(t *testing.T) {
	withPathsForTest(t, func() {
		paths.RulesRepo = "/tmp/rules-repo"
		t.Setenv("SYL_LISTING_ALLOW_DEV_PRIVATE_KEY", "")
		privateKey, restore := resolveSingleE2EPrivateKey("")
		defer restore()

		want := filepath.Join(paths.RulesRepo, "keys", "rules_private.pem")
		if privateKey != want {
			t.Fatalf("private key = %q, want %q", privateKey, want)
		}
		if got := os.Getenv("SYL_LISTING_ALLOW_DEV_PRIVATE_KEY"); got != "1" {
			t.Fatalf("allow dev env = %q, want 1", got)
		}
	})
}

func TestResolveSingleE2EPrivateKeyKeepsExplicitValueUntouched(t *testing.T) {
	withPathsForTest(t, func() {
		t.Setenv("SYL_LISTING_ALLOW_DEV_PRIVATE_KEY", "")
		privateKey, restore := resolveSingleE2EPrivateKey("/abs/custom.pem")
		defer restore()

		if privateKey != "/abs/custom.pem" {
			t.Fatalf("private key = %q", privateKey)
		}
		if got := os.Getenv("SYL_LISTING_ALLOW_DEV_PRIVATE_KEY"); got != "" {
			t.Fatalf("allow dev env = %q, want empty", got)
		}
	})
}

func TestE2ERunCmdUnknownCaseWinsBeforeInputValidation(t *testing.T) {
	withPathsForTest(t, func() {
		cmd := newE2ERunCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{
			"--case", "mystery-gate",
			"--tenant", "syl",
			"--out", t.TempDir(),
			"--key", "key",
			"--admin-token", "admin",
		})

		err := cmd.Execute()
		if err == nil {
			t.Fatal("expected error")
		}
		if got := err.Error(); got == "" || got == "缺少输入文件：mystery-gate 需要传 --input" {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := err.Error(); got != "未知 e2e 用例: mystery-gate" {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
