package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"agentdast/internal/mcp"
	"agentdast/internal/storage"
	"agentdast/internal/toolexec"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Run as an MCP server (stdio by default, or --tcp)",
	RunE:  runMCP,
}

func init() {
	mcpCmd.Flags().String("tcp", "", "listen on this TCP address (e.g. :8765) instead of stdio")
}

func runMCP(cmd *cobra.Command, _ []string) error {
	tcp, _ := cmd.Flags().GetString("tcp")

	store, err := storage.FromEnv()
	if err != nil {
		return err
	}
	defer store.Close()

	exec := toolexec.New(registry(), store)
	server := mcp.NewServer(exec, Version)
	ctx := context.Background()

	if tcp != "" {
		fmt.Fprintf(os.Stderr, "agentdast MCP server listening on %s\n", tcp)
		return server.ServeTCP(ctx, tcp)
	}
	return server.ServeStdio(ctx, os.Stdin, os.Stdout)
}
