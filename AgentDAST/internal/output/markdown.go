package output

import (
	"fmt"
	"strings"

	"agentdast/pkg/types"
)

// MarkdownFormatter renders a scan result as a markdown report.
type MarkdownFormatter struct{}

func (f *MarkdownFormatter) Format(result *types.ScanResult) ([]byte, error) {
	return []byte(Markdown(result)), nil
}

// Markdown produces a full markdown report for a scan result: metadata, an
// executive summary with an overall risk verdict, then the findings.
func Markdown(result *types.ScanResult) string {
	s := result.Summary
	var b strings.Builder

	b.WriteString("# DAST Security Scan Report\n\n")
	b.WriteString("| | |\n|---|---|\n")
	b.WriteString(fmt.Sprintf("| **Scan ID** | `%s` |\n", result.ID))
	if result.ScanConfig.TargetBaseURL != "" {
		b.WriteString(fmt.Sprintf("| **Target** | %s |\n", result.ScanConfig.TargetBaseURL))
	}
	if result.ScanConfig.SwaggerSource != "" {
		b.WriteString(fmt.Sprintf("| **Spec** | %s |\n", result.ScanConfig.SwaggerSource))
	}
	b.WriteString(fmt.Sprintf("| **Status** | %s |\n", result.Status))
	if !result.StartedAt.IsZero() {
		b.WriteString(fmt.Sprintf("| **Started** | %s |\n", result.StartedAt.Format("2006-01-02 15:04:05 MST")))
	}

	b.WriteString("\n## Executive Summary\n\n")
	b.WriteString(fmt.Sprintf("**Overall risk: %s**\n\n", RiskVerdict(s)))
	b.WriteString("| Severity | Count |\n|---|---:|\n")
	b.WriteString(fmt.Sprintf("| 🔴 Critical | %d |\n", s.Critical))
	b.WriteString(fmt.Sprintf("| 🟠 High | %d |\n", s.High))
	b.WriteString(fmt.Sprintf("| 🟡 Medium | %d |\n", s.Medium))
	b.WriteString(fmt.Sprintf("| 🔵 Low | %d |\n", s.Low))
	b.WriteString(fmt.Sprintf("| **Total** | **%d** |\n", s.TotalFindings))
	b.WriteString(fmt.Sprintf("\n_Scanned %d endpoint(s) with %d request(s)._\n", s.TotalEndpoints, s.TotalRequests))

	if result.Error != "" {
		b.WriteString(fmt.Sprintf("\n> ⚠️ **Scan error:** %s\n", result.Error))
	}

	b.WriteString("\n")
	b.WriteString(FindingsMarkdown(result.Findings))
	return b.String()
}

// FindingsMarkdown renders findings as a scannable overview table followed by
// detailed entries grouped by severity. Embedded by both the scan report and the
// AI audit report so the format is consistent everywhere.
func FindingsMarkdown(findings []types.Finding) string {
	var b strings.Builder
	if len(findings) == 0 {
		b.WriteString("## Findings\n\n✅ No vulnerabilities detected.\n")
		return b.String()
	}

	sorted := sortFindings(findings)

	// Overview table — quick triage.
	b.WriteString("## Findings Overview\n\n")
	b.WriteString("| # | Severity | Finding | Endpoint | Confidence |\n")
	b.WriteString("|---|---|---|---|---|\n")
	for i, f := range sorted {
		b.WriteString(fmt.Sprintf("| %d | %s | %s | `%s %s` | %s |\n",
			i+1, severityLabel(f.Severity), escapePipes(f.Title), f.Method, f.Endpoint, f.Confidence))
	}

	// Detailed findings grouped by severity (highest first).
	b.WriteString("\n## Detailed Findings\n")
	order := []types.Severity{types.SeverityCritical, types.SeverityHigh, types.SeverityMedium, types.SeverityLow, types.SeverityInfo}
	n := 0
	for _, sev := range order {
		group := filterBySeverity(sorted, sev)
		if len(group) == 0 {
			continue
		}
		fmt.Fprintf(&b, "\n### %s\n", severityLabel(sev))
		for _, f := range group {
			n++
			writeFinding(&b, n, f)
		}
	}
	return b.String()
}

func writeFinding(b *strings.Builder, n int, f types.Finding) {
	fmt.Fprintf(b, "\n#### %d. %s\n\n", n, f.Title)
	fmt.Fprintf(b, "- **Location:** `%s %s`\n", f.Method, f.Endpoint)
	if f.ParamName != "" {
		in := f.ParamIn
		if in == "" {
			in = "query"
		}
		fmt.Fprintf(b, "- **Insert point:** `%s:%s`\n", in, f.ParamName)
	}
	fmt.Fprintf(b, "- **Type:** %s\n", owaspFor(f.Plugin))
	fmt.Fprintf(b, "- **Confidence:** %s\n", f.Confidence)
	if f.Payload != "" {
		fmt.Fprintf(b, "- **Payload:** `%s`\n", oneLineMD(f.Payload))
	}
	if f.Description != "" {
		fmt.Fprintf(b, "\n**Impact:** %s\n", f.Description)
	}
	if f.Evidence != "" {
		fmt.Fprintf(b, "\n**Evidence:**\n```\n%s\n```\n", strings.TrimRight(f.Evidence, "\n"))
	}
	if f.Remediation != "" {
		fmt.Fprintf(b, "\n**Remediation:** %s\n", f.Remediation)
	}
}

func filterBySeverity(findings []types.Finding, sev types.Severity) []types.Finding {
	var out []types.Finding
	for _, f := range findings {
		if f.Severity == sev {
			out = append(out, f)
		}
	}
	return out
}

// escapePipes keeps a value from breaking a markdown table cell.
func escapePipes(s string) string {
	return strings.ReplaceAll(oneLineMD(s), "|", "\\|")
}

// oneLineMD collapses newlines so a value fits on a single markdown line.
func oneLineMD(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
