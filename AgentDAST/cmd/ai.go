package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"agentdast/config"
	"agentdast/internal/ai"
	"agentdast/internal/storage"
	"agentdast/internal/toolexec"
	"agentdast/pkg/types"
)

var aiCmd = &cobra.Command{
	Use:   "ai",
	Short: "Run an AI-orchestrated security audit that verifies SAST findings via scanning",
	RunE:  runAI,
}

func init() {
	f := aiCmd.Flags()
	f.String("swagger", "", "path or URL to an OpenAPI/Swagger spec — context for the model (required)")
	f.String("target", "", "live API base URL the scanner hits (required to scan)")
	f.String("sast-report", "", "path to a SAST report to verify")
	f.String("context", "", "additional guidance for the auditor")
	f.String("model", "", "model name (or OPENAI_MODEL env)")
	f.String("api-key", "", "API key (or OPENAI_API_KEY env)")
	f.String("base-url", "", "OpenAI-compatible base URL (or OPENAI_BASE_URL env)")
	f.StringArrayP("header", "H", nil, "custom header 'Name: Value' applied to every scan, e.g. auth (repeatable)")
	f.StringSlice("insert-point", nil, "restrict injection to these points across scans: name or 'location:name' (default: all)")
	f.Int("max-turns", 300, "maximum tool-calling turns (safety backstop; the model normally stops when done)")
	f.Bool("enable-mcp", true, "expose the scan tool to the model so it can scan (default true)")
	f.Bool("auto-scan", true, "run a baseline scan over the spec×target before the AI reasons (default true)")
	f.String("out", "", "write the audit report to this file (default: stdout)")
	_ = aiCmd.MarkFlagRequired("swagger")
}

func runAI(cmd *cobra.Command, _ []string) error {
	f := cmd.Flags()
	swagger, _ := f.GetString("swagger")
	target, _ := f.GetString("target")
	sastReport, _ := f.GetString("sast-report")
	guidance, _ := f.GetString("context")
	model, _ := f.GetString("model")
	apiKey, _ := f.GetString("api-key")
	baseURL, _ := f.GetString("base-url")
	headerList, _ := f.GetStringArray("header")
	insertPoints, _ := f.GetStringSlice("insert-point")
	maxTurns, _ := f.GetInt("max-turns")
	enableMCP, _ := f.GetBool("enable-mcp")
	autoScan, _ := f.GetBool("auto-scan")
	outFile, _ := f.GetString("out")
	cfgPath, _ := f.GetString("config")

	// Layer config file + env behind explicit flags.
	fileCfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}
	aiCfg := types.AIConfig{
		BaseURL:       pick(baseURL, fileCfg.AI.BaseURL),
		APIKey:        pick(apiKey, fileCfg.AI.APIKey),
		Model:         pick(model, fileCfg.AI.Model),
		MaxTurns:      maxTurns,
		SASTReport:    sastReport,
		Context:       guidance,
		TargetBaseURL: pick(target, fileCfg.AI.TargetBaseURL),
		CustomHeaders: parseHeaders(headerList),
		InsertPoints:  insertPoints,
		EnableMCP:     &enableMCP,
		AutoScan:      &autoScan,
	}
	if aiCfg.APIKey == "" {
		return fmt.Errorf("no API key: pass --api-key or set OPENAI_API_KEY")
	}
	if aiCfg.Model == "" {
		return fmt.Errorf("no model: pass --model or set OPENAI_MODEL")
	}
	if aiCfg.TargetBaseURL == "" {
		fmt.Fprintln(os.Stderr, "warning: no --target set; the scanner cannot reach the live API. The model will need full URLs and auto-scan is skipped.")
	}

	store, err := storage.FromEnv()
	if err != nil {
		return err
	}
	defer store.Close()

	exec := toolexec.New(registry(), store)
	orch := ai.NewOrchestrator(aiCfg, exec)

	audit, err := orch.Run(context.Background(), swagger)
	if err != nil {
		return err
	}

	if outFile != "" {
		if err := os.WriteFile(outFile, []byte(audit.Report), 0o644); err != nil {
			return err
		}
		fmt.Printf("wrote %s (%d scans, %d turns)\n", outFile, len(audit.Scans), audit.Turns)
		return nil
	}
	fmt.Println(audit.Report)
	return nil
}

// pick returns the first non-empty string.
func pick(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
