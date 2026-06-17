package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	openai "github.com/sashabaranov/go-openai"

	"agentdast/internal/output"
	"agentdast/internal/parser"
	"agentdast/internal/toolexec"
	"agentdast/pkg/types"
)

// Orchestrator drives the AI audit conversation.
type Orchestrator struct {
	cfg    types.AIConfig
	client *openai.Client
	exec   *toolexec.Executor
	// Logger receives structured progress logs. Defaults to the process logger.
	Logger *slog.Logger
}

// NewOrchestrator builds an orchestrator from an AI config and tool executor.
// The executor's base URL is set from the config's target so that scan_api
// calls supplying only a path resolve against the live API.
func NewOrchestrator(cfg types.AIConfig, exec *toolexec.Executor) *Orchestrator {
	cfg = cfg.WithDefaults()
	if exec != nil {
		exec.BaseURL = cfg.TargetBaseURL
		// Apply custom headers (e.g. auth) and the param filter to every scan,
		// including the model's scan_api calls.
		exec.DefaultHeaders = cfg.CustomHeaders
		exec.DefaultInsertPoints = cfg.InsertPoints
	}
	return &Orchestrator{cfg: cfg, client: newClient(cfg), exec: exec, Logger: slog.Default().With("component", "ai")}
}

func (o *Orchestrator) log() *slog.Logger {
	if o.Logger != nil {
		return o.Logger
	}
	return slog.Default()
}

// AuditResult is the output of an AI-orchestrated audit.
type AuditResult struct {
	Report string              // final markdown audit report
	Scans  []*types.ScanResult // scans executed during the audit
	Turns  int                 // model turns used
}

// ConfirmedFindings returns the number of distinct scanner-confirmed findings.
func (a *AuditResult) ConfirmedFindings() int {
	merged, _ := consolidate(a.Scans)
	return len(merged)
}

