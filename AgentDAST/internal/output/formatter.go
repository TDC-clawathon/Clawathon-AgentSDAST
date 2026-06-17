// Package output renders scan results in JSON, human-readable text, or markdown.
package output

import (
	"strings"

	"agentdast/pkg/types"
)

// Formatter renders a scan result into bytes.
type Formatter interface {
	Format(result *types.ScanResult) ([]byte, error)
}

// New returns a formatter for the named format (json | text | markdown).
func New(format string) Formatter {
	switch strings.ToLower(format) {
	case "json":
		return &JSONFormatter{}
	case "markdown", "md":
		return &MarkdownFormatter{}
	default:
		return &TextFormatter{}
	}
}

// severityLabel renders a severity with an icon for quick visual scanning.
func severityLabel(s types.Severity) string {
	switch s {
	case types.SeverityCritical:
		return "🔴 Critical"
	case types.SeverityHigh:
		return "🟠 High"
	case types.SeverityMedium:
		return "🟡 Medium"
	case types.SeverityLow:
		return "🔵 Low"
	default:
		return "⚪ Info"
	}
}

// owaspFor maps a plugin to its OWASP API Top 10 / CWE classification.
func owaspFor(plugin string) string {
	switch plugin {
	case "sqli":
		return "Injection — SQL (CWE-89)"
	case "cmdi":
		return "Injection — OS Command (CWE-78)"
	case "ssti":
		return "Injection — Template / SSTI (CWE-1336)"
	case "xss":
		return "Cross-Site Scripting (CWE-79)"
	case "path_traversal":
		return "Path Traversal (CWE-22)"
	case "xxe":
		return "XML External Entity (CWE-611)"
	case "ssrf":
		return "API7:2023 Server-Side Request Forgery (CWE-918)"
	case "idor":
		return "API1:2023 Broken Object Level Authorization (CWE-639)"
	case "auth":
		return "API2:2023 Broken Authentication (CWE-287)"
	case "mass_assignment":
		return "API3:2023 Broken Object Property Level Auth (CWE-915)"
	case "sensitive_data":
		return "API3:2023 Excessive Data Exposure (CWE-200)"
	case "cors":
		return "API8:2023 Security Misconfiguration (CWE-942)"
	case "open_redirect":
		return "Open Redirect (CWE-601)"
	default:
		return plugin
	}
}

// RiskVerdict summarizes overall posture from a severity summary.
func RiskVerdict(s types.ScanSummary) string {
	switch {
	case s.Critical > 0:
		return "CRITICAL — exploitable high-impact issues confirmed; do not expose until fixed"
	case s.High > 0:
		return "HIGH — serious issues confirmed; remediate before release"
	case s.Medium > 0:
		return "MEDIUM — notable weaknesses present; schedule fixes"
	case s.Low > 0:
		return "LOW — minor issues only"
	default:
		return "PASS — no vulnerabilities confirmed by the scanner"
	}
}

// sortFindings returns findings ordered by descending severity, then endpoint.
func sortFindings(findings []types.Finding) []types.Finding {
	out := make([]types.Finding, len(findings))
	copy(out, findings)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0; j-- {
			a, b := out[j-1], out[j]
			if a.Severity.Rank() < b.Severity.Rank() ||
				(a.Severity.Rank() == b.Severity.Rank() && a.Endpoint > b.Endpoint) {
				out[j-1], out[j] = out[j], out[j-1]
			} else {
				break
			}
		}
	}
	return out
}
