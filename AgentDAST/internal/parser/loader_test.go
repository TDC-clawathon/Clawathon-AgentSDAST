package parser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSpecOpenAPI3(t *testing.T) {
	path := filepath.Join("..", "..", "examples", "petstore.yaml")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("example spec not present: %v", err)
	}
	spec, err := LoadSpec(path)
	if err != nil {
		t.Fatalf("LoadSpec: %v", err)
	}
	if spec.Title == "" {
		t.Error("expected a title")
	}
	if len(spec.Endpoints) == 0 {
		t.Fatal("expected endpoints")
	}

	// Find the secured /profile POST and confirm extraction details.
	var found bool
	for _, ep := range spec.Endpoints {
		if ep.Path == "/profile" && ep.Method == "POST" {
			found = true
			if !ep.Secured {
				t.Error("expected /profile POST to be marked secured")
			}
			if ep.RequestBody == nil {
				t.Error("expected /profile POST to have a request body")
			}
		}
	}
	if !found {
		t.Error("did not extract POST /profile")
	}
}

func TestResolveInsertPoints(t *testing.T) {
	ep := Endpoint{Parameters: []ParamInfo{
		{Name: "a", In: "query"},
		{Name: "b", In: "query"},
		{Name: "id", In: "path"},
	}}

	// Empty → all declared params.
	if got := ep.ResolveInsertPoints(nil); len(got) != 3 {
		t.Fatalf("nil should return all, got %d", len(got))
	}

	// Multiple bare names.
	got := ep.ResolveInsertPoints([]string{"a", "b"})
	if len(got) != 2 || got[0].Name != "a" || got[1].Name != "b" {
		t.Fatalf("filter [a b] = %v", got)
	}

	// Location-qualified match.
	got = ep.ResolveInsertPoints([]string{"path:id"})
	if len(got) != 1 || got[0].In != "path" || got[0].Name != "id" {
		t.Fatalf("path:id = %v", got)
	}

	// Undeclared header point is synthesized so it can still be injected.
	got = ep.ResolveInsertPoints([]string{"header:X-Custom"})
	if len(got) != 1 || got[0].In != "header" || got[0].Name != "X-Custom" {
		t.Fatalf("header:X-Custom = %v", got)
	}
}

func TestParseInsertPoint(t *testing.T) {
	cases := map[string][2]string{
		"header:X-Api":  {"header", "X-Api"},
		"query:q":       {"query", "q"},
		"q":             {"", "q"},
		"X-No-Location": {"", "X-No-Location"}, // unknown prefix → bare name
	}
	for in, want := range cases {
		loc, name := ParseInsertPoint(in)
		if loc != want[0] || name != want[1] {
			t.Errorf("ParseInsertPoint(%q) = (%q,%q), want (%q,%q)", in, loc, name, want[0], want[1])
		}
	}
}
