package plugins

import (
	"strings"

	"agentdast/internal/core"
	"agentdast/pkg/types"
)

// OpenRedirectPlugin detects unvalidated redirects: a parameter whose value
// controls the Location of a 3xx response (or a client-side redirect target).
type OpenRedirectPlugin struct{}

func (p *OpenRedirectPlugin) Name() string             { return "open_redirect" }
func (p *OpenRedirectPlugin) Category() string         { return "config" }
func (p *OpenRedirectPlugin) Severity() types.Severity { return types.SeverityMedium }
func (p *OpenRedirectPlugin) Description() string {
	return "Detects open redirects where a parameter controls the redirect target (Location header or client-side redirect)"
}

// evilHost is the attacker-controlled destination we try to redirect to.
const evilHost = "evil.example.com"

func (p *OpenRedirectPlugin) DefaultPayloads() []string {
	return []string{
		"https://" + evilHost,
		"//" + evilHost,             // scheme-relative
		"https:/" + evilHost,        // missing slash, browsers normalize
		"/\\" + evilHost,            // backslash trick
		"https://" + evilHost + "/", // trailing slash
	}
}

// redirectParamHints identify parameters that commonly drive a redirect.
var redirectParamHints = []string{"redirect", "url", "next", "return", "returnto", "return_url", "redirect_uri", "dest", "destination", "continue", "goto", "to", "target", "callback"}

func looksLikeRedirectParam(name string) bool {
	n := strings.ToLower(name)
	for _, h := range redirectParamHints {
		if n == h || strings.Contains(n, h) {
			return true
		}
	}
	return false
}

func (p *OpenRedirectPlugin) Test(ctx *core.ScanContext) []types.Finding {
	var findings []types.Finding
	for _, param := range ctx.Params() {
		if !looksLikeRedirectParam(param.Name) {
			continue
		}
		for _, payload := range p.DefaultPayloads() {
			if ctx.Ctx.Err() != nil {
				return findings
			}
			log := ctx.Inject(param, payload)
			if log.Error != "" {
				continue
			}
			location := log.ResponseHeaders["Location"]
			redirected := log.StatusCode >= 300 && log.StatusCode < 400 && pointsTo(location, evilHost)
			// Some apps redirect client-side via meta refresh / JS.
			clientSide := strings.Contains(log.ResponseBody, "url="+payload) ||
				strings.Contains(log.ResponseBody, "location.href='"+payload)

			if redirected || clientSide {
				where := "Location header: " + location
				if !redirected {
					where = "client-side redirect in body"
				}
				findings = append(findings, types.Finding{
					Title:       "Open redirect in parameter " + param.Name,
					Endpoint:    ctx.Endpoint.Path,
					Method:      ctx.Endpoint.Method,
					ParamName:   param.Name,
					ParamIn:     param.In,
					Payload:     payload,
					Evidence:    "redirect to attacker-controlled host (" + where + ")\n" + evidence(log),
					Confidence:  types.ConfidenceConfirmed,
					Description: "The endpoint redirects to a destination taken from user input without validation, enabling phishing and OAuth token theft.",
					Remediation: "Redirect only to an allow-list of internal paths/hosts; reject absolute or scheme-relative external URLs.",
					RequestLog:  logForMode(ctx, log),
				})
				break
			}
		}
	}
	return findings
}

// pointsTo reports whether a Location value targets the given host.
func pointsTo(location, host string) bool {
	if location == "" {
		return false
	}
	l := strings.ToLower(location)
	return strings.Contains(l, strings.ToLower(host))
}
