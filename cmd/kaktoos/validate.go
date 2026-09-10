package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kaktooslabs/kaktoos/internal/config"
	"github.com/kaktooslabs/kaktoos/internal/openapi"
	"github.com/kaktooslabs/kaktoos/internal/scenario"
	"github.com/kaktooslabs/kaktoos/internal/template"
	"github.com/kaktooslabs/kaktoos/internal/variable"
)

var (
	validateOpenapiPath    string
	validateEnvPath        string
	validateScenarioPaths  []string
	validateScenarioInline []string
)

var validateCmd = &cobra.Command{
	Use:           "validate",
	Short:         "Validate configuration files",
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(validateScenarioPaths) == 0 && len(validateScenarioInline) == 0 {
			fmt.Fprintln(cmd.ErrOrStderr(), "at least one of --scenario or --scenario-inline is required")
			os.Exit(2)
		}
		errs, warnings := runValidation(validateEnvPath, validateOpenapiPath, validateScenarioPaths, validateScenarioInline, cmd.OutOrStdout())

		for _, w := range warnings {
			fmt.Fprintln(cmd.ErrOrStderr(), w)
		}
		if len(errs) > 0 {
			for _, e := range errs {
				fmt.Fprintln(cmd.ErrOrStderr(), e)
				if s := suggestFor(e); s != "" {
					fmt.Fprintln(cmd.ErrOrStderr(), s)
				}
			}
			os.Exit(2)
		}

		fmt.Fprintln(cmd.OutOrStdout(), "\nAll files valid!")
		return nil
	},
}

// runValidation validates every input file, collecting all errors and warnings
// rather than stopping at the first failure.
func runValidation(envPath, openapiPath string, scenarioPaths []string, scenarioInline []string, out io.Writer) (errs []string, warnings []string) {
	if _, err := config.Load(envPath); err != nil {
		errs = append(errs, fmt.Sprintf("environment validation failed: %v", err))
	} else {
		fmt.Fprintln(out, "✓ Environment file valid")
	}

	if _, err := openapi.Load(openapiPath); err != nil {
		errs = append(errs, fmt.Sprintf("OpenAPI validation failed: %v", err))
	} else {
		fmt.Fprintln(out, "✓ OpenAPI spec valid")
	}

	for _, path := range scenarioPaths {
		data, err := os.ReadFile(path)
		if err != nil {
			errs = append(errs, fmt.Sprintf("reading scenario file %s: %v", path, err))
			continue
		}
		scn, err := scenario.Load(data)
		if err != nil {
			errs = append(errs, fmt.Sprintf("scenario validation failed (%s): %v", path, err))
			continue
		}

		e, w := validateScenarioPhase2(scn, path)
		errs = append(errs, e...)
		warnings = append(warnings, w...)
		if len(e) == 0 {
			fmt.Fprintf(out, "✓ Scenario file valid: %s\n", path)
		}
	}

	// Inline scenarios are parsed from memory — no filesystem access.
	for i, y := range scenarioInline {
		label := fmt.Sprintf("inline scenario #%d", i+1)
		scn, err := scenario.Load([]byte(y))
		if err != nil {
			errs = append(errs, fmt.Sprintf("scenario validation failed (%s): %v", label, err))
			continue
		}
		e, w := validateScenarioPhase2(scn, label)
		errs = append(errs, e...)
		warnings = append(warnings, w...)
		if len(e) == 0 {
			fmt.Fprintf(out, "✓ Scenario valid: %s\n", label)
		}
	}

	return errs, warnings
}

// validateScenarioPhase2 adds the Phase 2 checks that the loader does not cover:
// template function syntax, and the client-error retry warning.
func validateScenarioPhase2(scn *scenario.Scenario, path string) (errs []string, warnings []string) {
	for _, step := range scn.Steps {
		for _, field := range templatedFields(step) {
			if err := checkTemplateSyntax(field, step.Name); err != nil {
				errs = append(errs, fmt.Sprintf("scenario %s: %v", path, err))
			}
		}

		if step.Retry == nil {
			continue
		}
		for _, code := range step.Retry.RetryOn.StatusCodes {
			if code < 500 {
				warnings = append(warnings, fmt.Sprintf(
					"Warning: retry policy for step '%s' includes client error status codes; these are usually deterministic failures",
					step.Name))
				break
			}
		}
	}
	return errs, warnings
}

// templatedFields returns every step string that may contain template tokens.
func templatedFields(step scenario.Step) []string {
	if step.Request == nil {
		return nil
	}
	fields := []string{step.Request.Body}
	for _, m := range []map[string]string{step.Request.Path, step.Request.Query, step.Request.Headers} {
		for _, v := range m {
			fields = append(fields, v)
		}
	}
	return fields
}

// checkTemplateSyntax validates function name and argument count only. It resolves
// against an empty store, so "variable not found" is expected and ignored here —
// variable presence is a runtime concern, not a syntax one.
func checkTemplateSyntax(s, stepName string) error {
	_, err := template.Resolve(s, variable.NewStore(), stepName)
	if err == nil || strings.Contains(err.Error(), "not found") {
		return nil
	}
	return err
}

func init() {
	validateCmd.Flags().StringVar(&validateOpenapiPath, "openapi", "", "Path to OpenAPI spec (required)")
	validateCmd.Flags().StringVar(&validateEnvPath, "env", "", "Path to environment file (required)")
	validateCmd.Flags().StringArrayVar(&validateScenarioPaths, "scenario", nil, "Path to scenario file (repeatable)")
	validateCmd.Flags().StringArrayVar(&validateScenarioInline, "scenario-inline", nil, "inline scenario YAML (repeatable)")
	validateCmd.MarkFlagRequired("openapi")
	validateCmd.MarkFlagRequired("env")

	rootCmd.AddCommand(validateCmd)
}
