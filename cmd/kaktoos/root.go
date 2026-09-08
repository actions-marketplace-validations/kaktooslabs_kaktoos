package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "kaktoos",
	Short: "API regression testing tool",
}

// Execute runs the root command with panic recovery.
func Execute() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "PANIC: %v\n", r)
			os.Exit(3)
		}
	}()

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
