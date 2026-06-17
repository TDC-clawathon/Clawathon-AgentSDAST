package ai

// Config configures a single SAST orchestration run.
type Config struct {
	BaseURL     string // OpenAI-compatible endpoint root (LLM_BASE_URL)
	APIKey      string // API key for the endpoint (LLM_API_KEY)
	Model       string // resolved model name for this job
	MaxTurns    int    // max tool-calling turns (safety backstop)
	WorkDir     string // job workspace; the sandbox root (contains ./raw, gets ./sast)
	BaseURLHint string // base URL supplied by the Manager (may be wrong; model verifies)
}

// WithDefaults returns a copy with sensible defaults applied.
func (c Config) WithDefaults() Config {
	if c.MaxTurns <= 0 {
		c.MaxTurns = 100
	}
	return c
}
