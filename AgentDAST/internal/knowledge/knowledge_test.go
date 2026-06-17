package knowledge

import (
	"strings"
	"testing"
)

func TestGetKnownAndAliases(t *testing.T) {
	cases := []string{"idor", "IDOR", "bola", "Broken Object Level Authorization",
		"sqli", "sql injection", "ssrf", "ssti", "template injection"}
	for _, name := range cases {
		content, ok := Get(name)
		if !ok {
			t.Errorf("Get(%q) = not found, want a document", name)
			continue
		}
		if !strings.Contains(content, "#") {
			t.Errorf("Get(%q) returned non-markdown content", name)
		}
	}
}

func TestGetUnknown(t *testing.T) {
	if _, ok := Get("definitely-not-a-vuln"); ok {
		t.Error("expected unknown topic to be not found")
	}
}

func TestListCoversPlugins(t *testing.T) {
	got := List()
	want := []string{"auth", "cmdi", "cors", "idor", "mass_assignment", "open_redirect",
		"path_traversal", "sensitive_data", "sqli", "ssrf", "ssti", "xss", "xxe"}
	set := make(map[string]bool, len(got))
	for _, g := range got {
		set[g] = true
	}
	for _, w := range want {
		if !set[w] {
			t.Errorf("knowledge base missing %q (have: %v)", w, got)
		}
	}
}
