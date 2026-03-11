package rules

import "net/http"

type Service struct {
	Root       string
	HTTPClient *http.Client
}

type PackageOutput struct {
	PackageDir   string
	ArchivePath  string
	ManifestPath string
}

type Manifest struct {
	TenantID                        string `json:"tenant_id"`
	RulesVersion                    string `json:"rules_version"`
	ManifestSHA256                  string `json:"manifest_sha256"`
	SignatureBase64                 string `json:"signature_base64"`
	SignatureAlgo                   string `json:"signature_algo"`
	SigningPublicKeyPathInArchive   string `json:"signing_public_key_path_in_archive"`
	SigningPublicKeySignatureBase64 string `json:"signing_public_key_signature_base64"`
	SigningPublicKeySignatureAlgo   string `json:"signing_public_key_signature_algo"`
	Archive                         string `json:"archive"`
}

type PublishInput struct {
	Tenant     string
	Version    string
	WorkerURL  string
	AdminToken string
}

type PublishResponse struct {
	OK           bool   `json:"ok"`
	TenantID     string `json:"tenant_id"`
	RulesVersion string `json:"rules_version"`
}

type publishPayload struct {
	TenantID                        string `json:"tenant_id"`
	RulesVersion                    string `json:"rules_version"`
	ManifestSHA256                  string `json:"manifest_sha256"`
	ArchiveBase64                   string `json:"archive_base64"`
	SignatureBase64                 string `json:"signature_base64"`
	SignatureAlgo                   string `json:"signature_algo"`
	SigningPublicKeyPathInArchive   string `json:"signing_public_key_path_in_archive"`
	SigningPublicKeySignatureBase64 string `json:"signing_public_key_signature_base64"`
	SigningPublicKeySignatureAlgo   string `json:"signing_public_key_signature_algo"`
}
