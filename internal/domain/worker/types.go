package worker

import (
	"context"
	"net/http"
	"time"
)

type Server struct {
	Name string
	Host string
	User string
	Port int
	Dir  string
}

type Service struct {
	HTTPClient *http.Client
	WorkerRepo string
	Remote     RemoteExecutor
	Servers    map[string]Server
}

type DiagnoseExternalInput struct {
	BaseURL      string
	SYLKey       string
	WithGenerate bool
	Timeout      time.Duration
	Interval     time.Duration
}

type PushEnvInput struct {
	Server string
}

type LogsInput struct {
	Server   string
	Services []string
	Tail     int
	Since    string
	NoFollow bool
}

type DeployInput struct {
	Server        string
	SkipBuild     bool
	StopLegacy    bool
	InstallDocker bool
	SkipWaitHTTPS bool
	HTTPSTimeout  int
	HTTPSInterval int
	SkipDiagnose  bool
}

type CheckRemoteVersionInput struct {
	BaseURL    string
	AdminToken string
}

type RemoteVersionInfo struct {
	OK            bool              `json:"ok"`
	TenantID      string            `json:"tenant_id"`
	Service       string            `json:"service"`
	GitCommit     string            `json:"git_commit"`
	BuildTime     string            `json:"build_time"`
	DeployedAt    string            `json:"deployed_at"`
	RulesVersions map[string]string `json:"rules_versions"`
}

type CheckRemoteVersionResult struct {
	LocalGitCommit string
	Remote         RemoteVersionInfo
	UpToDate       bool
}

type RemoteExecutor interface {
	Copy(ctx context.Context, server Server, src, dst string) error
	Run(ctx context.Context, server Server, cmd string) error
	Stream(ctx context.Context, server Server, cmd string) error
}

type healthResponse struct {
	OK  bool `json:"ok"`
	LLM struct {
		Deepseek providerHealth `json:"deepseek"`
	} `json:"llm"`
}

type providerHealth struct {
	OK       bool  `json:"ok"`
	Required *bool `json:"required"`
}

type authExchangeResponse struct {
	TenantID    string `json:"tenant_id"`
	AccessToken string `json:"access_token"`
}

type resolveRulesResponse struct {
	RulesVersion string `json:"rules_version"`
	DownloadURL  string `json:"download_url"`
}

type generateResponse struct {
	JobID string `json:"job_id"`
}

type jobStatusResponse struct {
	Status string `json:"status"`
	Error  string `json:"error"`
}

type jobResultResponse struct {
	ENMarkdown string `json:"en_markdown"`
	CNMarkdown string `json:"cn_markdown"`
}

type workerConfig struct {
	Server struct {
		Domain           string `json:"domain"`
		LetsencryptEmail string `json:"letsencrypt_email"`
	} `json:"server"`
}

func DefaultServers() map[string]Server {
	return map[string]Server{
		"syl-server": {
			Name: "syl-server",
			Host: "43.135.112.167",
			User: "ubuntu",
			Port: 22,
			Dir:  "/opt/syl-listing-worker",
		},
	}
}