// Run executes the audit. swaggerSource is the spec used for context; the live
// API base URL, SAST report path, and extra context come from the AI config.
func (o *Orchestrator) Run(ctx context.Context, swaggerSource string) (*AuditResult, error) {
	spec, err := parser.LoadSpec(swaggerSource)
	if err != nil {
		return nil, fmt.Errorf("load spec: %w", err)
	}
	sastReport, err := loadSASTReport(o.cfg.SASTReport)
	if err != nil {
		return nil, err
	}

	audit := &AuditResult{}

	// Baseline auto-scan: ground the model with real findings before it reasons.
	// This makes results robust even with models that cannot tool-call well.
	var baseline *types.ScanResult
	if o.cfg.AutoScanEnabled() && o.cfg.TargetBaseURL != "" && o.exec != nil {
		o.log().Info("baseline scan started", "target", o.cfg.TargetBaseURL)
		baselineCfg := types.ScanConfig{
			SwaggerSource: swaggerSource,
			TargetBaseURL: o.cfg.TargetBaseURL,
		}
		if o.cfg.CollectLogs {
			// Retain request/response exchanges so the caller can persist them.
			baselineCfg.OutputMode = types.OutputModeFull
		}
		baseline, err = o.exec.RunConfig(ctx, baselineCfg)
		if err != nil {
			o.log().Warn("baseline scan failed", "error", err)
		} else {
			audit.Scans = append(audit.Scans, baseline)
			o.log().Info("baseline scan complete",
				"findings", baseline.Summary.TotalFindings,
				"endpoints", baseline.Summary.TotalEndpoints,
				"requests", baseline.Summary.TotalRequests)
		}
	}

	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: systemPrompt(o.cfg.MCPEnabled())},
		{Role: openai.ChatMessageRoleUser, Content: userPrompt(swaggerSource, o.cfg.TargetBaseURL, spec, sastReport, o.cfg.Context, baseline, headerNames(o.cfg.CustomHeaders), o.cfg.InsertPoints)},
	}

	var tools []openai.Tool
	if o.cfg.MCPEnabled() && o.exec != nil {
		tools = toolDefinitions()
	}

	// Verification gate: when a SAST report is supplied, the model must verify at
	// least one test case against the live API before it is allowed to conclude.
	hasSAST := strings.TrimSpace(sastReport) != "" && o.cfg.MCPEnabled()
	verifications := 0
	verifyNudged := false

	for turn := 0; turn < o.cfg.MaxTurns; turn++ {
		audit.Turns = turn + 1
		o.log().Debug("model turn", "turn", turn+1, "max", o.cfg.MaxTurns)

		req := openai.ChatCompletionRequest{Model: o.cfg.Model, Messages: messages}
		if len(tools) > 0 {
			req.Tools = tools
		}
		resp, err := o.client.CreateChatCompletion(ctx, req)
		if err != nil {
			// On the first turn a failure is fatal (bad creds/model/endpoint).
			// Later, fall back to a report built from the scans gathered so far.
			if turn == 0 {
				return nil, fmt.Errorf("chat completion failed (check --base-url/--model/--api-key and tool-calling support): %w", err)
			}
			o.log().Warn("model error mid-audit; finalizing with collected scans", "turn", turn+1, "error", err)
			audit.Report = finalReport(swaggerSource, "_The model stopped responding; the report below is built from completed scans._", audit.Scans)
			return audit, nil
		}
		if len(resp.Choices) == 0 {
			return nil, fmt.Errorf("model returned no choices")
		}
		msg := resp.Choices[0].Message
		messages = append(messages, msg)

		if len(msg.ToolCalls) == 0 {
			// Verification gate: do not let the model conclude before it has
			// actually tested any SAST test case against the live API.
			if hasSAST && !verifyNudged && verifications == 0 {
				verifyNudged = true
				messages = append(messages, openai.ChatCompletionMessage{
					Role: openai.ChatMessageRoleUser,
					Content: "You have not verified any test case against the live API yet. For EACH finding in the SAST " +
						"report, call scan_api (or http_request) on the specific endpoint/parameter to confirm or refute " +
						"it. Only write the final report once every SAST test case has a verdict.",
				})
				continue
			}
			// No tool calls: the model is done. Use its message as the report,
			// or force a synthesis if it returned an empty turn.
			narrative := strings.TrimSpace(msg.Content)
			if narrative == "" {
				narrative = o.synthesize(ctx, messages)
			}
			o.log().Info("audit concluded", "turns", audit.Turns, "verifications", verifications, "scans", len(audit.Scans))
			audit.Report = finalReport(swaggerSource, narrative, audit.Scans)
			return audit, nil
		}

		// Execute each requested tool call and feed results back.
		for _, tc := range msg.ToolCalls {
			o.log().Info("tool call", "tool", tc.Function.Name, "args", truncate(tc.Function.Arguments, 200))
			if tc.Function.Name == fnScanAPI || tc.Function.Name == fnHTTPRequest {
				verifications++
			}
			content := o.dispatch(ctx, tc, audit)
			messages = append(messages, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				ToolCallID: tc.ID,
				Content:    content,
			})
		}
	}

	// Turn cap is a safety backstop, not the normal exit. When reached, force the
	// model to synthesize a real report from everything it has gathered rather
	// than emitting a placeholder.
	o.log().Warn("reached max turns; forcing final report", "max", o.cfg.MaxTurns, "verifications", verifications)
	audit.Report = finalReport(swaggerSource, o.synthesize(ctx, messages), audit.Scans)
	return audit, nil
}

// synthesize asks the model, with tools disabled, to write the final audit
// report from the conversation so far. Used when the model returns an empty turn
// or when the turn cap is reached, so the user always gets a real narrative.
func (o *Orchestrator) synthesize(ctx context.Context, messages []openai.ChatCompletionMessage) string {
	messages = append(messages, openai.ChatCompletionMessage{
		Role: openai.ChatMessageRoleUser,
		Content: "Stop testing now. Using every scan result gathered above, write the FINAL security " +
			"audit report in markdown: an executive summary, per-finding analysis (severity, evidence, " +
			"SAST correlation, remediation), and a clear overall verdict on whether the API is safe to " +
			"expose. Distinguish scanner-confirmed findings from unverified SAST claims. Do not call any tools.",
	})
	resp, err := o.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{Model: o.cfg.Model, Messages: messages})
	if err != nil || len(resp.Choices) == 0 {
		return "_The model did not produce a final narrative; see the consolidated scanner findings below._"
	}
	return strings.TrimSpace(resp.Choices[0].Message.Content)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// headerNames returns the sorted keys of a header map for prompt display.
