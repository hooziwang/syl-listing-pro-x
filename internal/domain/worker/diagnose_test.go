package worker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDefaultServers(t *testing.T) {
	servers := DefaultServers()
	srv, ok := servers["syl-server"]
	if !ok {
		t.Fatal("missing syl-server")
	}
	if srv.Host != "159.75.124.28" {
		t.Fatalf("host=%q", srv.Host)
	}
	if srv.User != "ubuntu" {
		t.Fatalf("user=%q", srv.User)
	}
}

func TestDiagnoseExternal(t *testing.T) {
	var gotAuth string
	var gotGenerateAuth string
	var baseURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/healthz":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"tenant_id":"system","ok":true,"llm":{"deepseek":{"ok":true}}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/auth/exchange":
			gotAuth = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"tenant_id":"syl","access_token":"token-1"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/rules/resolve":
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]any{
				"up_to_date":    false,
				"rules_version": "rules-syl-1",
				"download_url":  baseURL + "/download/rules-syl-1",
			}
			_ = json.NewEncoder(w).Encode(resp)
		case r.Method == http.MethodPost && r.URL.Path == "/v1/rules/refresh":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true}`))
		case r.Method == http.MethodGet && r.URL.Path == "/download/rules-syl-1":
			_, _ = w.Write([]byte("tarball"))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/generate":
			gotGenerateAuth = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true,"job_id":"job_1"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/jobs/job_1/events":
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte("event: status\n"))
			_, _ = w.Write([]byte("data: {\"job_id\":\"job_1\",\"tenant_id\":\"syl\",\"status\":\"succeeded\",\"updated_at\":\"2026-03-12T00:00:01Z\"}\n\n"))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/jobs/job_1/result":
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]any{
				"ok":          true,
				"en_markdown": "en",
				"cn_markdown": "cn",
			}
			_ = json.NewEncoder(w).Encode(resp)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()
	baseURL = ts.URL

	svc := Service{
		HTTPClient: ts.Client(),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := svc.DiagnoseExternal(ctx, DiagnoseExternalInput{
		BaseURL:      ts.URL,
		SYLKey:       "key-123",
		WithGenerate: true,
		Timeout:      2 * time.Second,
	})
	if err != nil {
		t.Fatalf("DiagnoseExternal() error = %v", err)
	}
	if gotAuth != "Bearer key-123" {
		t.Fatalf("auth header=%q", gotAuth)
	}
	if gotGenerateAuth != "Bearer token-1" {
		t.Fatalf("generate auth=%q", gotGenerateAuth)
	}
}

func TestDiagnoseExternal_IgnoresOptionalProviderFailure(t *testing.T) {
	var baseURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/healthz":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"tenant_id":"system","ok":false,"llm":{"deepseek":{"ok":true,"required":true}}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/auth/exchange":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"tenant_id":"syl","access_token":"token-1"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/rules/resolve":
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]any{
				"up_to_date":    true,
				"rules_version": "rules-syl-1",
				"download_url":  baseURL + "/download/rules-syl-1",
			}
			_ = json.NewEncoder(w).Encode(resp)
		case r.Method == http.MethodPost && r.URL.Path == "/v1/rules/refresh":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true}`))
		case r.Method == http.MethodGet && r.URL.Path == "/download/rules-syl-1":
			_, _ = w.Write([]byte("tarball"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()
	baseURL = ts.URL

	svc := Service{
		HTTPClient: ts.Client(),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := svc.DiagnoseExternal(ctx, DiagnoseExternalInput{
		BaseURL:      ts.URL,
		SYLKey:       "key-123",
		WithGenerate: false,
	}); err != nil {
		t.Fatalf("DiagnoseExternal() error = %v", err)
	}
}

func TestDiagnoseExternal_RejectsUnexpectedTenant(t *testing.T) {
	var baseURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/healthz":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"tenant_id":"system","ok":true,"llm":{"deepseek":{"ok":true}}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/auth/exchange":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"tenant_id":"demo","access_token":"token-1"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/rules/resolve":
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]any{
				"up_to_date":    true,
				"rules_version": "rules-demo-1",
				"download_url":  baseURL + "/download/rules-demo-1",
			}
			_ = json.NewEncoder(w).Encode(resp)
		case r.Method == http.MethodPost && r.URL.Path == "/v1/rules/refresh":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true}`))
		case r.Method == http.MethodGet && r.URL.Path == "/download/rules-demo-1":
			_, _ = w.Write([]byte("tarball"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()
	baseURL = ts.URL

	svc := Service{
		HTTPClient: ts.Client(),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := svc.DiagnoseExternal(ctx, DiagnoseExternalInput{
		BaseURL:        ts.URL,
		SYLKey:         "key-123",
		ExpectedTenant: "syl",
		WithGenerate:   false,
	})
	if err == nil {
		t.Fatal("DiagnoseExternal() expected tenant mismatch error")
	}
	for _, want := range []string{"tenant_id", "demo", "syl"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q missing %q", err, want)
		}
	}
}

func TestDiagnoseExternal_RejectsUnexpectedTenantFromJobEvent(t *testing.T) {
	var baseURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/healthz":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"tenant_id":"system","ok":true,"llm":{"deepseek":{"ok":true}}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/auth/exchange":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"tenant_id":"syl","access_token":"token-1"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/rules/resolve":
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]any{
				"up_to_date":    true,
				"rules_version": "rules-syl-1",
				"download_url":  baseURL + "/download/rules-syl-1",
			}
			_ = json.NewEncoder(w).Encode(resp)
		case r.Method == http.MethodPost && r.URL.Path == "/v1/rules/refresh":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true}`))
		case r.Method == http.MethodGet && r.URL.Path == "/download/rules-syl-1":
			_, _ = w.Write([]byte("tarball"))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/generate":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true,"job_id":"job_1"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/jobs/job_1/events":
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte("event: status\n"))
			_, _ = w.Write([]byte("data: {\"job_id\":\"job_1\",\"tenant_id\":\"demo\",\"status\":\"succeeded\",\"updated_at\":\"2026-03-12T00:00:01Z\"}\n\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()
	baseURL = ts.URL

	svc := Service{
		HTTPClient: ts.Client(),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := svc.DiagnoseExternal(ctx, DiagnoseExternalInput{
		BaseURL:        ts.URL,
		SYLKey:         "key-123",
		ExpectedTenant: "syl",
		WithGenerate:   true,
		Timeout:        2 * time.Second,
	})
	if err == nil {
		t.Fatal("DiagnoseExternal() expected job event tenant mismatch error")
	}
	for _, want := range []string{"tenant_id", "demo", "syl"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q missing %q", err, want)
		}
	}
}
