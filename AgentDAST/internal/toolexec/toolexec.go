// Package toolexec bridges tool invocations (from the MCP server and the AI
// orchestrator) to the core scanner, so both share one implementation of
// argument parsing, scanning, and result lookup.
package toolexec

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"agentdast/internal/core"
	"agentdast/internal/knowledge"
	"agentdast/internal/storage"
	"agentdast/pkg/types"
)

// ScanArgs mirrors the JSON schema for the scan_api tool. A scan targets either
// a single endpoint (TargetURL) or a whole spec (SwaggerSource).
type ScanArgs struct {
	TargetURL     string            `json:"target_url"`
	Method        string            `json:"method"`
	Body          string            `json:"body"`
	BodyParams    []string          `json:"body_params"`
	SwaggerSource string            `json:"swagger_source"`
	TargetBaseURL string            `json:"target_base_url"`
	Plugins       []string          `json:"plugins"`
	InsertPoints  []string          `json:"insert_point"`
	Headers       map[string]string `json:"headers"`
	Params        map[string]string `json:"params"`
	OutputMode    string            `json:"output_mode"`
	TimeoutSecs   int               `json:"timeout_secs"`
	Concurrency   int               `json:"concurrency"`
}

// ToConfig converts tool arguments to a ScanConfig, using fallbackBase as the
// base URL when the caller did not provide one.
func (a ScanArgs) ToConfig(fallbackBase string) types.ScanConfig {
	mode := types.OutputMode(a.OutputMode)
	if mode != types.OutputModeFull {
		mode = types.OutputModeResults
	}
	base := a.TargetBaseURL
	if base == "" {
		base = fallbackBase
	}
	return types.ScanConfig{
		TargetURL:     a.TargetURL,
		Method:        a.Method,
		Body:          a.Body,
		BodyParams:    a.BodyParams,
		SwaggerSource: a.SwaggerSource,
		TargetBaseURL: base,
		Plugins:       a.Plugins,
		InsertPoints:  a.InsertPoints,
		CustomHeaders: a.Headers,
		CustomParams:  a.Params,
		OutputMode:    mode,
		Timeout:       a.TimeoutSecs,
		Concurrency:   a.Concurrency,
	}
}

// Executor runs scanner tools against a registry and persists results to a store.
type Executor struct {
	Registry *core.PluginRegistry
	Store    storage.Store
	// BaseURL is the live API base URL used when a scan_api call supplies only a
	// path (set from the AI mode --target flag).
	BaseURL string
	// DefaultHeaders are merged into every scan unless the caller provides the
	// same header (e.g. an Authorization token set via the AI --header flag).
	DefaultHeaders map[string]string
	// DefaultInsertPoints restricts injection when a scan does not specify its own.
	DefaultInsertPoints []string
}

// New returns an Executor. A nil registry defaults to the global registry;
// a nil store defaults to an in-memory store.
func New(registry *core.PluginRegistry, store storage.Store) *Executor {
	if registry == nil {
		registry = core.GlobalRegistry
	}
	if store == nil {
		store = storage.NewInMemoryStore()
	}
	return &Executor{Registry: registry, Store: store}
}

// Scan parses raw JSON args, runs the scan, persists it, and returns the result.
func (e *Executor) Scan(ctx context.Context, rawArgs json.RawMessage) (*types.ScanResult, error) {
	var args ScanArgs
	if len(rawArgs) > 0 {
		if err := json.Unmarshal(rawArgs, &args); err != nil {
			return nil, fmt.Errorf("invalid scan arguments: %w", err)
		}
	}
	if args.TargetURL == "" && args.SwaggerSource == "" {
		return nil, fmt.Errorf("provide target_url (a URL or path to scan) or swagger_source")
	}
	cfg := args.ToConfig(e.BaseURL)
	// Always capture the full request/response log for a tool-driven scan so the
	// caller (the AI) can later inspect exactly what was sent via get_scan_logs.
	cfg.OutputMode = types.OutputModeFull
	return e.RunConfig(ctx, cfg)
}

// Knowledge returns the reference document for a vulnerability name (accepting
// aliases). When none exists it returns a note listing available topics so the
// caller knows to fall back to its own expertise.
func (e *Executor) Knowledge(name string) string {
	if content, ok := knowledge.Get(name); ok {
		return content
	}
	return fmt.Sprintf("No knowledge document for %q. Available topics: %s.\nProceed using your own security expertise.",
		name, strings.Join(knowledge.List(), ", "))
}

