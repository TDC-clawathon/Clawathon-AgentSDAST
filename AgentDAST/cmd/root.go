// Package cmd implements the agentdast CLI using cobra.
package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"agentdast/internal/core"
	"agentdast/internal/logging"
	"agentdast/internal/plugins"
)

// Version is the build version, overridable via -ldflags.
var Version = "0.1.0"

var rootCmd = &cobra.Command{
	Use:   "agentdast",
	Short: "AgentDAST — a plugin-based DAST scanner for REST APIs",
	Long: `AgentDAST scans REST APIs (described by OpenAPI/Swagger) for security
vulnerabilities. Each vulnerability class is a plugin. It runs as a CLI, as an
MCP server, or as an AI-orchestrated auditor that verifies SAST findings.`,
	SilenceUsage: true,
	PersistentPreRun: func(cmd *cobra.Command, _ []string) {
		level, _ := cmd.Flags().GetString("log-level")
		format, _ := cmd.Flags().GetString("log-format")
		if level == "" {
			level = os.Getenv("LOG_LEVEL")
		}
		if format == "" {
			format = os.Getenv("LOG_FORMAT")
		}
		logging.Setup(level, format)
	},
}

// Execute registers plugins and runs the root command.
func Execute() error {
	plugins.RegisterGlobal()
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().String("config", "", "path to YAML config file")
	rootCmd.PersistentFlags().String("log-level", "info", "log level: debug | info | warn | error")
	rootCmd.PersistentFlags().String("log-format", "text", "log format: text | json")
	rootCmd.AddCommand(scanCmd, pluginsCmd, reportCmd, mcpCmd, aiCmd)
}

// registry returns the global plugin registry (populated in Execute).
func registry() *core.PluginRegistry { return core.GlobalRegistry }