func headerNames(headers map[string]string) []string {
	if len(headers) == 0 {
		return nil
	}
	names := make([]string, 0, len(headers))
	for k := range headers {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// dispatch runs a single tool call and returns its textual result for the model.
func (o *Orchestrator) dispatch(ctx context.Context, tc openai.ToolCall, audit *AuditResult) string {
	switch tc.Function.Name {
	case fnScanAPI:
		result, err := o.exec.Scan(ctx, json.RawMessage(tc.Function.Arguments))
		if err != nil {
			return "scan error: " + err.Error() + "\nIf the endpoint requires authentication, retry with an Authorization header."
		}
		audit.Scans = append(audit.Scans, result)
		return scanVerdict(result)
	case fnListPlugins:
		var in struct {
			Category string `json:"category"`
		}
		_ = json.Unmarshal([]byte(tc.Function.Arguments), &in)
		data, _ := json.Marshal(o.exec.ListPlugins(in.Category))
		return string(data)
	case fnHTTPRequest:
		log, err := o.exec.HTTPRequest(ctx, json.RawMessage(tc.Function.Arguments))
		if err != nil {
			return "http_request error: " + err.Error()
		}
		return formatHTTPLog(log)
	case fnGetKnowledge:
		var in struct {
			Vuln string `json:"vuln"`
		}
		_ = json.Unmarshal([]byte(tc.Function.Arguments), &in)
		return o.exec.Knowledge(in.Vuln)
	case fnGetScanLogs:
		var in struct {
			ScanID string `json:"scan_id"`
			Max    int    `json:"max"`
		}
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &in); err != nil {
			return "get_scan_logs error: invalid arguments"
		}
		out, err := o.exec.ScanLogs(ctx, in.ScanID, in.Max)
		if err != nil {
			return "get_scan_logs error: " + err.Error()
		}
		return out
	default:
		return "unknown tool: " + tc.Function.Name
	}
}

// formatHTTPLog renders a raw HTTP exchange for the model to inspect.
func formatHTTPLog(log *types.RequestLog) string {
	if log.Error != "" {
		return fmt.Sprintf("request error: %s", log.Error)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "HTTP %d (%dms) %s %s\n", log.StatusCode, log.DurationMS, log.Method, log.URL)
	if ct := log.ResponseHeaders["Content-Type"]; ct != "" {
		fmt.Fprintf(&b, "Content-Type: %s\n", ct)
	}
	if sc := log.ResponseHeaders["Set-Cookie"]; sc != "" {
		fmt.Fprintf(&b, "Set-Cookie: %s\n", truncate(sc, 120))
	}
	fmt.Fprintf(&b, "Body: %s", truncate(oneLine(log.ResponseBody), 800))
	return b.String()
}

// scanVerdict renders a single scan_api result as a crisp verdict the auditor
// can act on, including evidence for confirmed findings.
func scanVerdict(r *types.ScanResult) string {
	if r == nil {
		return "scan error: no result"
	}
	if r.Status == types.ScanStatusFailed {
		return "scan failed: " + r.Error + "\nIf the endpoint requires authentication, retry with an Authorization header."
	}
	if len(r.Findings) == 0 {
		return fmt.Sprintf("scan_id: %s\nRESULT: NOT VULNERABLE — sent %d request(s), no issue detected for the requested plugin(s) on this endpoint. "+
			"A negative result is not proof of safety. If you doubt it, call get_scan_logs(scan_id) to inspect the exact requests/responses, "+
			"then craft your own payload and re-test (scan_api or http_request).", r.ID, r.Summary.TotalRequests)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "scan_id: %s\nRESULT: VULNERABLE — %d finding(s):\n", r.ID, len(r.Findings))
	for _, f := range r.Findings {
		fmt.Fprintf(&b, "- [%s] %s (confidence: %s)\n  %s %s param=%s payload=%q\n  evidence: %s\n",
			f.Severity, f.Title, f.Confidence, f.Method, f.Endpoint, f.ParamName,
			truncate(f.Payload, 80), truncate(oneLine(f.Evidence), 300))
	}
	return b.String()
}

// oneLine collapses whitespace/newlines so evidence fits on a single tool line.
func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// summarizeForModel renders a scan result compactly for the model to reason over.
func summarizeForModel(r *types.ScanResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "scan_id=%s status=%s endpoints=%d requests=%d findings=%d (critical=%d high=%d medium=%d low=%d)\n",
		r.ID, r.Status, r.Summary.TotalEndpoints, r.Summary.TotalRequests, r.Summary.TotalFindings,
		r.Summary.Critical, r.Summary.High, r.Summary.Medium, r.Summary.Low)
	if r.Error != "" {
		fmt.Fprintf(&b, "error: %s\n", r.Error)
	}
	for _, f := range r.Findings {
		fmt.Fprintf(&b, "- [%s] %s | %s %s | param=%s | confidence=%s\n",
			f.Severity, f.Title, f.Method, f.Endpoint, f.ParamName, f.Confidence)
	}
	return b.String()
}

