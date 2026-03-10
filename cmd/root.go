package cmd

import (
	"github.com/spf13/cobra"
)

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "search-orchestrator",
		Short: "Search Orchestrator Service",
		Long:  "Orchestrates search queries against OpenSearch using QUS-driven search plans.",
	}

	root.AddCommand(newHTTPCmd())

	return root
}
