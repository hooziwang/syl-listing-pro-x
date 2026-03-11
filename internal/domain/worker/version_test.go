package worker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCheckRemoteVersion(t *testing.T) {
	root := t.TempDir()
	workerRepo := filepath.Join(root, "worker")
	if err := os.MkdirAll(workerRepo, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, workerRepo, "init")
	runGit(t, workerRepo, "config", "user.email", "test@example.com")
	runGit(t, workerRepo, "config", "user.name", "tester")
	if err := os.WriteFile(filepath.Join(workerRepo, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, workerRepo, "add", "README.md")
	runGit(t, workerRepo, "commit", "-m", "init")
	localCommit := runGit(t, workerRepo, "rev-parse", "--short", "HEAD")

	var gotAuth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"tenant_id":"admin","service":"syl-listing-worker","git_commit":"` + localCommit + `","build_time":"2026-03-11T04:00:00Z","deployed_at":"2026-03-11T04:05:00Z","rules_versions":{"demo":"rules-demo-1","syl":"rules-syl-1"}}`))
	}))
	defer ts.Close()

	svc := Service{
		HTTPClient: ts.Client(),
		WorkerRepo: workerRepo,
	}

	result, err := svc.CheckRemoteVersion(context.Background(), CheckRemoteVersionInput{
		BaseURL:    ts.URL,
		AdminToken: "admin-secret",
	})
	if err != nil {
		t.Fatalf("CheckRemoteVersion() error = %v", err)
	}
	if gotAuth != "Bearer admin-secret" {
		t.Fatalf("authorization=%q", gotAuth)
	}
	if !result.UpToDate {
		t.Fatalf("UpToDate=false")
	}
	if result.LocalGitCommit != localCommit {
		t.Fatalf("LocalGitCommit=%q want %q", result.LocalGitCommit, localCommit)
	}
	if result.Remote.GitCommit != localCommit {
		t.Fatalf("Remote.GitCommit=%q want %q", result.Remote.GitCommit, localCommit)
	}
	if result.Remote.RulesVersions["syl"] != "rules-syl-1" {
		t.Fatalf("RulesVersions[syl]=%q", result.Remote.RulesVersions["syl"])
	}
}

func TestCheckRemoteVersion_Mismatch(t *testing.T) {
	root := t.TempDir()
	workerRepo := filepath.Join(root, "worker")
	if err := os.MkdirAll(workerRepo, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, workerRepo, "init")
	runGit(t, workerRepo, "config", "user.email", "test@example.com")
	runGit(t, workerRepo, "config", "user.name", "tester")
	if err := os.WriteFile(filepath.Join(workerRepo, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, workerRepo, "add", "README.md")
	runGit(t, workerRepo, "commit", "-m", "init")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"tenant_id":"admin","service":"syl-listing-worker","git_commit":"deadbee","build_time":"2026-03-11T04:00:00Z","deployed_at":"2026-03-11T04:05:00Z","rules_versions":{"demo":"rules-demo-1","syl":"rules-syl-1"}}`))
	}))
	defer ts.Close()

	svc := Service{
		HTTPClient: ts.Client(),
		WorkerRepo: workerRepo,
	}

	result, err := svc.CheckRemoteVersion(context.Background(), CheckRemoteVersionInput{
		BaseURL:    ts.URL,
		AdminToken: "admin-secret",
	})
	if err == nil {
		t.Fatal("expected mismatch error")
	}
	if result.UpToDate {
		t.Fatalf("UpToDate=true")
	}
}

func TestCheckRemoteVersion_LoadsAdminTokenFromHomeEnv(t *testing.T) {
	root := t.TempDir()
	workerRepo := filepath.Join(root, "worker")
	if err := os.MkdirAll(workerRepo, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, workerRepo, "init")
	runGit(t, workerRepo, "config", "user.email", "test@example.com")
	runGit(t, workerRepo, "config", "user.name", "tester")
	if err := os.WriteFile(filepath.Join(workerRepo, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, workerRepo, "add", "README.md")
	runGit(t, workerRepo, "commit", "-m", "init")
	localCommit := runGit(t, workerRepo, "rev-parse", "--short", "HEAD")

	homeDir := filepath.Join(root, "home")
	if err := os.MkdirAll(filepath.Join(homeDir, ".syl-listing-pro-x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(homeDir, ".syl-listing-pro-x", ".env"), []byte("ADMIN_TOKEN=from-home-env\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	oldHome := os.Getenv("HOME")
	if err := os.Setenv("HOME", homeDir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if oldHome == "" {
			_ = os.Unsetenv("HOME")
			return
		}
		_ = os.Setenv("HOME", oldHome)
	}()

	var gotAuth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"tenant_id":"admin","service":"syl-listing-worker","git_commit":"` + localCommit + `","build_time":"2026-03-11T04:00:00Z","deployed_at":"2026-03-11T04:05:00Z","rules_versions":{"syl":"rules-syl-1"}}`))
	}))
	defer ts.Close()

	svc := Service{
		HTTPClient: ts.Client(),
		WorkerRepo: workerRepo,
	}

	_, err := svc.CheckRemoteVersion(context.Background(), CheckRemoteVersionInput{
		BaseURL: ts.URL,
	})
	if err != nil {
		t.Fatalf("CheckRemoteVersion() error = %v", err)
	}
	if gotAuth != "Bearer from-home-env" {
		t.Fatalf("authorization=%q", gotAuth)
	}
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v, output=%s", args, err, string(out))
	}
	return string(bytesTrimSpace(out))
}

func bytesTrimSpace(in []byte) []byte {
	for len(in) > 0 && (in[len(in)-1] == '\n' || in[len(in)-1] == '\r' || in[len(in)-1] == ' ' || in[len(in)-1] == '\t') {
		in = in[:len(in)-1]
	}
	for len(in) > 0 && (in[0] == '\n' || in[0] == '\r' || in[0] == ' ' || in[0] == '\t') {
		in = in[1:]
	}
	return in
}
