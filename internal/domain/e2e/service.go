package e2e

import (
	"bytes"
	"context"
	"encoding/json"
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
	Tenant         string
	WorkerURL      string
	AdminToken     string
	PrivateKeyPath string
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
	CaseName       string
	Tenant         string
	SYLKey         string
	AdminToken     string
	PrivateKeyPath string
	InputPath      string
	OutputDir      string
	WorkerURL      string
	ArtifactsID    string
}

type RunResult struct {
	ArtifactsDir string
	OutputFiles  []string
}

type architectureSummary struct {
	CaseName       string   `json:"case_name"`
	Tenant         string   `json:"tenant"`
	WorkerURL      string   `json:"worker_url"`
	PrivateKeyPath string   `json:"private_key_path,omitempty"`
	OutputFiles    []string `json:"output_files"`
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
	return []string{"release-gate", "architecture-gate"}
}

func (s Service) Run(ctx context.Context, in RunInput) (RunResult, error) {
	switch strings.TrimSpace(in.CaseName) {
	case "", "release-gate":
		return s.runReleaseGate(ctx, in)
	case "architecture-gate":
		return s.runArchitectureGate(ctx, in)
	default:
		return RunResult{}, fmt.Errorf("未知 e2e 用例: %s", in.CaseName)
	}
}

func (s Service) runReleaseGate(ctx context.Context, in RunInput) (RunResult, error) {
	return s.runGate(ctx, in, false)
}

func (s Service) runArchitectureGate(ctx context.Context, in RunInput) (RunResult, error) {
	return s.runGate(ctx, in, true)
}

func (s Service) runGate(ctx context.Context, in RunInput, withArchitectureSummary bool) (RunResult, error) {
	artifactDir, err := s.ensureArtifactsDir(in.ArtifactsID)
	if err != nil {
		return RunResult{}, err
	}
	if err := s.rulesRunner().Publish(ctx, PublishRulesInput{
		Tenant:         in.Tenant,
		WorkerURL:      in.WorkerURL,
		AdminToken:     in.AdminToken,
		PrivateKeyPath: in.PrivateKeyPath,
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
	if err := validateReleaseGateOutputs(files); err != nil {
		return RunResult{}, err
	}
	if withArchitectureSummary {
		if err := validateArchitectureGateArtifacts(artifactDir); err != nil {
			return RunResult{}, err
		}
		if err := writeArchitectureSummary(filepath.Join(artifactDir, "architecture-summary.json"), architectureSummary{
			CaseName:       "architecture-gate",
			Tenant:         in.Tenant,
			WorkerURL:      in.WorkerURL,
			PrivateKeyPath: in.PrivateKeyPath,
			OutputFiles:    files,
		}); err != nil {
			return RunResult{}, err
		}
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

func validateReleaseGateOutputs(files []string) error {
	enMarkdown := ""
	for _, file := range files {
		name := filepath.Base(file)
		if strings.HasSuffix(name, "_en.md") {
			enMarkdown = file
			break
		}
	}
	if enMarkdown == "" {
		return fmt.Errorf("缺少英文 markdown 产物")
	}
	return validateEnglishSearchTermsLowercase(enMarkdown)
}

func validateArchitectureGateArtifacts(artifactDir string) error {
	required := []string{
		"cli.verbose.ndjson",
		"cli.stdout.log",
		"cli.stderr.log",
	}
	for _, name := range required {
		path := filepath.Join(artifactDir, name)
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("缺少 architecture-gate artifacts: %s", path)
		}
	}
	return nil
}

func writeArchitectureSummary(path string, summary architectureSummary) error {
	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func validateEnglishSearchTermsLowercase(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取英文 markdown 失败: %w", err)
	}
	text := string(raw)
	marker := "## 搜索词\n"
	index := strings.Index(text, marker)
	if index < 0 {
		return fmt.Errorf("英文 markdown 缺少 search_terms 段落: %s", path)
	}
	rest := strings.TrimSpace(text[index+len(marker):])
	if rest == "" {
		return fmt.Errorf("英文 markdown 的 search_terms 为空: %s", path)
	}
	line := strings.TrimSpace(strings.Split(rest, "\n")[0])
	if line == "" {
		return fmt.Errorf("英文 markdown 的 search_terms 为空: %s", path)
	}
	if line != strings.ToLower(line) {
		return fmt.Errorf("英文 markdown 的 search_terms 不是全小写: %s", path)
	}
	return nil
}

type defaultRulesRunner struct {
	root string
}

func (r defaultRulesRunner) Publish(ctx context.Context, in PublishRulesInput) error {
	svc := drules.Service{Root: r.root}
	version := drules.GenerateVersion(in.Tenant)
	if _, err := svc.Package(in.Tenant, version, in.PrivateKeyPath); err != nil {
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
