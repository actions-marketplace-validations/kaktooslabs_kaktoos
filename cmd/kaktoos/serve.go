package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kaktooslabs/kaktoos/internal/webhook"
)

var (
	webhookConfigPath string
	servePort         int
	serveHost         string
)

var serveCmd = &cobra.Command{
	Use:           "serve",
	Short:         "Run the webhook server",
	SilenceUsage:  true,
	SilenceErrors: false,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := webhook.LoadConfig(webhookConfigPath)
		if err != nil {
			return fmt.Errorf("loading webhook config: %w", err)
		}

		port := cfg.Server.Port
		if cmd.Flags().Changed("port") {
			port = servePort
		}
		host := cfg.Server.Host
		if cmd.Flags().Changed("host") {
			host = serveHost
		}

		return webhook.NewServer(cfg, port, host).Start()
	},
}

func init() {
	serveCmd.Flags().StringVar(&webhookConfigPath, "config", "", "Path to webhook config file (required)")
	serveCmd.Flags().IntVar(&servePort, "port", 8080, "Port to listen on (overrides config)")
	serveCmd.Flags().StringVar(&serveHost, "host", "0.0.0.0", "Host to bind to (overrides config)")
	serveCmd.MarkFlagRequired("config")

	rootCmd.AddCommand(serveCmd)
}
