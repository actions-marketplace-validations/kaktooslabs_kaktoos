// Package mcp exposes Kaktoos over the Model Context Protocol so an AI coding
// agent can discover API operations and verify integrations. It is a thin
// adapter: every execution path goes through engine.RunScenario. The core
// engine has no dependency on this package or on any AI provider.
package mcp

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Version is reported to MCP clients during initialization.
const Version = "0.1.0"

// errorResult builds a structured, human-readable tool error. Tool failures
// are reported this way rather than as Go errors, so a stack trace never
// reaches the client.
func errorResult(msg string) *mcpsdk.CallToolResult {
	return &mcpsdk.CallToolResult{
		IsError: true,
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: msg}},
	}
}

// NewServer wires the three Kaktoos tools onto an MCP server.
func NewServer() *mcpsdk.Server {
	s := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "kaktoos", Version: Version}, nil)

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "list_operations",
		Description: "List the API operations declared in an OpenAPI specification, with their methods, paths, parameters and declared response codes.",
	}, ListOperations)

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "validate_scenario",
		Description: "Check that a scenario YAML is well-formed and, when an OpenAPI spec is supplied, that every step operation exists. Does not execute anything.",
	}, ValidateScenario)

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "run_workflow",
		Description: "Execute a scenario against a real API and report per-step outcomes: HTTP failures, assertion failures, and OpenAPI response schema violations.",
	}, RunWorkflow)

	return s
}

// Serve runs the MCP server over stdio until the context is cancelled or the
// client disconnects.
func Serve(ctx context.Context) error {
	return NewServer().Run(ctx, &mcpsdk.StdioTransport{})
}
