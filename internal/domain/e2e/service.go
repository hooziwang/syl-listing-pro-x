package e2e

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/hooziwang/syl-listing-pro-x/internal/config"
	drules "github.com/hooziwang/syl-listing-pro-x/internal/domain/rules"
	dworker "github.com/hooziwang/syl-listing-pro-x/internal/domain/worker"
)

type PublishRulesInput struct {
	Tenant     string
	WorkerURL  string
	AdminToken string
}

type DiagnoseWorkerInput struct {
	BaseURL string
	SYLKey  string
}

type RulesRunner interface {
	Publish(ctx context.Context, in PublishRulesInput) error
}

type WorkerRunner interface {
	DiagnoseExternal(ctx context.Context, in DiagnoseWorkerInput) error
}

type Service struct {
	CLIPath       string
	ArtifactsRoot string
	RulesRunner   RulesRunner
	WorkerRunner  WorkerRunner
}

type RunInput struct {
	CaseName    string
	Tenant      string
	SYLKey      string
	AdminToken  string
	InputPath   string
	OutputDir   string
	WorkerURL   string
	ArtifactsID string
}

type RunResult struct {
	ArtifactsDir string
	OutputFiles  []string
}

func NewDefaultService(paths config.Paths) Service {
	return Service{
		ArtifactsRoot: filepath.Join(paths.WorkspaceRoot, "syl-listing-pro-x", "artifacts"),
		RulesRunner: defaultRulesRunner{
			root: paths.RulesRepo,
		},
		WorkerRunner: defaultWorkerRunner{},
	}
}

func (s Service) ListCases() []string {
	return []string{"release-gate"}
}

func (s Service) Run(ctx context.Context, in RunInput) (RunResult, error) {
	switch strings.TrimSpace(in.CaseName) {
	case "", "release-gate":
		return s.runReleaseGate(ctx, in)
	default:
		return RunResult{}, fmt.Errorf("未知 e2e 用例: %s", in.CaseName)
	}
}

func (s Service) runReleaseGate(ctx context.Context, in RunInput) (RunResult, error) {
	artifactDir, err := s.ensureArtifactsDir(in.ArtifactsID)
	if err != nil {
		return RunResult{}, err
	}
	if err := s.rulesRunner().Publish(ctx, PublishRulesInput{
		Tenant:     in.Tenant,
		WorkerURL:  in.WorkerURL,
		AdminToken: in.AdminToken,
	}); err != nil {
		return RunResult{}, err
	}
	if err := s.workerRunner().DiagnoseExternal(ctx, DiagnoseWorkerInput{
		BaseURL: in.WorkerURL,
		SYLKey:  in.SYLKey,
	}); err != nil {
		return RunResult{}, err
	}

	cliPath, err := s.cliPath()
	if err != nil {
		return RunResult{}, err
	}
	if err := os.MkdirAll(in.OutputDir, 0o755); err != nil {
		return RunResult{}, err
	}
	args := []string{
		in.InputPath,
		"-o", in.OutputDir,
		"--verbose",
		"--log-file", filepath.Join(artifactDir, "cli.verbose.ndjson"),
	}
	cmd := exec.CommandContext(ctx, cliPath, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	if err := os.WriteFile(filepath.Join(artifactDir, "cli.stdout.log"), stdout.Bytes(), 0o644); err != nil {
		return RunResult{}, err
	}
	if err := os.WriteFile(filepath.Join(artifactDir, "cli.stderr.log"), stderr.Bytes(), 0o644); err != nil {
		return RunResult{}, err
	}
	if runErr != nil {
		return RunResult{}, fmt.Errorf("CLI 执行失败: %w", runErr)
	}

	files, err := collectOutputFiles(in.OutputDir)
	if err != nil {
		return RunResult{}, err
	}
	if len(files) < 4 {
		return RunResult{}, fmt.Errorf("产物不足: %d < 4", len(files))
	}
	return RunResult{
		ArtifactsDir: artifactDir,
		OutputFiles:  files,
	}, nil
}

func (s Service) ensureArtifactsDir(id string) (string, error) {
	root := s.ArtifactsRoot
	if root == "" {
		root = filepath.Join(".", "artifacts")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", err
	}
	if strings.TrimSpace(id) == "" {
		id = time.Now().UTC().Format("20060102-150405")
	}
	dir := filepath.Join(root, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func (s Service) cliPath() (string, error) {
	if strings.TrimSpace(s.CLIPath) != "" {
		return s.CLIPath, nil
	}
	return exec.LookPath("syl-listing-pro")
}

func (s Service) rulesRunner() RulesRunner {
	if s.RulesRunner != nil {
		return s.RulesRunner
	}
	return defaultRulesRunner{}
}

func (s Service) workerRunner() WorkerRunner {
	if s.WorkerRunner != nil {
		return s.WorkerRunner
	}
	return defaultWorkerRunner{}
}

func collectOutputFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".md") || strings.HasSuffix(name, ".docx") {
			out = append(out, filepath.Join(dir, name))
		}
	}
	sort.Strings(out)
	return out, nil
}

type defaultRulesRunner struct {
	root string
}

func (r defaultRulesRunner) Publish(ctx context.Context, in PublishRulesInput) error {
	svc := drules.Service{Root: r.root}
	version := drules.GenerateVersion(in.Tenant)
	privateKey := filepath.Join(r.root, "keys", "rules_private.pem")
	if _, err := svc.Package(in.Tenant, version, privateKey); err != nil {
		return err
	}
	_, err := svc.Publish(ctx, drules.PublishInput{
		Tenant:     in.Tenant,
		Version:    version,
		WorkerURL:  in.WorkerURL,
		AdminToken: in.AdminToken,
	})
	return err
}

type defaultWorkerRunner struct{}

func (defaultWorkerRunner) DiagnoseExternal(ctx context.Context, in DiagnoseWorkerInput) error {
	svc := dworker.Service{}
	return svc.DiagnoseExternal(ctx, dworker.DiagnoseExternalInput{
		BaseURL:      in.BaseURL,
		SYLKey:       in.SYLKey,
		WithGenerate: false,
	})
}
