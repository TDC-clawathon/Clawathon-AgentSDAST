package core

import (
	"testing"

	"agentdast/pkg/types"
)

type fakePlugin struct{ name, cat string }

func (f *fakePlugin) Name() string                      { return f.name }
func (f *fakePlugin) Description() string               { return "fake" }
func (f *fakePlugin) Category() string                  { return f.cat }
func (f *fakePlugin) Severity() types.Severity          { return types.SeverityLow }
func (f *fakePlugin) DefaultPayloads() []string         { return nil }
func (f *fakePlugin) Test(*ScanContext) []types.Finding { return nil }

func TestRegistryRegisterAndResolve(t *testing.T) {
	r := NewRegistry()
	r.Register(&fakePlugin{name: "b", cat: "injection"})
	r.Register(&fakePlugin{name: "a", cat: "auth"})

	all := r.List()
	if len(all) != 2 {
		t.Fatalf("expected 2 plugins, got %d", len(all))
	}
	if all[0].Name() != "a" || all[1].Name() != "b" {
		t.Fatalf("expected sorted [a b], got [%s %s]", all[0].Name(), all[1].Name())
	}

	// Empty resolve = all.
	got, err := r.Resolve(nil)
	if err != nil || len(got) != 2 {
		t.Fatalf("Resolve(nil) = %v, %v", len(got), err)
	}

	// Named resolve.
	got, err = r.Resolve([]string{"a"})
	if err != nil || len(got) != 1 || got[0].Name() != "a" {
		t.Fatalf("Resolve([a]) = %v, %v", got, err)
	}

	// Unknown plugin errors.
	if _, err := r.Resolve([]string{"nope"}); err == nil {
		t.Fatal("expected error for unknown plugin")
	}
}

func TestRegistryDuplicatePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on duplicate registration")
		}
	}()
	r := NewRegistry()
	r.Register(&fakePlugin{name: "dup"})
	r.Register(&fakePlugin{name: "dup"})
}
