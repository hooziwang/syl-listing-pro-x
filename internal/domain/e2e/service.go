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
	TestdataRoot  string
	RulesRoot     string
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

type listingComplianceRunSummary struct {
	Sample      string                       `json:"sample"`
	Passed      bool                         `json:"passed"`
	OutputFiles []string                     `json:"output_files,omitempty"`
	Violations  []listingComplianceViolation `json:"violations,omitempty"`
	Error       string                       `json:"error,omitempty"`
}

func NewDefaultService(paths config.Paths) Service {
	return Service{
		ArtifactsRoot: filepath.Join(paths.WorkspaceRoot, "syl-listing-pro-x", "artifacts"),
		TestdataRoot:  filepath.Join(paths.WorkspaceRoot, "syl-listing-pro-x", "testdata", "e2e"),
		RulesRoot:     paths.RulesRepo,
		RulesRunner: defaultRulesRunner{
			root: paths.RulesRepo,
		},
		WorkerRunner: defaultWorkerRunner{},
	}
}

func (s Service) ListCases() []string {
	return []string{"release-gate", "architecture-gate", "listing-compliance-gate", "single-listing-compliance-gate"}
}

func (s Service) Run(ctx context.Context, in RunInput) (RunResult, error) {
	switch strings.TrimSpace(in.CaseName) {
	case "", "release-gate":
		return s.runReleaseGate(ctx, in)
	case "architecture-gate":
		return s.runArchitectureGate(ctx, in)
	case "listing-compliance-gate":
		return s.runListingComplianceGate(ctx, in)
	case "single-listing-compliance-gate":
		return s.runSingleListingComplianceGate(ctx, in)
	default:
		return RunResult{}, fmt.Errorf("未知 e2e 用例: %s；可用值只有 release-gate、architecture-gate、listing-compliance-gate 或 single-listing-compliance-gate", in.CaseName)
	}
}

func (s Service) runReleaseGate(ctx context.Context, in RunInput) (RunResult, error) {
	return s.runGate(ctx, in, false)
}

func (s Service) runArchitectureGate(ctx context.Context, in RunInput) (RunResult, error) {
	return s.runGate(ctx, in, true)
}

