package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var pluginsCmd = &cobra.Command{
	Use:   "plugins",
	Short: "List available vulnerability plugins",
	RunE:  runPlugins,
}

func init() {
	pluginsCmd.Flags().String("category", "", "filter by category: injection | auth | exposure | config")
}

func runPlugins(cmd *cobra.Command, _ []string) error {
	category, _ := cmd.Flags().GetString("category")

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tCATEGORY\tSEVERITY\tDESCRIPTION")
	for _, p := range registry().List() {
		if category != "" && p.Category() != category {
			continue
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", p.Name(), p.Category(), p.Severity(), p.Description())
	}
	return w.Flush()
}
