package cmd

import "testing"

func withPathsForTest(t *testing.T, fn func()) {
	t.Helper()
	oldPaths := paths
	t.Cleanup(func() {
		paths = oldPaths
	})
	fn()
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
