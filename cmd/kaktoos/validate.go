package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/kaktooslabs/kaktoos/internal/config"
	"github.com/kaktooslabs/kaktoos/internal/openapi"
	"github.com/kaktooslabs/kaktoos/internal/scenario"
)

var (
	validateOpenapiPath  string
	validateEnvPath      string
	validateScenarioPath string
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate configuration files",
	RunE: func(cmd *cobra.Command, args []string) error {
		_, err := config.Load(validateEnvPath)
		if err != nil {
			return fmt.Errorf("environment validation failed: %w", err)
		}
		fmt.Println("✓ Environment file valid")

		_, err = openapi.Load(validateOpenapiPath)
		if err != nil {
			return fmt.Errorf("OpenAPI validation failed: %w", err)
		}
		fmt.Println("✓ OpenAPI spec valid")

		scenarioData, err := os.ReadFile(validateScenarioPath)
		if err != nil {
			return fmt.Errorf("reading scenario file: %w", err)
		}
		_, err = scenario.Load(scenarioData)
		if err != nil {
			return fmt.Errorf("scenario validation failed: %w", err)
		}
		if err != nil {
			return fmt.Errorf("scenario validation failed: %w", err)
		}
		if err != nil {
			return fmt.Errorf("scenario validation failed: %w", err)
		}
		fmt.Println("✓ Scenario file valid")

		fmt.Println("\nAll files valid!")
		return nil
	},
}

func init() {
	validateCmd.Flags().StringVar(&validateOpenapiPath, "openapi", "", "Path to OpenAPI spec (required)")
	validateCmd.Flags().StringVar(&validateEnvPath, "env", "", "Path to environment file (required)")
	validateCmd.Flags().StringVar(&validateScenarioPath, "scenario", "", "Path to scenario file (required)")
	validateCmd.MarkFlagRequired("openapi")
	validateCmd.MarkFlagRequired("env")
	validateCmd.MarkFlagRequired("scenario")

	rootCmd.AddCommand(validateCmd)
}
