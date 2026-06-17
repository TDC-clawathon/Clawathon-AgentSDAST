package core

import (
	"fmt"
	"sort"
	"sync"
)

// PluginRegistry holds all registered Plugin implementations, keyed by name.
type PluginRegistry struct {
	mu      sync.RWMutex
	plugins map[string]Plugin
}

// GlobalRegistry is the process-wide registry populated by plugins/all.go.
var GlobalRegistry = NewRegistry()

// NewRegistry returns an empty registry.
func NewRegistry() *PluginRegistry {
	return &PluginRegistry{plugins: make(map[string]Plugin)}
}

// Register adds a plugin, panicking on duplicate names (a programming error).
func (r *PluginRegistry) Register(p Plugin) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.plugins[p.Name()]; exists {
		panic(fmt.Sprintf("duplicate plugin registration: %s", p.Name()))
	}
	r.plugins[p.Name()] = p
}

// Get returns the plugin with the given name.
func (r *PluginRegistry) Get(name string) (Plugin, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.plugins[name]
	return p, ok
}

// List returns all registered plugins, sorted by name.
func (r *PluginRegistry) List() []Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Plugin, 0, len(r.plugins))
	for _, p := range r.plugins {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	return out
}

// Resolve returns the plugins named in names, or all plugins when names is empty.
func (r *PluginRegistry) Resolve(names []string) ([]Plugin, error) {
	if len(names) == 0 {
		return r.List(), nil
	}
	out := make([]Plugin, 0, len(names))
	for _, name := range names {
		p, ok := r.Get(name)
		if !ok {
			return nil, fmt.Errorf("unknown plugin: %q", name)
		}
		out = append(out, p)
	}
	return out, nil
}
