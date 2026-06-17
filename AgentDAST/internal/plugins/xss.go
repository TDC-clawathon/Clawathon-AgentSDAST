package plugins

import (
	"strings"

	"agentdast/internal/core"
	"agentdast/pkg/types"
)

// XSSPlugin detects reflected XSS by injecting a payload with a unique marker
// and confirming it is reflected UNESCAPED (angle brackets intact) in an HTML
// response. Requiring an HTML content type and unescaped brackets avoids the
// common false positive of input echoed into a JSON body (which is not XSS).
type XSSPlugin struct{}

func (p *XSSPlugin) Name() string             { return "xss" }
func (p *XSSPlugin) Category() string         { return "injection" }
func (p *XSSPlugin) Severity() types.Severity { return types.SeverityHigh }
func (p *XSSPlugin) Description() string {
	return "Detects reflected XSS by confirming a unique payload is reflected unescaped in an HTML response"
}

// The marker is unlikely to occur naturally, so a match means our payload was reflected.
const xssMarker = "xq9z1"

func (p *XSSPlugin) DefaultPayloads() []string {
	return []string{
		// basic script context
		`<script>alert(` + xssMarker + `)</script>`,
		// attribute breakout → event handler
		`"><svg/onload=alert(` + xssMarker + `)>`,
		`'><img src=x onerror=alert(` + xssMarker + `)>`,
		// break out of <title>/<textarea>/<style> raw-text contexts
		`</title><svg/onload=alert(` + xssMarker + `)>`,
		`</textarea><svg/onload=alert(` + xssMarker + `)>`,
		// event handler that fires without interaction
		`"><details/open/ontoggle=alert(` + xssMarker + `)>`,
		// polyglot covering several contexts at once
		`'"></script><svg/onload=alert(` + xssMarker + `)>`,
	}
}

func (p *XSSPlugin) Test(ctx *core.ScanContext) []types.Finding {
	var findings []types.Finding
	for _, param := range ctx.Params() {
		for _, payload := range p.DefaultPayloads() {
			if ctx.Ctx.Err() != nil {
				return findings
			}
			log := ctx.Inject(param, payload)
			if log.Error != "" {
				continue
			}
			ctype := strings.ToLower(log.ResponseHeaders["Content-Type"])
			isHTML := strings.Contains(ctype, "html") || ctype == ""

			// Reflected verbatim (angle brackets NOT entity-encoded) → executable.
			reflectedRaw := strings.Contains(log.ResponseBody, payload)
			// Encoded reflection means the app is escaping correctly — not a finding.
			encoded := strings.Contains(log.ResponseBody, strings.ReplaceAll(payload, "<", "&lt;"))

			if reflectedRaw && !encoded && isHTML {
				findings = append(findings, types.Finding{
					Title:       "Reflected XSS in parameter " + param.Name,
					Endpoint:    ctx.Endpoint.Path,
					Method:      ctx.Endpoint.Method,
					ParamName:   param.Name,
					ParamIn:     param.In,
					Payload:     payload,
					Evidence:    "payload reflected unescaped in an HTML response (content-type: " + ctype + ")\n" + evidence(log),
					Confidence:  types.ConfidenceConfirmed,
					Description: "User input is reflected into an HTML response without output encoding, allowing script injection.",
					Remediation: "Apply context-aware output encoding; set a restrictive Content-Security-Policy and correct Content-Type.",
					RequestLog:  logForMode(ctx, log),
				})
				break
			}
		}
	}
	return findings
}
