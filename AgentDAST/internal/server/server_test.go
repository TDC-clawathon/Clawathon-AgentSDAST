package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newTestServer builds a Server without backends. Only request-validation paths
// (which run before any store/AI access) are exercised here.
func newTestServer() *Server {
	return &Server{cfg: LoadConfig(), sem: make(chan struct{}, 1), cancels: map[string]context.CancelFunc{}}
}

func TestScanValidation(t *testing.T) {
	s := newTestServer()
	h := s.Handler()

	// All cases return at validation, before any store/MinIO access: project_id
	// and base_url are both required, and base_url must be a valid http(s) URL.
	cases := []struct {
		name   string
		method string
		body   string
		want   int
	}{
		{"wrong method", http.MethodGet, "", http.StatusMethodNotAllowed},
		{"missing project_id", http.MethodPost, `{}`, http.StatusBadRequest},
		{"missing base_url", http.MethodPost, `{"project_id":"p1"}`, http.StatusBadRequest},
		{"bad base_url", http.MethodPost, `{"project_id":"p1","base_url":"not-a-url"}`, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/api/dast/scan", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("got %d, want %d (body: %s)", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}

func TestStatusRequiresScanID(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/dast/status", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got %d, want 400", rec.Code)
	}
}

func TestCancelValidation(t *testing.T) {
	s := newTestServer()
	h := s.Handler()

	cases := []struct {
		name   string
		method string
		target string
		want   int
	}{
		{"wrong method", http.MethodGet, "/api/dast/cancel", http.StatusMethodNotAllowed},
		{"missing id", http.MethodPost, "/api/dast/cancel", http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.target, nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("got %d, want %d (body: %s)", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}

func TestParseScanRequest(t *testing.T) {
	body := `{"project_id":"p1","base_url":"https://api.example.com","prompt":"focus on auth"}`
	req := httptest.NewRequest(http.MethodPost, "/api/dast/scan", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	got := parseScanRequest(req)
	if got.ProjectID != "p1" {
		t.Errorf("ProjectID = %q", got.ProjectID)
	}
	if got.BaseURL != "https://api.example.com" {
		t.Errorf("BaseURL = %q", got.BaseURL)
	}
	if got.Prompt != "focus on auth" {
		t.Errorf("Prompt = %q", got.Prompt)
	}
}

func TestWriteTempFile(t *testing.T) {
	cases := map[string]string{
		"specs/p1/api.json":     ".json",
		"specs/p1/api.yaml":     ".yaml",
		"sast/p1/report.sarif":  ".sarif",
		"sast/p1/report-nodext": ".yaml", // no extension → default
	}
	for key, wantExt := range cases {
		path, cleanup, err := WriteTempFile(key, []byte("data"))
		if err != nil {
			t.Fatalf("WriteTempFile(%q): %v", key, err)
		}
		if got := filepath.Ext(path); got != wantExt {
			t.Errorf("WriteTempFile(%q) ext = %q, want %q", key, got, wantExt)
		}
		if _, err := os.Stat(path); err != nil {
			t.Errorf("temp file %q not created: %v", path, err)
		}
		cleanup()
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("temp file %q not cleaned up", path)
		}
	}
}

func TestMySQLDSN(t *testing.T) {
	cfg := Config{MySQLUser: "u", MySQLPassword: "p", MySQLHost: "mysql", MySQLPort: "3306", MySQLDB: "db"}
	got := cfg.MySQLDSN()
	want := "u:p@tcp(mysql:3306)/db?parseTime=true&charset=utf8mb4&multiStatements=false"
	if got != want {
		t.Fatalf("DSN = %q, want %q", got, want)
	}
}