func (s Service) runListingComplianceGate(ctx context.Context, in RunInput) (RunResult, error) {
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

	inputs, err := collectMarkdownInputs(s.testdataRoot())
	if err != nil {
		return RunResult{}, err
	}
	if len(inputs) == 0 {
		return RunResult{}, fmt.Errorf("未找到 listing-compliance-gate 输入样例: %s", s.testdataRoot())
	}

	allFiles := make([]string, 0)
	for _, inputPath := range inputs {
		sample := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
		sampleArtifactDir := filepath.Join(artifactDir, sample)
		if err := os.MkdirAll(sampleArtifactDir, 0o755); err != nil {
			return RunResult{}, err
		}
		sampleOutputDir := filepath.Join(in.OutputDir, sample)
		if err := os.MkdirAll(sampleOutputDir, 0o755); err != nil {
			return RunResult{}, err
		}

		stdout, stderr, logPath, runErr := executeCLI(ctx, cliPath, inputPath, sampleOutputDir, sampleArtifactDir)
		if err := os.WriteFile(filepath.Join(sampleArtifactDir, "cli.stdout.log"), stdout, 0o644); err != nil {
			return RunResult{}, err
		}
		if err := os.WriteFile(filepath.Join(sampleArtifactDir, "cli.stderr.log"), stderr, 0o644); err != nil {
			return RunResult{}, err
		}
		if runErr != nil {
			summary := listingComplianceRunSummary{
				Sample: sample,
				Passed: false,
				Error:  fmt.Sprintf("CLI 执行失败: %v", runErr),
			}
			if err := writeListingComplianceSummary(filepath.Join(sampleArtifactDir, "compliance-summary.json"), summary); err != nil {
				return RunResult{}, err
			}
			return RunResult{}, fmt.Errorf("%s: %s", sample, summary.Error)
		}
		if err := validateVerboseExecution(stderr, logPath); err != nil {
			summary := listingComplianceRunSummary{
				Sample: sample,
				Passed: false,
				Error:  fmt.Sprintf("CLI verbose 检查失败: %v", err),
			}
			if err := writeListingComplianceSummary(filepath.Join(sampleArtifactDir, "compliance-summary.json"), summary); err != nil {
				return RunResult{}, err
			}
			return RunResult{}, fmt.Errorf("%s: %s", sample, summary.Error)
		}

		files, err := collectOutputFiles(sampleOutputDir)
		if err != nil {
			return RunResult{}, err
		}
		if err := validateListingOutputFiles(files); err != nil {
			summary := listingComplianceRunSummary{
				Sample:      sample,
				Passed:      false,
				OutputFiles: files,
				Error:       err.Error(),
			}
			if err := writeListingComplianceSummary(filepath.Join(sampleArtifactDir, "compliance-summary.json"), summary); err != nil {
				return RunResult{}, err
			}
			return RunResult{}, fmt.Errorf("%s: %s", sample, err)
		}

		enPath, cnPath, err := findMarkdownPair(files)
		if err != nil {
			return RunResult{}, err
		}
		report, err := validateListingCompliance(s.rulesRoot(), in.Tenant, enPath, cnPath)
		if err != nil {
			return RunResult{}, err
		}

		summary := listingComplianceRunSummary{
			Sample:      sample,
			Passed:      report.Passed,
			OutputFiles: files,
			Violations:  report.Violations,
		}
		if !report.Passed {
			summary.Error = "listing 合规校验失败"
		}
		if err := writeListingComplianceSummary(filepath.Join(sampleArtifactDir, "compliance-summary.json"), summary); err != nil {
			return RunResult{}, err
		}
		if !report.Passed {
			return RunResult{}, fmt.Errorf("%s: listing 合规校验失败: %+v", sample, report.Violations)
		}
		allFiles = append(allFiles, files...)
	}

	sort.Strings(allFiles)
	return RunResult{
		ArtifactsDir: artifactDir,
		OutputFiles:  allFiles,
	}, nil
}

