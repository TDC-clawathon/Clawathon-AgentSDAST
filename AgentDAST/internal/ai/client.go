// Package ai implements the AI-orchestrated audit mode: an LLM (via an
// OpenAI-compatible endpoint) reads the spec and optional SAST report, calls
// the scanner tools to verify and discover vulnerabilities, and writes a final
// security audit report.
package ai

import (
	openai "github.com/sashabaranov/go-openai"

	"agentdast/pkg/types"
)

// newClient builds an OpenAI-compatible client honoring a custom base URL.
func newClient(cfg types.AIConfig) *openai.Client {
	clientCfg := openai.DefaultConfig(cfg.APIKey)
	if cfg.BaseURL != "" {
		clientCfg.BaseURL = cfg.BaseURL
	}
	return openai.NewClientWithConfig(clientCfg)
}