// ScanLogs returns the captured request/response exchanges for a completed scan,
// for the AI to inspect when it wants to judge a result itself. Bodies are
// truncated and the number of exchanges is capped to protect the context window.
func (e *Executor) ScanLogs(ctx context.Context, scanID string, max int) (string, error) {
	r, err := e.Store.GetResult(ctx, scanID)
	if err != nil {
		return "", err
	}
	logs := r.RequestLogs
	if len(logs) == 0 {
		return "No request logs were captured for scan " + scanID + ".", nil
	}
	if max <= 0 || max > 30 {
		max = 30
	}
	shown := logs
	truncatedNote := ""
	if len(logs) > max {
		shown = logs[:max]
		truncatedNote = fmt.Sprintf("\n... %d more exchange(s) not shown; narrow the scan (one plugin / one insert_point) for focused logs.\n", len(logs)-max)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Scan %s — %d request(s):\n", scanID, len(logs))
	for i, l := range shown {
		fmt.Fprintf(&b, "\n[%d] %s %s\n", i+1, l.Method, l.URL)
		if l.RequestBody != "" {
			fmt.Fprintf(&b, "    request-body: %s\n", clip(l.RequestBody, 300))
		}
		if l.Error != "" {
			fmt.Fprintf(&b, "    error: %s\n", l.Error)
			continue
		}
		fmt.Fprintf(&b, "    -> %d (%dms) %s\n", l.StatusCode, l.DurationMS, l.ResponseHeaders["Content-Type"])
		fmt.Fprintf(&b, "    response-body: %s\n", clip(l.ResponseBody, 600))
	}
	b.WriteString(truncatedNote)
	return b.String(), nil
}

func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// RunConfig executes a scan from an already-built config and persists the result.
// Executor defaults (headers, test params) are merged in here so they apply to
// both the baseline scan and the model's scan_api calls, without overriding any
// value the caller explicitly set.
func (e *Executor) RunConfig(ctx context.Context, cfg types.ScanConfig) (*types.ScanResult, error) {
	cfg = e.applyDefaults(cfg)
	scanner := core.NewScanner(e.Registry)
	result, err := scanner.Run(ctx, cfg)
	if result != nil {
		_ = e.Store.SaveResult(ctx, result)
	}
	return result, err
}

// applyDefaults merges the executor's default headers and test params into cfg
// without overriding values the caller already provided.
func (e *Executor) applyDefaults(cfg types.ScanConfig) types.ScanConfig {
	if len(e.DefaultHeaders) > 0 {
		merged := make(map[string]string, len(e.DefaultHeaders)+len(cfg.CustomHeaders))
		for k, v := range e.DefaultHeaders {
			merged[k] = v
		}
		for k, v := range cfg.CustomHeaders { // caller-supplied headers win
			merged[k] = v
		}
		cfg.CustomHeaders = merged
	}
	if len(cfg.InsertPoints) == 0 {
		cfg.InsertPoints = e.DefaultInsertPoints
	}
	return cfg
}

// PluginInfo is a serializable description of a registered plugin.
type PluginInfo struct {
	Name        string `json:"name"`
	Category    string `json:"category"`
	Severity    string `json:"severity"`
	Description string `json:"description"`
}

// ListPlugins returns plugin descriptions, optionally filtered by category.
func (e *Executor) ListPlugins(category string) []PluginInfo {
	var out []PluginInfo
	for _, p := range e.Registry.List() {
		if category != "" && p.Category() != category {
			continue
		}
		out = append(out, PluginInfo{
			Name:        p.Name(),
			Category:    p.Category(),
			Severity:    string(p.Severity()),
			Description: p.Description(),
		})
	}
	return out
}

// GetResult fetches a stored scan result, optionally stripping full logs.
func (e *Executor) GetResult(ctx context.Context, scanID string, includeLogs bool) (*types.ScanResult, error) {
	r, err := e.Store.GetResult(ctx, scanID)
	if err != nil {
		return nil, err
	}
	if !includeLogs {
		clone := *r
		clone.RequestLogs = nil
		return &clone, nil
	}
	return r, nil
}

// HTTPArgs is a fully custom HTTP request the caller wants to send verbatim.
type HTTPArgs struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
}

// HTTPRequest sends an arbitrary HTTP request and returns the raw exchange. The
// URL may be a full URL or a path resolved against the executor BaseURL. Default
// headers (e.g. auth) are applied unless the caller overrides them. This lets an
// AI auditor craft any test case the built-in plugins do not cover.
func (e *Executor) HTTPRequest(ctx context.Context, raw json.RawMessage) (*types.RequestLog, error) {
	var args HTTPArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid http_request arguments: %w", err)
	}
	if args.URL == "" {
		return nil, fmt.Errorf("url is required")
	}
	method := strings.ToUpper(strings.TrimSpace(args.Method))
	if method == "" {
		method = http.MethodGet
	}

	target := args.URL
	if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") {
		if e.BaseURL == "" {
			return nil, fmt.Errorf("url %q is a path but no base URL is configured", target)
		}
		target = strings.TrimRight(e.BaseURL, "/") + "/" + strings.TrimLeft(target, "/")
	}

	var bodyReader *strings.Reader
	if args.Body != "" {
		bodyReader = strings.NewReader(args.Body)
	} else {
		bodyReader = strings.NewReader("")
	}
	req, err := http.NewRequestWithContext(ctx, method, target, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	for k, v := range e.DefaultHeaders {
		req.Header.Set(k, v)
	}
	for k, v := range args.Headers { // caller headers win over defaults
		req.Header.Set(k, v)
	}

	executor := core.NewRequestExecutor(types.ScanConfig{}.WithDefaults())
	log := executor.Send(req)
	return &log, nil
}
