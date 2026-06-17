package cmd

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"agentdast/internal/knowledge"
	"agentdast/internal/server"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run AgentDAST as an HTTP service (scan API backed by MySQL + MinIO)",
	Long: `Starts the AgentDAST HTTP service exposing:
  GET  /health              service + DB health
  GET  /api/dast/health     same as /health (Manager client)
  POST /api/dast/scan       start a scan (project_id, base_url required; prompt optional)
                         -> {id}. The swagger (required) and SAST report (optional) are
                         read from MinIO at <project>/sast/openapi.yaml and
                         <project>/sast/report.md; a missing swagger is rejected (400).
  GET  /api/dast/status  poll a scan (id) -> status, result_path or error
  POST /api/dast/cancel  cancel a running scan (id) -> status cancel

The report is written to <project>/dast/report.md and request logs to
<project>/dast/logs/. Configuration is read from the environment (MYSQL_*, MINIO_*,
OPENAI_*). See the Dockerfile / docker-compose for the full list.`,
	RunE: runServe,
}

func init() {
	serveCmd.Flags().String("addr", "", "listen address (overrides PORT env, e.g. :8080)")
	rootCmd.AddCommand(serveCmd)
}

func runServe(cmd *cobra.Command, _ []string) error {
	cfg := server.LoadConfig()
	if addr, _ := cmd.Flags().GetString("addr"); addr != "" {
		cfg.Addr = addr
	}

	dir := os.Getenv("SKILLS_DAST_DIR")
	if err := knowledge.Init(dir); err != nil {
		return err
	}
	if dir == "" {
		dir = knowledge.ResolveDastSkillsDir()
	}
	slog.Info("knowledge loaded from disk", "dir", dir)

	srv, err := server.New(cfg)
	if err != nil {
		return err
	}
	defer srv.Close()

	return srv.ListenAndServe()
}
