package worker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDefaultServers(t *testing.T) {
	servers := DefaultServers()
	srv, ok := servers["syl-server"]
	if !ok {
		t.Fatal("missing syl-server")
	}
	if srv.Host != "43.135.112.167" {
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
			_, _ = w.Write([]byte(`{"tenant_id":"system","ok":true,"llm":{"fluxcode":{"ok":true},"deepseek":{"ok":true}}}`))
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
		case r.Method == http.MethodGet && r.URL.Path == "/v1/jobs/job_1":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true,"status":"succeeded"}`))
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
		Interval:     10 * time.Millisecond,
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
