package plugins

import (
	"regexp"

	"agentdast/internal/core"
	"agentdast/pkg/types"
)

// SensitiveDataPlugin scans responses for PII and secret material leakage.
type SensitiveDataPlugin struct{}

func (p *SensitiveDataPlugin) Name() string             { return "sensitive_data" }
func (p *SensitiveDataPlugin) Category() string         { return "exposure" }
func (p *SensitiveDataPlugin) Severity() types.Severity { return types.SeverityMedium }
func (p *SensitiveDataPlugin) Description() string {
	return "Scans responses for exposed PII (emails, credit cards, SSNs) and secrets (API keys, private keys, JWTs)"
}

func (p *SensitiveDataPlugin) DefaultPayloads() []string { return nil }

type secretPattern struct {
	label    string
	severity types.Severity
	re       *regexp.Regexp
}

// Patterns are limited to high-signal secrets. Generic PII like email addresses
// is intentionally excluded — it appears legitimately in many API responses and
// produced overwhelming false positives.
var sensitivePatterns = []secretPattern{
	{"private key", types.SeverityHigh, regexp.MustCompile(`-----BEGIN (?:RSA |EC |OPENSSH |DSA )?PRIVATE KEY-----`)},
	{"AWS access key", types.SeverityHigh, regexp.MustCompile(`AKIA[0-9A-Z]{16}`)},
	{"AWS secret key", types.SeverityHigh, regexp.MustCompile(`(?i)aws_secret_access_key["'\s:=]+[0-9a-zA-Z/+]{40}`)},
	{"Google API key", types.SeverityHigh, regexp.MustCompile(`AIza[0-9A-Za-z\-_]{35}`)},
	{"Slack token", types.SeverityHigh, regexp.MustCompile(`xox[baprs]-[0-9A-Za-z\-]{10,}`)},
	{"GitHub token", types.SeverityHigh, regexp.MustCompile(`gh[pousr]_[0-9A-Za-z]{36}`)},
	{"credit card number", types.SeverityHigh, regexp.MustCompile(`\b(?:4[0-9]{12}(?:[0-9]{3})?|5[1-5][0-9]{14}|3[47][0-9]{13})\b`)},
	{"US SSN", types.SeverityHigh, regexp.MustCompile(`\b[0-9]{3}-[0-9]{2}-[0-9]{4}\b`)},
	{"password in body", types.SeverityMedium, regexp.MustCompile(`(?i)"(?:password|passwd|pwd|secret)"\s*:\s*"[^"]{3,}"`)},
}

func (p *SensitiveDataPlugin) Test(ctx *core.ScanContext) []types.Finding {
	if ctx.Ctx.Err() != nil {
		return nil
	}
	log := ctx.Baseline()
	if log.Error != "" || log.ResponseBody == "" {
		return nil
	}

	var findings []types.Finding
	for _, pat := range sensitivePatterns {
		if m := pat.re.FindString(log.ResponseBody); m != "" {
			findings = append(findings, types.Finding{
				Severity:    pat.severity,
				Title:       "Sensitive data exposure: " + pat.label,
				Endpoint:    ctx.Endpoint.Path,
				Method:      ctx.Endpoint.Method,
				Evidence:    "matched " + pat.label + ": " + truncate(m, 64),
				Confidence:  types.ConfidenceProbable,
				Description: "The response body contained data matching a sensitive-data pattern (" + pat.label + ").",
				Remediation: "Remove sensitive fields from API responses; apply field-level filtering and data minimization.",
				RequestLog:  logForMode(ctx, log),
			})
		}
	}
	return findings
}
