package plugins

import (
	"strings"

	"agentdast/internal/core"
	"agentdast/pkg/types"
)

// CORSPlugin detects permissive CORS configurations by sending a crafted Origin.
type CORSPlugin struct{}

func (p *CORSPlugin) Name() string             { return "cors" }
func (p *CORSPlugin) Category() string         { return "config" }
func (p *CORSPlugin) Severity() types.Severity { return types.SeverityMedium }
func (p *CORSPlugin) Description() string {
	return "Detects CORS misconfiguration: origin reflection or wildcard combined with credentials"
}

func (p *CORSPlugin) DefaultPayloads() []string {
	return []string{"https://evil.example.com"}
}

func (p *CORSPlugin) Test(ctx *core.ScanContext) []types.Finding {
	if ctx.Ctx.Err() != nil {
		return nil
	}
	evilOrigin := p.DefaultPayloads()[0]

	cfg := ctx.Config
	cfg.CustomHeaders = cloneHeaders(ctx.Config.CustomHeaders)
	cfg.CustomHeaders["Origin"] = evilOrigin

	log := ctx.Executor.Baseline(ctx.Ctx, ctx.Endpoint, ctx.BaseURL, cfg)
	if log.Error != "" {
		return nil
	}

	acao := log.ResponseHeaders["Access-Control-Allow-Origin"]
	acac := strings.ToLower(log.ResponseHeaders["Access-Control-Allow-Credentials"])
	if acao == "" {
		return nil
	}

	reflected := acao == evilOrigin
	wildcard := acao == "*"
	credentialed := acac == "true"

	switch {
	case reflected && credentialed:
		return []types.Finding{p.finding(ctx, log, types.SeverityHigh, types.ConfidenceConfirmed,
			"reflects arbitrary Origin with Access-Control-Allow-Credentials: true",
			"The server reflects any Origin and allows credentials, enabling cross-origin theft of authenticated data.")}
	case reflected:
		return []types.Finding{p.finding(ctx, log, types.SeverityMedium, types.ConfidenceProbable,
			"reflects arbitrary Origin in Access-Control-Allow-Origin",
			"The server reflects the request Origin, weakening the same-origin policy for this endpoint.")}
	case wildcard && credentialed:
		return []types.Finding{p.finding(ctx, log, types.SeverityMedium, types.ConfidenceProbable,
			"wildcard Access-Control-Allow-Origin with credentials",
			"A wildcard ACAO together with credentials is unsafe (and rejected by browsers, indicating misconfiguration).")}
	}
	return nil
}

func (p *CORSPlugin) finding(ctx *core.ScanContext, log types.RequestLog, sev types.Severity, conf, ev, desc string) types.Finding {
	return types.Finding{
		Severity:    sev,
		Title:       "CORS misconfiguration",
		Endpoint:    ctx.Endpoint.Path,
		Method:      ctx.Endpoint.Method,
		Evidence:    ev + "\n" + evidence(log),
		Confidence:  conf,
		Description: desc,
		Remediation: "Validate Origin against a strict allow-list; never reflect arbitrary origins or combine wildcard with credentials.",
		RequestLog:  logForMode(ctx, log),
	}
}

func cloneHeaders(in map[string]string) map[string]string {
	out := make(map[string]string, len(in)+1)
	for k, v := range in {
		out[k] = v
	}
	return out
}
