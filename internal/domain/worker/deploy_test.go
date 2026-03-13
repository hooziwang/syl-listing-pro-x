package worker

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeRemote struct {
	copies  []copyCall
	runs    []runCall
	streams []runCall
}

type copyCall struct {
	server Server
	src    string
	dst    string
}

type runCall struct {
	server Server
	cmd    string
}

func (f *fakeRemote) Copy(_ context.Context, server Server, src, dst string) error {
	f.copies = append(f.copies, copyCall{server: server, src: src, dst: dst})
	return nil
}

func (f *fakeRemote) Run(_ context.Context, server Server, cmd string) error {
	f.runs = append(f.runs, runCall{server: server, cmd: cmd})
	return nil
}

func (f *fakeRemote) Stream(_ context.Context, server Server, cmd string) error {
	f.streams = append(f.streams, runCall{server: server, cmd: cmd})
	return nil
}

func TestPushEnv(t *testing.T) {
	root := t.TempDir()
	workerRepo := filepath.Join(root, "worker")
	if err := os.MkdirAll(workerRepo, 0o755); err != nil {
		t.Fatal(err)
	}
	envPath := filepath.Join(workerRepo, ".env")
	if err := os.WriteFile(envPath, []byte("ADMIN_TOKEN=x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	remote := &fakeRemote{}
	svc := Service{
		WorkerRepo: workerRepo,
		Remote:     remote,
		Servers:    DefaultServers(),
	}
	if err := svc.PushEnv(context.Background(), PushEnvInput{Server: "syl-server"}); err != nil {
		t.Fatalf("PushEnv() error = %v", err)
	}
	if len(remote.copies) != 1 {
		t.Fatalf("copies=%d", len(remote.copies))
	}
	if remote.copies[0].src != envPath {
		t.Fatalf("copy src=%q", remote.copies[0].src)
	}
	if remote.copies[0].dst != "/tmp/syl-listing-worker.env.tmp" {
		t.Fatalf("copy dst=%q", remote.copies[0].dst)
	}
	if len(remote.runs) != 1 {
		t.Fatalf("runs=%d", len(remote.runs))
	}
	if !strings.Contains(remote.runs[0].cmd, "docker compose --env-file .compose.env up -d --no-deps --force-recreate worker-api worker-runner") {
		t.Fatalf("unexpected run cmd: %s", remote.runs[0].cmd)
	}
}

func TestDeploy(t *testing.T) {
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
	runGit(t, workerRepo, "add", "worker.config.json", ".env")
	runGit(t, workerRepo, "commit", "-m", "init")
	runGit(t, workerRepo, "tag", "--no-sign", "v0.1.2")
	remote := &fakeRemote{}
	svc := Service{
		WorkerRepo: workerRepo,
		Remote:     remote,
		Servers:    DefaultServers(),
	}
	if err := svc.Deploy(context.Background(), DeployInput{
		Server:             "syl-server",
		SkipBuild:          true,
		StopLegacy:         true,
		InstallDocker:      true,
		SkipWaitHTTPS:      true,
		HTTPSTimeout:       240,
		HTTPSCheckInterval: 2,
		SkipDiagnose:       true,
	}); err != nil {
		t.Fatalf("Deploy() error = %v", err)
	}
	if len(remote.copies) == 0 {
		t.Fatal("expected upload")
	}
	if !strings.HasPrefix(remote.copies[0].dst, "/tmp/syl-listing-worker-") {
		t.Fatalf("archive dst=%q", remote.copies[0].dst)
	}
	if len(remote.runs) != 3 {
		t.Fatalf("runs=%d", len(remote.runs))
	}
	if !strings.Contains(remote.runs[0].cmd, "find /home/ubuntu/syl-listing-worker -mindepth 1 -maxdepth 1 ! -name data ! -name .env -exec rm -rf {} +") {
		t.Fatalf("unexpected sync cleanup cmd: %s", remote.runs[0].cmd)
	}
	if !strings.Contains(remote.runs[1].cmd, "cp /tmp/syl-listing-worker.env.tmp /home/ubuntu/syl-listing-worker/.env") {
		t.Fatalf("unexpected env upload cmd: %s", remote.runs[1].cmd)
	}
	for _, want := range []string{
		"sudo apt-get install -y docker.io docker-compose-v2 python3",
		"sudo systemctl stop syl-listing-worker-api.service syl-listing-worker-runner.service nginx || true",
		"sudo -n docker compose --env-file .compose.env up -d",
		"sudo -n docker compose --env-file .compose.env ps",
	} {
		if !strings.Contains(remote.runs[2].cmd, want) {
			t.Fatalf("deploy cmd missing %q: %s", want, remote.runs[2].cmd)
		}
	}
	versionJSONPrefix := "printf %s '"
	start := strings.Index(remote.runs[2].cmd, versionJSONPrefix)
	if start < 0 {
		t.Fatalf("deploy cmd missing version json payload: %s", remote.runs[2].cmd)
	}
	start += len(versionJSONPrefix)
	end := strings.Index(remote.runs[2].cmd[start:], `' > data/runtime/version.json`)
	if end < 0 {
		t.Fatalf("deploy cmd missing version json redirect: %s", remote.runs[2].cmd)
	}
	raw := remote.runs[2].cmd[start : start+end]
	var payload map[string]string
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("version payload json invalid: %v", err)
	}
	if payload["worker_version"] != "v0.1.2" {
		t.Fatalf("worker_version=%q", payload["worker_version"])
	}
}

func TestDeployWaitHTTPSUsesHostResolvedDomain(t *testing.T) {
	root := t.TempDir()
	workerRepo := filepath.Join(root, "worker")
	if err := os.MkdirAll(workerRepo, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workerRepo, "worker.config.json"), []byte(`{"server":{"domain":"worker.aelus.tech"}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := Service{WorkerRepo: workerRepo}
	cmd := svc.buildRemoteDeployCommand(DefaultServers()["syl-server"], "/tmp/archive.tar.gz", DeployInput{
		Server:             "syl-server",
		SkipBuild:          true,
		SkipWaitHTTPS:      false,
		HTTPSTimeout:       120,
		HTTPSCheckInterval: 3,
	}, `{"service":"syl-listing-worker","git_commit":"abc1234","build_time":"2026-03-11T04:00:00Z","deployed_at":"2026-03-11T04:05:00Z"}`)
	if strings.Contains(cmd, "python3 -c") {
		t.Fatalf("deploy cmd should not invoke python3 inside container: %s", cmd)
	}
	if !strings.Contains(cmd, "/etc/letsencrypt/live/worker.aelus.tech/fullchain.pem") {
		t.Fatalf("deploy cmd missing inline domain: %s", cmd)
	}
	if !strings.Contains(cmd, "mkdir -p data/runtime") {
		t.Fatalf("deploy cmd missing runtime dir creation: %s", cmd)
	}
	if !strings.Contains(cmd, "data/runtime/version.json") {
		t.Fatalf("deploy cmd missing version.json write: %s", cmd)
	}
}

func TestBuildRuntimeVersionMetadataRequiresExactTag(t *testing.T) {
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

	svc := Service{WorkerRepo: workerRepo}
	_, err := svc.buildRuntimeVersionMetadata()
	if err == nil {
		t.Fatal("buildRuntimeVersionMetadata() expected error without tag")
	}
	if !strings.Contains(err.Error(), "git tag") {
		t.Fatalf("error=%q", err)
	}
}

func TestDiagnoseRunsFullChecklist(t *testing.T) {
	root := t.TempDir()
	workerRepo := filepath.Join(root, "worker")
	if err := os.MkdirAll(workerRepo, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workerRepo, "worker.config.json"), []byte(`{"server":{"domain":"worker.aelus.tech"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	remote := &fakeRemote{}
	svc := Service{
		WorkerRepo: workerRepo,
		Remote:     remote,
		Servers:    DefaultServers(),
	}
	if err := svc.Diagnose(context.Background(), "syl-server"); err != nil {
		t.Fatalf("Diagnose() error = %v", err)
	}
	if len(remote.runs) != 1 {
		t.Fatalf("runs=%d", len(remote.runs))
	}
	cmd := remote.runs[0].cmd
	for _, want := range []string{
		"sudo -n docker compose --env-file .compose.env ps --services --status running",
		"fetch(\"http://127.0.0.1:8080/healthz\")",
		"sudo -n docker compose --env-file .compose.env exec -T redis redis-cli ping",
		"sudo -n docker compose --env-file .compose.env exec -T nginx nginx -t",
		"exec -T -e SYL_KEY=\"$SYL_KEY\" worker-api",
		"/etc/letsencrypt/live/worker.aelus.tech/",
	} {
		if !strings.Contains(cmd, want) {
			t.Fatalf("diagnose cmd missing %q: %s", want, cmd)
		}
	}
	if strings.Contains(cmd, "python3 -c") {
		t.Fatalf("diagnose cmd should not invoke python3: %s", cmd)
	}
}

func TestDiagnoseCommandRequiresDeepseekHealthCheck(t *testing.T) {
	root := t.TempDir()
	workerRepo := filepath.Join(root, "worker")
	if err := os.MkdirAll(workerRepo, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workerRepo, "worker.config.json"), []byte(`{"server":{"domain":"worker.aelus.tech"}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := Service{WorkerRepo: workerRepo}
	cmd := svc.buildRemoteDiagnoseCommand(DefaultServers()["syl-server"])
	if strings.Contains(cmd, "flux"+"code") {
		t.Fatalf("diagnose cmd should not reference removed provider checks: %s", cmd)
	}
	if !strings.Contains(cmd, "data.llm?.deepseek?.ok!==true") {
		t.Fatalf("diagnose cmd should require deepseek health only: %s", cmd)
	}
}

func TestLogsUsesComposeFallback(t *testing.T) {
	remote := &fakeRemote{}
	svc := Service{
		Remote:  remote,
		Servers: DefaultServers(),
	}
	if err := svc.Logs(context.Background(), LogsInput{Server: "syl-server"}); err != nil {
		t.Fatalf("Logs() error = %v", err)
	}
	if len(remote.runs) != 0 {
		t.Fatalf("logs should not use buffered Run, runs=%d", len(remote.runs))
	}
	if len(remote.streams) != 1 {
		t.Fatalf("streams=%d", len(remote.streams))
	}
	cmd := remote.streams[0].cmd
	if !strings.Contains(cmd, "sudo -n docker compose --env-file .compose.env logs -f --tail=200") {
		t.Fatalf("logs cmd missing sudo fallback: %s", cmd)
	}
}

func TestLogsSupportsFilters(t *testing.T) {
	remote := &fakeRemote{}
	svc := Service{
		Remote:  remote,
		Servers: DefaultServers(),
	}
	if err := svc.Logs(context.Background(), LogsInput{
		Server:   "syl-server",
		Services: []string{"worker-api"},
		Tail:     20,
		Since:    "10m",
	}); err != nil {
		t.Fatalf("Logs() error = %v", err)
	}
	if len(remote.streams) != 1 {
		t.Fatalf("streams=%d", len(remote.streams))
	}
	cmd := remote.streams[0].cmd
	for _, want := range []string{
		"logs -f --tail=20",
		"--since '10m'",
		"'worker-api'",
	} {
		if !strings.Contains(cmd, want) {
			t.Fatalf("logs cmd missing %q: %s", want, cmd)
		}
	}
}

func TestLogsSupportsNoFollowAndMultipleServices(t *testing.T) {
	remote := &fakeRemote{}
	svc := Service{
		Remote:  remote,
		Servers: DefaultServers(),
	}
	if err := svc.Logs(context.Background(), LogsInput{
		Server:   "syl-server",
		Services: []string{"worker-api", "nginx"},
		Tail:     15,
		NoFollow: true,
	}); err != nil {
		t.Fatalf("Logs() error = %v", err)
	}
	if len(remote.streams) != 1 {
		t.Fatalf("streams=%d", len(remote.streams))
	}
	cmd := remote.streams[0].cmd
	if strings.Contains(cmd, " logs -f ") {
		t.Fatalf("logs cmd should not follow: %s", cmd)
	}
	for _, want := range []string{
		"logs --tail=15",
		"'worker-api' 'nginx'",
	} {
		if !strings.Contains(cmd, want) {
			t.Fatalf("logs cmd missing %q: %s", want, cmd)
		}
	}
}

func TestComposeFallbackSuppressesFirstAttemptStderr(t *testing.T) {
	cmd := composeFallbackCommand("ps")
	if !strings.Contains(cmd, "docker compose --env-file .compose.env ps 2>/dev/null") {
		t.Fatalf("first compose attempt should suppress stderr: %s", cmd)
	}
}
