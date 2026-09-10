package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/kaktooslabs/kaktoos/internal/config"
	"github.com/kaktooslabs/kaktoos/internal/engine"
	"github.com/kaktooslabs/kaktoos/internal/httpclient"
	"github.com/kaktooslabs/kaktoos/internal/idempotency"
	"github.com/kaktooslabs/kaktoos/internal/openapi"
	"github.com/kaktooslabs/kaktoos/internal/ratelimit"
	"github.com/kaktooslabs/kaktoos/internal/reporter"
	"github.com/kaktooslabs/kaktoos/internal/scenario"
	"github.com/kaktooslabs/kaktoos/internal/schema"
	"github.com/kaktooslabs/kaktoos/internal/tracer"
)

var (
	openapiPath    string
	envPath        string
	scenarioPaths  []string
	traceEnabled   bool
	traceFormat    string
	traceSensitive bool
	tags           []string
	excludeTags    []string
	outputFormat   string
	schemaMode     string
	inlineScenario []string
)

// filterByTags applies --tag (inclusion) then --exclude-tag (exclusion), in that order.
// Scenarios with no tags are matched by neither flag.
func filterByTags(scenarios []*scenario.Scenario, include, exclude []string) []*scenario.Scenario {
	if len(include) == 0 && len(exclude) == 0 {
		return scenarios
	}
	out := make([]*scenario.Scenario, 0, len(scenarios))
	for _, s := range scenarios {
		if len(include) > 0 && !hasAnyTag(s.Tags, include) {
			continue
		}
		if len(exclude) > 0 && hasAnyTag(s.Tags, exclude) {
			continue
		}
		out = append(out, s)
	}
	return out
}

func hasAnyTag(tags, match []string) bool {
	for _, t := range tags {
		for _, m := range match {
			if t == m {
				return true
			}
		}
	}
	return false
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Execute API test scenarios",
	RunE: func(cmd *cobra.Command, args []string) error {
		switch outputFormat {
		case "text", "json", "junit":
		default:
			fmt.Fprintf(os.Stderr, "invalid --output-format %q: must be text, json, or junit\n", outputFormat)
			os.Exit(2)
		}

		switch schema.Mode(schemaMode) {
		case schema.Off, schema.Warn, schema.Strict:
		default:
			fmt.Fprintf(os.Stderr, "invalid --schema-mode %q: must be off, warn, or strict\n", schemaMode)
			os.Exit(2)
		}

		if len(scenarioPaths) == 0 && len(inlineScenario) == 0 {
			fmt.Fprintln(os.Stderr, "at least one of --scenario or --scenario-inline is required")
			os.Exit(2)
		}

		// Load environment
		env, err := config.Load(envPath)
		if err != nil {
			return withSuggestion(fmt.Errorf("loading environment: %w", err))
		}

		// Load OpenAPI spec
		ops, err := openapi.Load(openapiPath)
		if err != nil {
			return withSuggestion(fmt.Errorf("loading OpenAPI spec: %w", err))
		}

		scenarios, err := loadScenarios(scenarioPaths, inlineScenario)
		if err != nil {
			return withSuggestion(err)
		}

		scenarios = filterByTags(scenarios, tags, excludeTags)
		if len(scenarios) == 0 {
			fmt.Fprintln(os.Stderr, "no scenarios matched the provided filters")
			os.Exit(2)
		}

		results := make([]engine.ExecutionResult, 0, len(scenarios))
		client := httpclient.NewClient().HTTPClient()
		idempotencyStore := idempotency.NewMemoryStore()
		var limiter *ratelimit.Limiter
		if len(env.RateLimits) > 0 {
			limiter = ratelimit.NewLimiter(env.RateLimits)
		}
		for _, scn := range scenarios {
			ctx := engine.NewContext(env, ops)
			ctx.Idempotency = idempotencyStore
			ctx.RateLimiter = limiter
			ctx.TraceEnabled = traceEnabled
			ctx.SchemaMode = schema.Mode(schemaMode)
			results = append(results, engine.RunScenario(ctx, *scn, client))
		}

		// Report results
		switch outputFormat {
		case "json":
			if err := reporter.ReportJSON(results, os.Stdout); err != nil {
				return fmt.Errorf("writing JSON report: %w", err)
			}
		case "junit":
			if err := reporter.ReportJUnit(results, os.Stdout); err != nil {
				return fmt.Errorf("writing JUnit report: %w", err)
			}
		default:
			reporter.Report(results, os.Stdout)
			printRunSuggestions(results, os.Stdout)
		}
		t := tracer.New(traceEnabled, tracer.Format(traceFormat), traceSensitive)
		for _, r := range results {
			_ = t.Write(r, os.Stderr)
		}

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
	runCmd.Flags().StringArrayVar(&scenarioPaths, "scenario", nil, "Path to scenario file (repeatable)")
	runCmd.Flags().BoolVar(&traceEnabled, "trace", false, "enable execution trace output")
	runCmd.Flags().StringVar(&traceFormat, "trace-format", "text", "trace format: text or json")
	runCmd.Flags().BoolVar(&traceSensitive, "trace-sensitive", false, "include sensitive headers in trace")
	runCmd.Flags().StringArrayVar(&tags, "tag", nil, "run only scenarios with this tag (repeatable)")
	runCmd.Flags().StringArrayVar(&excludeTags, "exclude-tag", nil, "skip scenarios with this tag (repeatable)")
	runCmd.Flags().StringVar(&outputFormat, "output-format", "text", "output format: text, json, or junit")
	runCmd.Flags().StringVar(&schemaMode, "schema-mode", "off", "OpenAPI response schema validation: off, warn, or strict")
	runCmd.Flags().StringArrayVar(&inlineScenario, "scenario-inline", nil, "inline scenario YAML (repeatable)")
	runCmd.MarkFlagRequired("openapi")
	runCmd.MarkFlagRequired("env")

	rootCmd.AddCommand(runCmd)
}
