package worker

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestCreateWorkerArchiveSkipsRuntimeOnlyFiles(t *testing.T) {
	root := t.TempDir()
	workerRepo := filepath.Join(root, "worker")
	for _, dir := range []string{
		filepath.Join(workerRepo, "docker"),
		filepath.Join(workerRepo, "src"),
		filepath.Join(workerRepo, "data", "runtime"),
		filepath.Join(workerRepo, "node_modules", "pkg"),
		filepath.Join(workerRepo, "dist"),
		filepath.Join(workerRepo, ".git"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for path, body := range map[string]string{
		filepath.Join(workerRepo, "worker.config.json"):              `{"server":{"domain":"worker.aelus.tech","letsencrypt_email":"ops@example.com"}}`,
		filepath.Join(workerRepo, ".env"):                            "ADMIN_TOKEN=secret\n",
		filepath.Join(workerRepo, "docker", "compose.yml"):           "services:\n",
		filepath.Join(workerRepo, "src", "index.js"):                 "console.log('ok')\n",
		filepath.Join(workerRepo, "data", "runtime", "version.json"): "{}\n",
		filepath.Join(workerRepo, "node_modules", "pkg", "index.js"): "module.exports = {}\n",
		filepath.Join(workerRepo, "dist", "app.js"):                  "compiled\n",
		filepath.Join(workerRepo, ".git", "HEAD"):                    "ref: refs/heads/main\n",
	} {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	svc := Service{WorkerRepo: workerRepo}
	archivePath, err := svc.createWorkerArchive()
	if err != nil {
		t.Fatalf("createWorkerArchive() error = %v", err)
	}
	defer os.Remove(archivePath)

	got := readArchiveEntries(t, archivePath)
	if _, ok := got[".env"]; ok {
		t.Fatalf("archive should not include .env: %+v", got)
	}
	for _, unwanted := range []string{
		"data/",
		"data/runtime/",
		"data/runtime/version.json",
		"node_modules/",
		"node_modules/pkg/index.js",
		"dist/",
		"dist/app.js",
		".git/",
		".git/HEAD",
	} {
		if _, ok := got[unwanted]; ok {
			t.Fatalf("archive should not include %s: %+v", unwanted, got)
		}
	}
	for _, wanted := range []string{
		".compose.env",
		"worker.config.json",
		"docker/",
		"docker/compose.yml",
		"src/",
		"src/index.js",
	} {
		if _, ok := got[wanted]; !ok {
			t.Fatalf("archive missing %s: %+v", wanted, got)
		}
	}
	if content := string(got[".compose.env"]); !strings.Contains(content, "DOMAIN=worker.aelus.tech") || !strings.Contains(content, "LETSENCRYPT_EMAIL=ops@example.com") {
		t.Fatalf(".compose.env content = %q", content)
	}
}

func TestCreateWorkerArchiveClosesFilesDuringWalk(t *testing.T) {
	var original syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &original); err != nil {
		t.Fatalf("Getrlimit() error = %v", err)
	}
	limit := original
	if limit.Cur > 96 {
		limit.Cur = 96
	}
	if limit.Max > 96 {
		limit.Max = 96
	}
	if limit.Cur == 0 || limit.Max == 0 {
		t.Skip("RLIMIT_NOFILE unavailable")
	}
	if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &limit); err != nil {
		t.Skipf("Setrlimit() error = %v", err)
	}
	t.Cleanup(func() {
		_ = syscall.Setrlimit(syscall.RLIMIT_NOFILE, &original)
	})

	root := t.TempDir()
	workerRepo := filepath.Join(root, "worker")
	for _, dir := range []string{
		filepath.Join(workerRepo, "docker"),
		filepath.Join(workerRepo, "src"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(workerRepo, "worker.config.json"), []byte(`{"server":{"domain":"worker.aelus.tech"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 180; i++ {
		path := filepath.Join(workerRepo, "src", fmt.Sprintf("file-%03d.txt", i))
		if err := os.WriteFile(path, []byte("hello\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	svc := Service{WorkerRepo: workerRepo}
	archivePath, err := svc.createWorkerArchive()
	if err != nil {
		t.Fatalf("createWorkerArchive() error = %v", err)
	}
	defer os.Remove(archivePath)
}

func TestCreateWorkerArchiveUsesServerSpecificComposeEnv(t *testing.T) {
	root := t.TempDir()
	workerRepo := filepath.Join(root, "worker")
	for _, dir := range []string{
		filepath.Join(workerRepo, "docker"),
		filepath.Join(workerRepo, "src"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for path, body := range map[string]string{
		filepath.Join(workerRepo, "worker.config.json"):    `{"server":{"domain":"worker.aelus.tech","letsencrypt_email":"ops@example.com"}}`,
		filepath.Join(workerRepo, "docker", "compose.yml"): "services:\n",
		filepath.Join(workerRepo, "src", "index.js"):       "console.log('ok')\n",
	} {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	svc := Service{WorkerRepo: workerRepo}
	archivePath, err := svc.createWorkerArchiveForServer(Server{
		Name:             "syl-test-server",
		Domain:           "worker.test.example",
		LetsencryptEmail: "qa@example.com",
	})
	if err != nil {
		t.Fatalf("createWorkerArchiveForServer() error = %v", err)
	}
	defer os.Remove(archivePath)

	got := readArchiveEntries(t, archivePath)
	content := string(got[".compose.env"])
	for _, want := range []string{
		"DOMAIN=worker.test.example",
		"LETSENCRYPT_EMAIL=qa@example.com",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf(".compose.env content = %q, want contains %q", content, want)
		}
	}
}

func readArchiveEntries(t *testing.T, archivePath string) map[string][]byte {
	t.Helper()

	f, err := os.Open(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	entries := make(map[string][]byte)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			return entries
		}
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			t.Fatal(err)
		}
		entries[header.Name] = data
	}
}
