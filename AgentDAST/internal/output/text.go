package output

import (
	"fmt"
	"strings"

	"github.com/fatih/color"

	"agentdast/pkg/types"
)

// TextFormatter renders a human-readable, colorized summary for the terminal.
type TextFormatter struct{}

func (f *TextFormatter) Format(result *types.ScanResult) ([]byte, error) {
	var b strings.Builder

	b.WriteString(color.New(color.Bold).Sprintf("\nDAST Scan Report\n"))
	b.WriteString(fmt.Sprintf("Scan ID:    %s\n", result.ID))
	b.WriteString(fmt.Sprintf("Target:     %s\n", result.ScanConfig.TargetBaseURL))
	b.WriteString(fmt.Sprintf("Spec:       %s\n", result.ScanConfig.SwaggerSource))
	b.WriteString(fmt.Sprintf("Status:     %s\n", result.Status))
	if result.Error != "" {
		b.WriteString(color.RedString("Error:      %s\n", result.Error))
	}

	s := result.Summary
	b.WriteString(fmt.Sprintf("\nEndpoints:  %d    Requests: %d    Findings: %d\n",
		s.TotalEndpoints, s.TotalRequests, s.TotalFindings))
	b.WriteString(fmt.Sprintf("Severity:   %s  %s  %s  %s  %s\n",
		color.New(color.FgRed, color.Bold).Sprintf("critical=%d", s.Critical),
		color.RedString("high=%d", s.High),
		color.YellowString("medium=%d", s.Medium),
		color.CyanString("low=%d", s.Low),
		fmt.Sprintf("info=%d", s.Info)))

	if len(result.Findings) == 0 {
		b.WriteString("\nNo vulnerabilities detected.\n")
		return []byte(b.String()), nil
	}

	b.WriteString("\nFindings:\n")
	for i, fnd := range sortFindings(result.Findings) {
		b.WriteString(fmt.Sprintf("\n%s %s\n", severityTag(fnd.Severity), color.New(color.Bold).Sprint(fnd.Title)))
		b.WriteString(fmt.Sprintf("  %s %s\n", fnd.Method, fnd.Endpoint))
		if fnd.ParamName != "" {
			b.WriteString(fmt.Sprintf("  param: %s (in %s)\n", fnd.ParamName, fnd.ParamIn))
		}
		if fnd.Payload != "" {
			b.WriteString(fmt.Sprintf("  payload: %s\n", fnd.Payload))
		}
		b.WriteString(fmt.Sprintf("  plugin: %s | confidence: %s\n", fnd.Plugin, fnd.Confidence))
		if fnd.Evidence != "" {
			b.WriteString(fmt.Sprintf("  evidence: %s\n", indent(fnd.Evidence)))
		}
		_ = i
	}
	b.WriteString("\n")
	return []byte(b.String()), nil
}

func severityTag(s types.Severity) string {
	switch s {
	case types.SeverityCritical:
		return color.New(color.BgRed, color.FgWhite, color.Bold).Sprint(" CRITICAL ")
	case types.SeverityHigh:
		return color.New(color.FgRed, color.Bold).Sprint("[HIGH]")
	case types.SeverityMedium:
		return color.New(color.FgYellow, color.Bold).Sprint("[MEDIUM]")
	case types.SeverityLow:
		return color.New(color.FgCyan).Sprint("[LOW]")
	default:
		return "[INFO]"
	}
}

func indent(s string) string {
	return strings.ReplaceAll(s, "\n", "\n    ")
}
