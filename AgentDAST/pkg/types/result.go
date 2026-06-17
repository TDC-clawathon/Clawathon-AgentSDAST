package types

import "time"

// ScanStatus is the lifecycle state of a scan.
type ScanStatus string

const (
	ScanStatusPending   ScanStatus = "pending"
	ScanStatusRunning   ScanStatus = "running"
	ScanStatusCompleted ScanStatus = "completed"
	ScanStatusFailed    ScanStatus = "failed"
)

// ScanResult is the full output of a scan run.
type ScanResult struct {
	ID          string       `json:"id"`
	ScanConfig  ScanConfig   `json:"scan_config"`
	Status      ScanStatus   `json:"status"`
	StartedAt   time.Time    `json:"started_at"`
	CompletedAt *time.Time   `json:"completed_at,omitempty"`
	Findings    []Finding    `json:"findings"`
	RequestLogs []RequestLog `json:"request_logs,omitempty"` // populated only in full output mode
	Error       string       `json:"error,omitempty"`
	Summary     ScanSummary  `json:"summary"`
}

// ScanSummary aggregates counts across a scan.
type ScanSummary struct {
	TotalRequests  int `json:"total_requests"`
	TotalEndpoints int `json:"total_endpoints"`
	TotalFindings  int `json:"total_findings"`
	Critical       int `json:"critical"`
	High           int `json:"high"`
	Medium         int `json:"medium"`
	Low            int `json:"low"`
	Info           int `json:"info"`
}

// Summarize computes a ScanSummary from a slice of findings.
func Summarize(findings []Finding) ScanSummary {
	s := ScanSummary{TotalFindings: len(findings)}
	for _, f := range findings {
		switch f.Severity {
		case SeverityCritical:
			s.Critical++
		case SeverityHigh:
			s.High++
		case SeverityMedium:
			s.Medium++
		case SeverityLow:
			s.Low++
		case SeverityInfo:
			s.Info++
		}
	}
	return s
}
