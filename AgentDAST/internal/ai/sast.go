package ai

import (
	"fmt"
	"os"
)

// loadSASTReport reads a SAST report file and returns its contents (truncated
// to keep the prompt manageable). The report is passed to the model as-is; the
// model interprets whatever format (Semgrep, Bandit, Checkmarx JSON, SARIF…).
func loadSASTReport(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read SAST report: %w", err)
	}
	const maxBytes = 60_000
	if len(data) > maxBytes {
		return string(data[:maxBytes]) + "\n...[truncated]...", nil
	}
	return string(data), nil
}
