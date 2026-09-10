package main

import (
	"github.com/spf13/cobra"

	"github.com/kaktooslabs/kaktoos/internal/mcp"
)

var mcpCmd = &cobra.Command{
	Use:           "mcp",
	Short:         "Run an MCP server exposing Kaktoos to an AI coding agent over stdio",
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return mcp.Serve(cmd.Context())
	},
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}
