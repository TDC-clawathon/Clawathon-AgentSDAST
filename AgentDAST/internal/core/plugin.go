package core

import "agentdast/pkg/types"

// Plugin is the contract every vulnerability scanner must fulfill.
//
// The interface lives in the core package (rather than the plugins package)
// so that the registry and scanner can reference it without creating an
// import cycle: concrete plugins import core to obtain ScanContext.
type Plugin interface {
	// Name is a machine-readable identifier used in CLI flags and the registry.
	Name() string
	// Description is the human-readable explanation shown by `agentdast plugins`.
	Description() string
	// Category groups related plugins: injection | auth | exposure | config.
	Category() string
	// Severity is the default severity assigned to findings from this plugin.
	Severity() types.Severity
	// DefaultPayloads returns the built-in payload set for this plugin.
	DefaultPayloads() []string
	// Test executes the plugin against the given context and returns findings.
	// Implementations must honor ctx.Ctx.Done() for cancellation.
	Test(ctx *ScanContext) []types.Finding
}