func (s Service) runSingleListingComplianceGate(ctx context.Context, in RunInput) (RunResult, error) {
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

	stdout, stderr, logPath, runErr := executeCLI(ctx, cliPath, in.InputPath, in.OutputDir, artifactDir)
	if err := os.WriteFile(filepath.Join(artifactDir, "cli.stdout.log"), stdout, 0o644); err != nil {
		return RunResult{}, err
	}
	if err := os.WriteFile(filepath.Join(artifactDir, "cli.stderr.log"), stderr, 0o644); err != nil {
		return RunResult{}, err
	}

	summary := listingComplianceRunSummary{
		Sample: strings.TrimSuffix(filepath.Base(in.InputPath), filepath.Ext(in.InputPath)),
	}
	if runErr != nil {
		summary.Passed = false
		summary.Error = fmt.Sprintf("CLI 执行失败: %v", runErr)
		if err := writeListingComplianceSummary(filepath.Join(artifactDir, "compliance-summary.json"), summary); err != nil {
			return RunResult{}, err
		}
		return RunResult{}, fmt.Errorf("%s: %s", summary.Sample, summary.Error)
	}
	if err := validateVerboseExecution(stderr, logPath); err != nil {
		summary.Passed = false
		summary.Error = fmt.Sprintf("CLI verbose 检查失败: %v", err)
		if err := writeListingComplianceSummary(filepath.Join(artifactDir, "compliance-summary.json"), summary); err != nil {
			return RunResult{}, err
		}
		return RunResult{}, fmt.Errorf("%s: %s", summary.Sample, summary.Error)
	}

	files, err := collectOutputFiles(in.OutputDir)
	if err != nil {
		return RunResult{}, err
	}
	summary.OutputFiles = files
	if err := validateListingOutputFiles(files); err != nil {
		summary.Passed = false
		summary.Error = err.Error()
		if err := writeListingComplianceSummary(filepath.Join(artifactDir, "compliance-summary.json"), summary); err != nil {
			return RunResult{}, err
		}
		return RunResult{}, fmt.Errorf("%s: %s", summary.Sample, summary.Error)
	}

	enPath, cnPath, err := findMarkdownPair(files)
	if err != nil {
		return RunResult{}, err
	}
	report, err := validateSingleListingRegression(s.rulesRoot(), in.Tenant, enPath, cnPath)
	if err != nil {
		return RunResult{}, err
	}
	summary.Passed = report.Passed
	summary.Violations = report.Violations
	if !report.Passed {
		summary.Error = "listing 合规校验失败"
	}
	if err := writeListingComplianceSummary(filepath.Join(artifactDir, "compliance-summary.json"), summary); err != nil {
		return RunResult{}, err
	}
	if !report.Passed {
		return RunResult{}, fmt.Errorf("%s: listing 合规校验失败: %+v", summary.Sample, report.Violations)
	}

	return RunResult{
		ArtifactsDir: artifactDir,
		OutputFiles:  files,
	}, nil
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
		if _, err := os.Stat(s.CLIPath); err != nil {
			if os.IsNotExist(err) {
				return "", fmt.Errorf("未找到 syl-listing-pro CLI: %s；先构建或安装 CLI，并确保路径存在或 PATH 可找到 syl-listing-pro", s.CLIPath)
			}
			return "", fmt.Errorf("检查 syl-listing-pro CLI 失败: %w", err)
		}
		return s.CLIPath, nil
	}
	path, err := exec.LookPath("syl-listing-pro")
	if err != nil {
		return "", fmt.Errorf("未找到 syl-listing-pro CLI：先构建或安装 CLI，并确保 PATH 中可执行 syl-listing-pro")
	}
	return path, nil
}

func (s Service) testdataRoot() string {
	if strings.TrimSpace(s.TestdataRoot) != "" {
		return s.TestdataRoot
	}
	return filepath.Join("testdata", "e2e")
}

func (s Service) rulesRoot() string {
	if strings.TrimSpace(s.RulesRoot) != "" {
		return s.RulesRoot
	}
	return filepath.Join("..", "rules")
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

func collectMarkdownInputs(dir string) ([]string, error) {
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
		if strings.HasSuffix(name, ".md") {
			out = append(out, filepath.Join(dir, name))
		}
	}
	sort.Strings(out)
	return out, nil
}

func executeCLI(ctx context.Context, cliPath string, inputPath string, outputDir string, artifactDir string) ([]byte, []byte, string, error) {
	logPath := filepath.Join(artifactDir, "cli.verbose.ndjson")
	args := []string{
		inputPath,
		"-o", outputDir,
		"--verbose",
		"--log-file", logPath,
	}
	cmd := exec.CommandContext(ctx, cliPath, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), logPath, err
}

func validateListingOutputFiles(files []string) error {
	required := []string{"_en.md", "_cn.md", "_en.docx", "_cn.docx"}
	for _, suffix := range required {
		found := false
		for _, file := range files {
			if strings.HasSuffix(filepath.Base(file), suffix) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("缺少产物: %s", suffix)
		}
	}
	return nil
}

func findMarkdownPair(files []string) (string, string, error) {
	enPath := ""
	cnPath := ""
	for _, file := range files {
		base := filepath.Base(file)
		switch {
		case strings.HasSuffix(base, "_en.md"):
			enPath = file
		case strings.HasSuffix(base, "_cn.md"):
			cnPath = file
		}
	}
	if enPath == "" {
		return "", "", fmt.Errorf("缺少英文 markdown 产物")
	}
	if cnPath == "" {
		return "", "", fmt.Errorf("缺少中文 markdown 产物")
	}
	return enPath, cnPath, nil
}

func writeListingComplianceSummary(path string, summary listingComplianceRunSummary) error {
	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
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
