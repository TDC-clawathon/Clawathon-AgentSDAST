package plugins

import "agentdast/internal/core"

// Register adds all built-in plugins to the given registry. It is called from
// main.go (and tests) so plugin activation is explicit rather than relying on
// import-time side effects.
func Register(r *core.PluginRegistry) {
	for _, p := range builtins() {
		r.Register(p)
	}
}

// RegisterGlobal registers all built-in plugins with the global registry.
func RegisterGlobal() {
	Register(core.GlobalRegistry)
}

func builtins() []core.Plugin {
	return []core.Plugin{
		&SQLInjectionPlugin{},
		&XSSPlugin{},
		&CommandInjectionPlugin{},
		&PathTraversalPlugin{},
		&IDORPlugin{},
		&BrokenAuthPlugin{},
		&SensitiveDataPlugin{},
		&CORSPlugin{},
		&XXEPlugin{},
		&SSRFPlugin{},
		&MassAssignmentPlugin{},
		&OpenRedirectPlugin{},
		&SSTIPlugin{},
	}
}
