package toolexec_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"agentdast/internal/core"
	"agentdast/internal/plugins"
	"agentdast/internal/toolexec"
)

// TestScanAppliesDefaultHeadersAndDetectsSQLi verifies the model's scan_api code
// path (Executor.Scan): the executor's default headers (e.g. an auth token set
// via the AI --header flag) reach the target, AND SQLi is detected from a SQLite
// error signature — the juice-shop case that previously slipped through.
func TestScanAppliesDefaultHeadersAndDetectsSQLi(t *testing.T) {
	var (
		mu      sync.Mutex
		sawAuth string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		if a := r.Header.Get("Authorization"); a != "" {
			sawAuth = a
		}
		mu.Unlock()
		q := r.URL.Query().Get("q")
		if strings.ContainsAny(q, "'\"") {
			// Mimic juice-shop / sequelize + sqlite error output.
			w.WriteHeader(500)
			w.Write([]byte(`{"error":{"message":"SQLITE_ERROR: unrecognized token near \"` + q + `\""}}`))
			return
		}
		w.Write([]byte(`{"results":[]}`))
	}))
	defer srv.Close()

	reg := core.NewRegistry()
	plugins.Register(reg)
	exec := toolexec.New(reg, nil)
	exec.BaseURL = srv.URL
	exec.DefaultHeaders = map[string]string{"Authorization": "Bearer SECRET"}

	args, _ := json.Marshal(map[string]any{
		"target_url": "/rest/products/search?q=test",
		"plugins":    []string{"sqli"},
	})
	result, err := exec.Scan(context.Background(), args)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	if sawAuth != "Bearer SECRET" {
		t.Errorf("target did not receive the default Authorization header; saw %q", sawAuth)
	}
	if result.Summary.TotalFindings == 0 {
		t.Fatal("expected SQLi to be detected from SQLITE_ERROR signature, got 0 findings")
	}
	if result.Findings[0].Plugin != "sqli" {
		t.Errorf("expected sqli finding, got %q", result.Findings[0].Plugin)
	}
}

// TestScanCallerHeaderOverridesDefault confirms a header supplied on the call
// wins over the executor default (the model can override per call).
func TestScanCallerHeaderOverridesDefault(t *testing.T) {
	var mu sync.Mutex
	var sawAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		sawAuth = r.Header.Get("Authorization")
		mu.Unlock()
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	reg := core.NewRegistry()
	plugins.Register(reg)
	exec := toolexec.New(reg, nil)
	exec.BaseURL = srv.URL
	exec.DefaultHeaders = map[string]string{"Authorization": "Bearer DEFAULT"}

	args, _ := json.Marshal(map[string]any{
		"target_url": "/x?a=1",
		"plugins":    []string{"sqli"},
		"headers":    map[string]string{"Authorization": "Bearer CALLER"},
	})
	if _, err := exec.Scan(context.Background(), args); err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if sawAuth != "Bearer CALLER" {
		t.Errorf("caller header should override default; saw %q", sawAuth)
	}
}

// TestScanLogsAndKnowledge verifies that a scan captures its request/response
// exchanges (retrievable by scan_id) and that the knowledge base is reachable.
func TestScanLogsAndKnowledge(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.ContainsAny(r.URL.Query().Get("q"), "'\"") {
			w.WriteHeader(500)
			w.Write([]byte("SQLITE_ERROR: unrecognized token"))
			return
		}
		w.Write([]byte(`{"results":[]}`))
	}))
	defer srv.Close()

	reg := core.NewRegistry()
	plugins.Register(reg)
	exec := toolexec.New(reg, nil)
	exec.BaseURL = srv.URL

	args, _ := json.Marshal(map[string]any{"target_url": "/search?q=test", "plugins": []string{"sqli"}})
	result, err := exec.Scan(context.Background(), args)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	// Per-scan logs must be retrievable by scan_id and show the actual exchange.
	logs, err := exec.ScanLogs(context.Background(), result.ID, 0)
	if err != nil {
		t.Fatalf("ScanLogs: %v", err)
	}
	if !strings.Contains(logs, "/search") || !strings.Contains(strings.ToLower(logs), "sqlite_error") {
		t.Errorf("scan logs missing request/response detail:\n%s", logs)
	}

	// Knowledge is reachable via the executor (alias resolves).
	if k := exec.Knowledge("sql injection"); !strings.Contains(k, "SQL Injection") {
		t.Errorf("Knowledge(\"sql injection\") unexpected: %s", k[:min(80, len(k))])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
