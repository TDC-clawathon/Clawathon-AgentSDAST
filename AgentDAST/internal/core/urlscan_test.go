package core

import "testing"

func TestBuildEndpointFromURLAbsolute(t *testing.T) {
	ep, base, err := BuildEndpointFromURL("http://host:8080/users/search?q=test&p=1", "", "", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if base != "http://host:8080" {
		t.Errorf("base = %q", base)
	}
	if ep.Path != "/users/search" || ep.Method != "GET" {
		t.Errorf("path/method = %q %q", ep.Path, ep.Method)
	}
	if len(ep.Parameters) != 2 {
		t.Fatalf("expected 2 query params, got %d", len(ep.Parameters))
	}
	for _, p := range ep.Parameters {
		if p.In != "query" {
			t.Errorf("param %s in = %q", p.Name, p.In)
		}
		if p.Name == "q" && p.Example != "test" {
			t.Errorf("expected q example 'test', got %q", p.Example)
		}
	}
}

func TestBuildEndpointFromURLPathWithBase(t *testing.T) {
	ep, base, err := BuildEndpointFromURL("/rest/products/search?q=x", "POST", "http://localhost:3000/", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if base != "http://localhost:3000" {
		t.Errorf("base = %q", base)
	}
	if ep.Path != "/rest/products/search" || ep.Method != "POST" {
		t.Errorf("path/method = %q %q", ep.Path, ep.Method)
	}
}

func TestBuildEndpointFromURLPathWithoutBaseErrors(t *testing.T) {
	if _, _, err := BuildEndpointFromURL("/x", "GET", "", "", nil); err == nil {
		t.Fatal("expected error when path given without base URL")
	}
}

func TestBuildEndpointFromURLBodyParams(t *testing.T) {
	ep, _, err := BuildEndpointFromURL("http://h/profile", "POST", "", `{"a":1}`, []string{"role", "isAdmin"})
	if err != nil {
		t.Fatal(err)
	}
	if ep.RequestBody == nil || ep.RequestBody.Example != `{"a":1}` {
		t.Fatalf("body not set: %+v", ep.RequestBody)
	}
	var bodyCount int
	for _, p := range ep.Parameters {
		if p.In == "body" {
			bodyCount++
		}
	}
	if bodyCount != 2 {
		t.Errorf("expected 2 body params, got %d", bodyCount)
	}
}
