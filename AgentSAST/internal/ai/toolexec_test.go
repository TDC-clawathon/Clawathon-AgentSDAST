package ai

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestExec(t *testing.T) *Executor {
	t.Helper()
	return NewExecutor(t.TempDir(), &skillStore{refs: map[string]string{"sqli": "test the q param"}})
}

func TestResolveRejectsEscape(t *testing.T) {
	e := newTestExec(t)
	for _, bad := range []string{"../etc/passwd", "/etc/passwd", "a/../../b", ""} {
		if _, err := e.resolve(bad); err == nil {
			t.Errorf("resolve(%q) should have failed", bad)
		}
	}
	if _, err := e.resolve("raw/app/main.go"); err != nil {
		t.Errorf("resolve of a normal path failed: %v", err)
	}
}

func TestWriteArtifactRestrictsNames(t *testing.T) {
	e := newTestExec(t)
	if out := e.WriteArtifact(json.RawMessage(`{"name":"../escape","content":"x"}`)); !strings.Contains(out, "error") {
		t.Errorf("expected rejection, got %q", out)
	}
	out := e.WriteArtifact(json.RawMessage(`{"name":"base_url.txt","content":"https://x/api\nignored"}`))
	if strings.Contains(out, "error") {
		t.Fatalf("write failed: %q", out)
	}
	b, _ := os.ReadFile(filepath.Join(e.Root, "sast", "base_url.txt"))
	if got := string(b); got != "https://x/api" {
		t.Errorf("base_url.txt = %q, want single line", got)
	}
}

func TestExtractZipBlocksZipSlip(t *testing.T) {
	e := newTestExec(t)
	// craft a zip with a traversal entry
	zpath := filepath.Join(e.Root, "raw")
	_ = os.MkdirAll(zpath, 0o755)
	archive := filepath.Join(zpath, "bad.zip")
	f, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, _ := zw.Create("../evil.txt")
	_, _ = w.Write([]byte("pwn"))
	zw.Close()
	f.Close()

	out := e.ExtractArchive(json.RawMessage(`{"path":"raw/bad.zip","dest":"raw/out"}`))
	if !strings.Contains(out, "error") {
		t.Errorf("zip-slip should be rejected, got %q", out)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(e.Root), "evil.txt")); err == nil {
		t.Errorf("zip-slip wrote outside the sandbox")
	}
}

func TestExtractAndReadRoundTrip(t *testing.T) {
	e := newTestExec(t)
	raw := filepath.Join(e.Root, "raw")
	_ = os.MkdirAll(raw, 0o755)
	archive := filepath.Join(raw, "src.zip")
	f, _ := os.Create(archive)
	zw := zip.NewWriter(f)
	w, _ := zw.Create("app/main.go")
	_, _ = w.Write([]byte("package main\n// SELECT * FROM users WHERE id=\nfunc main(){}"))
	zw.Close()
	f.Close()

	if out := e.ExtractArchive(json.RawMessage(`{"path":"raw/src.zip"}`)); strings.Contains(out, "error") {
		t.Fatalf("extract failed: %q", out)
	}
	rd := e.ReadFile(json.RawMessage(`{"path":"raw/extracted/app/main.go"}`))
	if !strings.Contains(rd, "package main") {
		t.Errorf("read_file did not return content: %q", rd)
	}
	sr := e.SearchCode(json.RawMessage(`{"pattern":"SELECT .* FROM"}`))
	if !strings.Contains(sr, "main.go") {
		t.Errorf("search_code missed the match: %q", sr)
	}
}

func TestValidateOpenAPI(t *testing.T) {
	e := newTestExec(t)
	_ = os.MkdirAll(filepath.Join(e.Root, "sast"), 0o755)
	good := "openapi: 3.0.3\ninfo:\n  title: t\n  version: '1'\npaths: {}\n"
	_ = os.WriteFile(filepath.Join(e.Root, "sast", "openapi.yaml"), []byte(good), 0o644)
	if out := e.ValidateOpenAPI(json.RawMessage(`{}`)); out != "valid" {
		t.Errorf("expected valid, got %q", out)
	}
	_ = os.WriteFile(filepath.Join(e.Root, "sast", "openapi.yaml"), []byte("not: an: openapi"), 0o644)
	if out := e.ValidateOpenAPI(json.RawMessage(`{}`)); !strings.Contains(out, "INVALID") {
		t.Errorf("expected INVALID, got %q", out)
	}
}

func TestKnowledgeAliases(t *testing.T) {
	e := newTestExec(t)
	if got := e.Knowledge("sql injection"); !strings.Contains(got, "test the q param") {
		t.Errorf("alias lookup failed: %q", got)
	}
}
