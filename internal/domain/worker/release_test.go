package worker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseRejectsDirtyWorkerRepo(t *testing.T) {
	root := t.TempDir()
	workerRepo := filepath.Join(root, "worker")
	if err := os.MkdirAll(workerRepo, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, workerRepo, "init")
	runGit(t, workerRepo, "config", "user.email", "test@example.com")
	runGit(t, workerRepo, "config", "user.name", "tester")
	if err := os.WriteFile(filepath.Join(workerRepo, "worker.config.json"), []byte(`{"server":{"domain":"worker.aelus.tech"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workerRepo, ".env"), []byte("ADMIN_TOKEN=x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, workerRepo, "add", "worker.config.json", ".env")
	runGit(t, workerRepo, "commit", "-m", "init")
	if err := os.WriteFile(filepath.Join(workerRepo, "README.md"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := Service{
		WorkerRepo: workerRepo,
		Servers:    DefaultServers(),
	}
	err := svc.Release(context.Background(), ReleaseInput{
		Server:     "syl-server",
		Version:    "v0.1.3",
		AdminToken: "admin-secret",
	})
	if err == nil {
		t.Fatal("Release() expected dirty repo error")
	}
	if !strings.Contains(err.Error(), "工作区不干净") {
		t.Fatalf("error=%q", err)
	}
}

func TestReleaseRunsTestsTagsDeploysAndChecksRemoteVersion(t *testing.T) {
	root := t.TempDir()
	workerRepo := filepath.Join(root, "worker")
	if err := os.MkdirAll(filepath.Join(workerRepo, "docker"), 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, workerRepo, "init")
	runGit(t, workerRepo, "config", "user.email", "test@example.com")
	runGit(t, workerRepo, "config", "user.name", "tester")
	if err := os.WriteFile(filepath.Join(workerRepo, "worker.config.json"), []byte(`{"server":{"domain":"worker.aelus.tech"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workerRepo, ".env"), []byte("ADMIN_TOKEN=x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workerRepo, "package.json"), []byte(`{"name":"worker","scripts":{"test":"echo ok"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, workerRepo, "add", "worker.config.json", ".env", "package.json")
	runGit(t, workerRepo, "commit", "-m", "init")
	localCommit := runGit(t, workerRepo, "rev-parse", "--short", "HEAD")
	remoteRepo := filepath.Join(root, "worker-remote.git")
	runGit(t, root, "init", "--bare", remoteRepo)
	runGit(t, workerRepo, "remote", "add", "origin", remoteRepo)

	binDir := filepath.Join(root, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	npmLog := filepath.Join(root, "npm.log")
	npmPath := filepath.Join(binDir, "npm")
	npmScript := "#!/bin/sh\n" +
		"echo \"$@\" > " + shellSingleQuote(npmLog) + "\n" +
		"exit 0\n"
	if err := os.WriteFile(npmPath, []byte(npmScript), 0o755); err != nil {
		t.Fatal(err)
	}
	oldPath := os.Getenv("PATH")
	if err := os.Setenv("PATH", binDir+string(os.PathListSeparator)+oldPath); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Setenv("PATH", oldPath)
	}()

	remote := &fakeRemote{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"tenant_id":"admin","service":"syl-listing-worker","worker_version":"v0.1.3","git_commit":"` + localCommit + `","build_time":"2026-03-15T15:00:00Z","deployed_at":"2026-03-15T15:01:00Z","rules_versions":{"syl":"rules-syl-1"}}`))
	}))
	defer ts.Close()

	svc := Service{
		HTTPClient: ts.Client(),
		WorkerRepo: workerRepo,
		Remote:     remote,
		Servers: map[string]Server{
			"syl-server": {
				Name:   "syl-server",
				Host:   "127.0.0.1",
				User:   "ubuntu",
				Port:   22,
				Dir:    "/home/ubuntu/syl-listing-worker",
				Domain: strings.TrimPrefix(ts.URL, "http://"),
			},
		},
	}

	if err := svc.Release(context.Background(), ReleaseInput{
		Server:             "syl-server",
		Version:            "v0.1.3",
		AdminToken:         "admin-secret",
		BaseURL:            ts.URL,
		SkipBuild:          true,
		SkipDiagnose:       true,
		SkipWaitHTTPS:      true,
		HTTPSTimeout:       5,
		HTTPSCheckInterval: 1,
	}); err != nil {
		t.Fatalf("Release() error = %v", err)
	}

	if got := runGit(t, workerRepo, "describe", "--tags", "--exact-match", "HEAD"); got != "v0.1.3" {
		t.Fatalf("exact tag = %q", got)
	}
	if got := string(bytesTrimSpace(mustReadFile(t, npmLog))); got != "test" {
		t.Fatalf("npm args = %q", got)
	}
	if len(remote.copies) == 0 {
		t.Fatal("expected deploy archive upload")
	}
	if len(remote.runs) == 0 {
		t.Fatal("expected remote deploy commands")
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	return data
}
