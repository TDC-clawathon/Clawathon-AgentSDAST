package output

import (
	"encoding/json"

	"agentdast/pkg/types"
)

// JSONFormatter renders the full scan result as indented JSON.
type JSONFormatter struct{}

func (f *JSONFormatter) Format(result *types.ScanResult) ([]byte, error) {
	return json.MarshalIndent(result, "", "  ")
}
