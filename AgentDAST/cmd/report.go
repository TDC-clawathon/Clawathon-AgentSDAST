package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"agentdast/internal/output"
	"agentdast/internal/storage"
	"agentdast/pkg/types"
)

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Render a report from a saved scan result (file or stored scan ID)",
	RunE:  runReport,
}

func init() {
	f := reportCmd.Flags()
	f.String("file", "", "path to a saved scan result JSON file")
	f.String("scan-id", "", "ID of a scan stored in the configured backend (MySQL)")
	f.String("format", "markdown", "output format: markdown | json | text")
	f.String("out", "", "write to this file (default: stdout)")
}

func runReport(cmd *cobra.Command, _ []string) error {
	f := cmd.Flags()
	file, _ := f.GetString("file")
	scanID, _ := f.GetString("scan-id")
	format, _ := f.GetString("format")
	outFile, _ := f.GetString("out")

	var result *types.ScanResult

	switch {
	case file != "":
		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		result = &types.ScanResult{}
		if err := json.Unmarshal(data, result); err != nil {
			return fmt.Errorf("parse result file: %w", err)
		}
	case scanID != "":
		store, err := storage.FromEnv()
		if err != nil {
			return err
		}
		defer store.Close()
		result, err = store.GetResult(context.Background(), scanID)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("provide either --file or --scan-id")
	}

	data, err := output.New(format).Format(result)
	if err != nil {
		return err
	}
	if outFile != "" {
		if err := os.WriteFile(outFile, data, 0o644); err != nil {
			return err
		}
		fmt.Printf("wrote %s\n", outFile)
		return nil
	}
	fmt.Println(string(data))
	return nil
}
