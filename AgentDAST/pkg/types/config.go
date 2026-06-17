package types

// OutputMode controls how much detail a scan result carries.
type OutputMode string

const (
	OutputModeFull    OutputMode = "full"    // findings + every request/response log
	OutputModeResults OutputMode = "results" // findings only
)

// ScanConfig is the complete description of a single scan run.
//
// A scan targets either a single endpoint (TargetURL) or a whole API described
// by a spec (SwaggerSource). At least one must be set; TargetURL takes priority.
type ScanConfig struct {
	// SwaggerSource is a file path or HTTP(S) URL to an OpenAPI/Swagger document.
	// When set, every endpoint in the spec is scanned.
	SwaggerSource string `json:"swagger_source,omitempty"`
	// TargetURL is a single endpoint (full URL or path) to scan directly, without
	// a spec. If it is a path, it is resolved against TargetBaseURL.
	TargetURL string `json:"target_url,omitempty"`
	// Method is the HTTP method for a single-URL scan (default GET).
	Method string `json:"method,omitempty"`
	// Body is the raw request body for a single-URL scan (e.g. JSON for POST).
	Body string `json:"body,omitempty"`
	// BodyParams are JSON body field names to fuzz in a single-URL scan.
	BodyParams []string `json:"body_params,omitempty"`
	// TargetBaseURL overrides the base URL derived from the spec, and is the base
	// for resolving a relative TargetURL.
	TargetBaseURL string `json:"target_base_url,omitempty"`
	// CustomHeaders are injected into every request (e.g. Authorization).
	CustomHeaders map[string]string `json:"custom_headers,omitempty"`
	// CustomParams are extra query/body params merged into every request.
	CustomParams map[string]string `json:"custom_params,omitempty"`
	// Plugins to run by name; empty means run all registered plugins.
	Plugins []string `json:"plugins,omitempty"`
	// InsertPoints restricts where payloads are injected. Each entry is either a
	// bare name (matched in any location) or a location-qualified "loc:name"
	// where loc is one of query|header|path|cookie|body. Empty means every
	// parameter declared on the endpoint. A point not present in the spec is
	// injected anyway (e.g. an undeclared header), so callers can target
	// arbitrary positions. Multiple points are supported.
	InsertPoints []string `json:"insert_point,omitempty"`
	// OutputMode selects full logs vs results-only.
	OutputMode OutputMode `json:"output_mode,omitempty"`
	// Timeout is the per-request timeout in seconds.
	Timeout int `json:"timeout,omitempty"`
	// Concurrency is the max number of in-flight (endpoint, plugin) workers.
	Concurrency int `json:"concurrency,omitempty"`
	// FollowRedirects controls whether the HTTP client follows 3xx responses.
	FollowRedirects bool `json:"follow_redirects,omitempty"`
	// InsecureSkipVerify disables TLS certificate verification when true.
	InsecureSkipVerify bool `json:"insecure_skip_verify,omitempty"`
}

// WithDefaults returns a copy of the config with sensible defaults applied.
func (c ScanConfig) WithDefaults() ScanConfig {
	if c.Timeout <= 0 {
		c.Timeout = 10
	}
	if c.Concurrency <= 0 {
		c.Concurrency = 10
	}
	if c.OutputMode == "" {
		c.OutputMode = OutputModeResults
	}
	return c
}

// AIConfig configures the AI-orchestrated audit mode.
type AIConfig struct {
	BaseURL       string `json:"base_url"`        // OpenAI-compatible endpoint, e.g. https://api.openai.com/v1
	APIKey        string `json:"api_key"`         // API key for the endpoint
	Model         string `json:"model"`           // model name, e.g. gpt-4o
	MaxTurns      int    `json:"max_turns"`       // max tool-calling turns
	SASTReport    string `json:"sast_report"`     // path to optional SAST report file
	Context       string `json:"context"`         // free-form guidance from the user
	TargetBaseURL string `json:"target_base_url"` // live API base URL the scanner hits
	// CustomHeaders are applied to every scan (baseline and the model's scan_api
	// calls), e.g. an Authorization token. The model may add/override per call.
	CustomHeaders map[string]string `json:"custom_headers,omitempty"`
	// InsertPoints restricts where payloads are injected across scans when the
	// caller (or model) does not specify a narrower set. See ScanConfig.InsertPoints.
	InsertPoints []string `json:"insert_point,omitempty"`
	// EnableMCP controls whether the scan tool is exposed to the model (default true).
	// When false the model audits statically without invoking the scanner.
	EnableMCP *bool `json:"enable_mcp,omitempty"`
	// AutoScan runs a baseline scan over the spec×target before the conversation
	// and feeds the findings to the model (default true). Grounds weaker models.
	AutoScan *bool `json:"auto_scan,omitempty"`
	// CollectLogs makes the baseline scan run in full-log mode so its request/
	// response exchanges are retained on the result (for persistence by the
	// caller). Tool-driven scans always collect logs; this only affects the
	// baseline. Default false to keep CLI memory low.
	CollectLogs bool `json:"collect_logs,omitempty"`
}

// WithDefaults returns a copy of the AI config with sensible defaults applied.
func (c AIConfig) WithDefaults() AIConfig {
	if c.MaxTurns <= 0 {
		c.MaxTurns = 300
	}
	if c.BaseURL == "" {
		c.BaseURL = "https://api.openai.com/v1"
	}
	if c.EnableMCP == nil {
		t := true
		c.EnableMCP = &t
	}
	if c.AutoScan == nil {
		t := true
		c.AutoScan = &t
	}
	return c
}

// MCPEnabled reports whether the scan tool should be offered to the model.
func (c AIConfig) MCPEnabled() bool { return c.EnableMCP == nil || *c.EnableMCP }

// AutoScanEnabled reports whether a baseline scan runs before the conversation.
func (c AIConfig) AutoScanEnabled() bool { return c.AutoScan == nil || *c.AutoScan }
