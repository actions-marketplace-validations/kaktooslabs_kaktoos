package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/kaktooslabs/kaktoos/internal/config"
	"github.com/kaktooslabs/kaktoos/internal/engine"
	"github.com/kaktooslabs/kaktoos/internal/openapi"
	"github.com/kaktooslabs/kaktoos/internal/reporter"
	"github.com/kaktooslabs/kaktoos/internal/scenario"
)

var (
	openapiPath  string
	envPath      string
	scenarioPath string
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Execute API test scenarios",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load environment
		env, err := config.Load(envPath)
		if err != nil {
			return fmt.Errorf("loading environment: %w", err)
		}

		// Load OpenAPI spec
		ops, err := openapi.Load(openapiPath)
		if err != nil {
			return fmt.Errorf("loading OpenAPI spec: %w", err)
		}

		// Load scenario - scenario.Load takes []byte, not string path
		scenarioData, err := os.ReadFile(scenarioPath)
		if err != nil {
			return fmt.Errorf("reading scenario file: %w", err)
		}
		scn, err := scenario.Load(scenarioData)
		if err != nil {
			return fmt.Errorf("loading scenario: %w", err)
		}

		// Execute scenario
		ctx := engine.NewContext(env, ops)
		// FIX: Use standard http.Client for compatibility with engine.RunScenario
		client := &http.Client{
			Timeout: 30 * time.Second,
		}
		result := engine.RunScenario(ctx, *scn, client)

		// Report results
		results := []engine.ExecutionResult{result}
		reporter.Report(results, os.Stdout)

		// Exit with proper code
		exitCode := reporter.ExitCode(results)
		if exitCode != 0 {
			os.Exit(exitCode)
		}

		return nil
	},
}

func init() {
	runCmd.Flags().StringVar(&openapiPath, "openapi", "", "Path to OpenAPI spec (required)")
	runCmd.Flags().StringVar(&envPath, "env", "", "Path to environment file (required)")
	runCmd.Flags().StringVar(&scenarioPath, "scenario", "", "Path to scenario file (required)")
	runCmd.MarkFlagRequired("openapi")
	runCmd.MarkFlagRequired("env")
	runCmd.MarkFlagRequired("scenario")

	rootCmd.AddCommand(runCmd)
}
