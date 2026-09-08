package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// version is the application version. It can be overridden at build time using ldflags.
// Example: go build -ldflags "-X main.version=v1.0.0"
var version = "dev"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintln(cmd.OutOrStdout(),"Kaktoos",version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