// specSummary produces a compact endpoint listing for the prompt.
func specSummary(spec *parser.ParsedSpec) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Title: %s (version %s)\nBase URL: %s\nEndpoints (%d):\n", spec.Title, spec.Version, spec.BaseURL, len(spec.Endpoints))
	eps := make([]parser.Endpoint, len(spec.Endpoints))
	copy(eps, spec.Endpoints)
	sort.Slice(eps, func(i, j int) bool {
		if eps[i].Path == eps[j].Path {
			return eps[i].Method < eps[j].Method
		}
		return eps[i].Path < eps[j].Path
	})
	for _, ep := range eps {
		var params []string
		for _, p := range ep.Parameters {
			params = append(params, fmt.Sprintf("%s(%s)", p.Name, p.In))
		}
		secured := ""
		if ep.Secured {
			secured = " [secured]"
		}
		fmt.Fprintf(&b, "  %s %s%s params=[%s]\n", ep.Method, ep.Path, secured, strings.Join(params, ", "))
	}
	return b.String()
}

// finalReport assembles the model's narrative followed by ONE consolidated,
// deduplicated findings section drawn from every scan executed. Individual scan
// dumps are intentionally not pasted — the audit is a single result.
func finalReport(source, narrative string, scans []*types.ScanResult) string {
	merged, requests := consolidate(scans)
	summary := types.Summarize(merged)

	var b strings.Builder
	b.WriteString("# AI Security Audit Report\n\n")
	b.WriteString("| | |\n|---|---|\n")
	fmt.Fprintf(&b, "| **Spec** | %s |\n", source)
	fmt.Fprintf(&b, "| **Scans executed** | %d (%d HTTP requests) |\n", len(scans), requests)
	fmt.Fprintf(&b, "| **Scanner-confirmed findings** | %d (🔴 %d / 🟠 %d / 🟡 %d / 🔵 %d) |\n",
		summary.TotalFindings, summary.Critical, summary.High, summary.Medium, summary.Low)
	fmt.Fprintf(&b, "\n**Overall risk: %s**\n\n", output.RiskVerdict(summary))

	b.WriteString("## Auditor Analysis\n\n")
	if n := strings.TrimSpace(narrative); n != "" {
		b.WriteString(n)
	} else {
		b.WriteString("_The model produced no narrative; the consolidated scanner findings are below._")
	}

	b.WriteString("\n\n## Scanner-Confirmed Findings\n")
	b.WriteString("_The following were dynamically confirmed by the scanner (deduplicated). " +
		"Claims in the analysis above that are not listed here are unverified._\n\n")
	if len(merged) == 0 {
		b.WriteString("No vulnerabilities were dynamically confirmed by the scanner.\n")
		return b.String()
	}
	b.WriteString(output.FindingsMarkdown(merged))
	return b.String()
}

// consolidate merges findings from all scans, deduplicating by
// (plugin, method, endpoint, param) and keeping the highest-severity instance.
// It also returns the total number of HTTP requests across all scans.
func consolidate(scans []*types.ScanResult) ([]types.Finding, int) {
	seen := map[string]int{} // key -> index into merged
	var merged []types.Finding
	var requests int
	for _, s := range scans {
		if s == nil {
			continue
		}
		requests += s.Summary.TotalRequests
		for _, f := range s.Findings {
			key := f.Plugin + "|" + f.Method + "|" + f.Endpoint + "|" + f.ParamName
			if idx, ok := seen[key]; ok {
				if f.Severity.Rank() > merged[idx].Severity.Rank() {
					merged[idx] = f
				}
				continue
			}
			seen[key] = len(merged)
			merged = append(merged, f)
		}
	}
	return merged, requests
}
