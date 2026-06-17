// Package ai runs the SAST stage as an OpenAI-compatible chat-completions loop
// with function/tool calling — the same model-invocation pattern AgentDAST uses.
// It replaces the former Codex CLI. The SAST skill content (skills/sast/SKILL.md
// + references/*.md) is injected as the system prompt / on-demand knowledge.
//
// The model drives an agentic loop over local-filesystem tools (extract the
// uploaded archive, read/search source, write artifacts, validate OpenAPI) and
// produces sast/openapi.yaml + sast/report.md + sast/base_url.txt in the job
// workdir, exactly the contract job.go expects.
package ai

import openai "github.com/sashabaranov/go-openai"

// newClient builds an OpenAI-compatible client honoring a custom base URL.
func newClient(baseURL, apiKey string) *openai.Client {
	cfg := openai.DefaultConfig(apiKey)
	if baseURL != "" {
		cfg.BaseURL = baseURL
	}
	return openai.NewClientWithConfig(cfg)
}
