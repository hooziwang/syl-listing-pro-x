package e2e

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeRulesRunner struct {
	calls []rulesCall
}

type rulesCall struct {
	tenant         string
	workerURL      string
	privateKeyPath string
}

func (f *fakeRulesRunner) Publish(ctx context.Context, in PublishRulesInput) error {
	f.calls = append(f.calls, rulesCall{
		tenant:         in.Tenant,
		workerURL:      in.WorkerURL,
		privateKeyPath: in.PrivateKeyPath,
	})
	return nil
}

type fakeWorkerRunner struct {
	called bool
}

func (f *fakeWorkerRunner) DiagnoseExternal(ctx context.Context, in DiagnoseWorkerInput) error {
	f.called = true
	return nil
}

func TestListCases(t *testing.T) {
	svc := Service{}
	cases := svc.ListCases()
	if len(cases) != 2 {
		t.Fatal("expected cases")
	}
	if cases[0] != "release-gate" {
		t.Fatalf("first case=%q", cases[0])
	}
	if cases[1] != "architecture-gate" {
		t.Fatalf("second case=%q", cases[1])
	}
}

func TestRunReleaseGate(t *testing.T) {
	root := t.TempDir()
	inputDir := filepath.Join(root, "input")
	outDir := filepath.Join(root, "out")
	artifactsDir := filepath.Join(root, "artifacts")
	if err := os.MkdirAll(inputDir, 0o755); err != nil {
		t.Fatal(err)
	}
	inputPath := filepath.Join(inputDir, "demo.md")
	if err := os.WriteFile(inputPath, []byte("demo"), 0o644); err != nil {
		t.Fatal(err)
	}

	cliPath := filepath.Join(root, "syl-listing-pro")
	script := "#!/bin/sh\n" +
		"out=\"\"\n" +
		"while [ \"$#\" -gt 0 ]; do\n" +
		"  if [ \"$1\" = \"-o\" ] || [ \"$1\" = \"--out\" ]; then out=\"$2\"; shift 2; continue; fi\n" +
		"  shift\n" +
		"done\n" +
		"mkdir -p \"$out\"\n" +
		"printf '# EN\\n\\n## 搜索词\\npaper lanterns classroom decor\\n' > \"$out/demo_abcd_en.md\"\n" +
		"printf '# CN\\n\\n## 搜索词\\n中文搜索词\\n' > \"$out/demo_abcd_cn.md\"\n" +
		"printf 'docx' > \"$out/demo_abcd_en.docx\"\n" +
		"printf 'docx' > \"$out/demo_abcd_cn.docx\"\n" +
		"echo '任务完成：成功 1，失败 0，总耗时 1s'\n"
	if err := os.WriteFile(cliPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	rulesRunner := &fakeRulesRunner{}
	workerRunner := &fakeWorkerRunner{}
	svc := Service{
		CLIPath:       cliPath,
		ArtifactsRoot: artifactsDir,
		RulesRunner:   rulesRunner,
		WorkerRunner:  workerRunner,
	}
	result, err := svc.Run(context.Background(), RunInput{
		CaseName:    "release-gate",
		Tenant:      "syl",
		WorkerURL:   "https://worker.aelus.tech",
		SYLKey:      "key",
		AdminToken:  "admin",
		InputPath:   inputPath,
		OutputDir:   outDir,
		ArtifactsID: "run-1",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !workerRunner.called {
		t.Fatal("expected worker diagnose call")
	}
	if len(rulesRunner.calls) != 1 || rulesRunner.calls[0].tenant != "syl" {
		t.Fatalf("unexpected rules calls: %+v", rulesRunner.calls)
	}
	if len(result.OutputFiles) != 4 {
		t.Fatalf("output files=%d", len(result.OutputFiles))
	}
	if _, err := os.Stat(filepath.Join(result.ArtifactsDir, "cli.stdout.log")); err != nil {
		t.Fatalf("stdout artifact missing: %v", err)
	}
}

func TestRunArchitectureGate(t *testing.T) {
	root := t.TempDir()
	inputDir := filepath.Join(root, "input")
	outDir := filepath.Join(root, "out")
	artifactsDir := filepath.Join(root, "artifacts")
	if err := os.MkdirAll(inputDir, 0o755); err != nil {
		t.Fatal(err)
	}
	inputPath := filepath.Join(inputDir, "demo.md")
	if err := os.WriteFile(inputPath, []byte("demo"), 0o644); err != nil {
		t.Fatal(err)
	}

	cliPath := filepath.Join(root, "syl-listing-pro")
	script := "#!/bin/sh\n" +
		"out=\"\"\n" +
		"logfile=\"\"\n" +
		"while [ \"$#\" -gt 0 ]; do\n" +
		"  if [ \"$1\" = \"-o\" ] || [ \"$1\" = \"--out\" ]; then out=\"$2\"; shift 2; continue; fi\n" +
		"  if [ \"$1\" = \"--log-file\" ]; then logfile=\"$2\"; shift 2; continue; fi\n" +
		"  shift\n" +
		"done\n" +
		"mkdir -p \"$out\"\n" +
		"mkdir -p \"$(dirname \"$logfile\")\"\n" +
		"printf '{\"event\":\"cli_start\"}\\n' > \"$logfile\"\n" +
		"printf '# EN\\n\\n## 搜索词\\npaper lanterns classroom decor\\n' > \"$out/demo_abcd_en.md\"\n" +
		"printf '# CN\\n\\n## 搜索词\\n中文搜索词\\n' > \"$out/demo_abcd_cn.md\"\n" +
		"printf 'docx' > \"$out/demo_abcd_en.docx\"\n" +
		"printf 'docx' > \"$out/demo_abcd_cn.docx\"\n" +
		"echo '任务完成：成功 1，失败 0，总耗时 1s'\n"
	if err := os.WriteFile(cliPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	rulesRunner := &fakeRulesRunner{}
	workerRunner := &fakeWorkerRunner{}
	svc := Service{
		CLIPath:       cliPath,
		ArtifactsRoot: artifactsDir,
		RulesRunner:   rulesRunner,
		WorkerRunner:  workerRunner,
	}
	result, err := svc.Run(context.Background(), RunInput{
		CaseName:       "architecture-gate",
		Tenant:         "syl",
		WorkerURL:      "https://worker.example.test",
		SYLKey:         "key",
		AdminToken:     "admin",
		PrivateKeyPath: "/tmp/custom-rules.pem",
		InputPath:      inputPath,
		OutputDir:      outDir,
		ArtifactsID:    "arch-run-1",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !workerRunner.called {
		t.Fatal("expected worker diagnose call")
	}
	if len(rulesRunner.calls) != 1 {
		t.Fatalf("unexpected rules calls: %+v", rulesRunner.calls)
	}
	if got := rulesRunner.calls[0].workerURL; got != "https://worker.example.test" {
		t.Fatalf("workerURL=%q", got)
	}
	if got := rulesRunner.calls[0].privateKeyPath; got != "/tmp/custom-rules.pem" {
		t.Fatalf("privateKeyPath=%q", got)
	}
	summaryPath := filepath.Join(result.ArtifactsDir, "architecture-summary.json")
	data, err := os.ReadFile(summaryPath)
	if err != nil {
		t.Fatalf("read summary: %v", err)
	}
	var summary architectureSummary
	if err := json.Unmarshal(data, &summary); err != nil {
		t.Fatalf("unmarshal summary: %v", err)
	}
	if summary.CaseName != "architecture-gate" {
		t.Fatalf("summary case=%q", summary.CaseName)
	}
	if summary.WorkerURL != "https://worker.example.test" {
		t.Fatalf("summary workerURL=%q", summary.WorkerURL)
	}
	if summary.PrivateKeyPath != "/tmp/custom-rules.pem" {
		t.Fatalf("summary privateKeyPath=%q", summary.PrivateKeyPath)
	}
	if len(summary.OutputFiles) != 4 {
		t.Fatalf("summary output files=%d", len(summary.OutputFiles))
	}
	for _, name := range []string{"cli.verbose.ndjson", "cli.stdout.log", "cli.stderr.log"} {
		if _, err := os.Stat(filepath.Join(result.ArtifactsDir, name)); err != nil {
			t.Fatalf("artifact %s missing: %v", name, err)
		}
	}
}

func TestRunReleaseGate_FailsWhenEnglishSearchTermsNotLowercase(t *testing.T) {
	root := t.TempDir()
	inputDir := filepath.Join(root, "input")
	outDir := filepath.Join(root, "out")
	artifactsDir := filepath.Join(root, "artifacts")
	if err := os.MkdirAll(inputDir, 0o755); err != nil {
		t.Fatal(err)
	}
	inputPath := filepath.Join(inputDir, "demo.md")
	if err := os.WriteFile(inputPath, []byte("demo"), 0o644); err != nil {
		t.Fatal(err)
	}

	cliPath := filepath.Join(root, "syl-listing-pro")
	script := "#!/bin/sh\n" +
		"out=\"\"\n" +
		"while [ \"$#\" -gt 0 ]; do\n" +
		"  if [ \"$1\" = \"-o\" ] || [ \"$1\" = \"--out\" ]; then out=\"$2\"; shift 2; continue; fi\n" +
		"  shift\n" +
		"done\n" +
		"mkdir -p \"$out\"\n" +
		"printf '# EN\\n\\n## 搜索词\\nPaper Lanterns Classroom Decor\\n' > \"$out/demo_abcd_en.md\"\n" +
		"printf '# CN\\n\\n## 搜索词\\n中文搜索词\\n' > \"$out/demo_abcd_cn.md\"\n" +
		"printf 'docx' > \"$out/demo_abcd_en.docx\"\n" +
		"printf 'docx' > \"$out/demo_abcd_cn.docx\"\n" +
		"echo '任务完成：成功 1，失败 0，总耗时 1s'\n"
	if err := os.WriteFile(cliPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	svc := Service{
		CLIPath:       cliPath,
		ArtifactsRoot: artifactsDir,
		RulesRunner:   &fakeRulesRunner{},
		WorkerRunner:  &fakeWorkerRunner{},
	}
	_, err := svc.Run(context.Background(), RunInput{
		CaseName:    "release-gate",
		Tenant:      "syl",
		WorkerURL:   "https://worker.aelus.tech",
		SYLKey:      "key",
		AdminToken:  "admin",
		InputPath:   inputPath,
		OutputDir:   outDir,
		ArtifactsID: "run-uppercase",
	})
	if err == nil {
		t.Fatal("expected lowercase validation error")
	}
	if got := err.Error(); got == "" || !containsAll(got, "search_terms", "小写") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func containsAll(s string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(s, part) {
			return false
		}
	}
	return true
}

func TestRunUnknownCaseErrorMentionsAvailableCases(t *testing.T) {
	svc := Service{}
	_, err := svc.Run(context.Background(), RunInput{CaseName: "mystery-gate"})
	if err == nil {
		t.Fatal("Run() expected error")
	}
	if got := err.Error(); !containsAll(got, "未知 e2e 用例", "release-gate", "architecture-gate") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunMissingCLIMessageSuggestsBuildOrInstall(t *testing.T) {
	root := t.TempDir()
	inputPath := filepath.Join(root, "demo.md")
	outDir := filepath.Join(root, "out")
	if err := os.WriteFile(inputPath, []byte("demo"), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := Service{
		CLIPath:       filepath.Join(root, "missing-syl-listing-pro"),
		ArtifactsRoot: filepath.Join(root, "artifacts"),
		RulesRunner:   &fakeRulesRunner{},
		WorkerRunner:  &fakeWorkerRunner{},
	}
	_, err := svc.Run(context.Background(), RunInput{
		CaseName:   "release-gate",
		Tenant:     "syl",
		WorkerURL:  "https://worker.example.test",
		SYLKey:     "key",
		AdminToken: "admin",
		InputPath:  inputPath,
		OutputDir:  outDir,
	})
	if err == nil {
		t.Fatal("Run() expected error")
	}
	if got := err.Error(); !containsAll(got, "syl-listing-pro", "先构建或安装 CLI", "PATH") {
		t.Fatalf("unexpected error: %v", err)
	}
}
