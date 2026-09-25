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

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "get_related_context",
		Description: "Given a plain-language description of work you're about to do, return the services, APIs, workflows, owners, related work and docs already in this repository that touch it. Every result is grounded in something discovered from the repo; nothing is inferred beyond declared or convention-derived relationships, and those are marked inferred.",
	}, GetRelatedContext)

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "get_dependencies",
		Description: "Given an entity id (service, file, or operation), return what it depends on, what depends on it, and what it exposes.",
	}, GetDependencies)

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "get_owners",
		Description: "Look up CODEOWNERS ownership for a set of paths. Ownership is contextual information, not a guarantee of who is responsible.",
	}, GetOwners)

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "get_related_work",
		Description: "Find work items (e.g. issue keys) related to a change. Reports cleanly when no work-item provider is configured rather than failing.",
	}, GetRelatedWork)

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "get_related_documents",
		Description: "Find documentation related to a set of search terms. Reports cleanly when no documentation provider is configured rather than failing.",
	}, GetRelatedDocuments)

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "analyze_change",
		Description: "Analyze a change (working tree, a commit, a base/head range, or a GitHub PR) against the repository's engineering context. Reports what was actually changed and what could be affected, clearly separated — potential impact is reachability, never a claim that something is broken.",
	}, AnalyzeChange)

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "get_verification_plan",
		Description: "Given a change, identify existing Kaktoos workflows that verify the potentially affected behavior. Only selects scenarios that already exist; never generates one.",
	}, GetVerificationPlan)

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "run_verification",
		Description: "Execute an existing scenario file (as named by get_verification_plan) and report pass/fail with evidence. Delegates to the same execution path as run_workflow.",
	}, RunVerification)

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "propose_change",
		Description: "Given a plain-language description of a proposed change, return grounded existing-system context (components, APIs, workflows, dependencies, related work, docs) to design against. Returns no design of its own — the calling agent remains responsible for that.",
	}, ProposeChange)

	return s
}

// Serve runs the MCP server over stdio until the context is cancelled or the
// client disconnects.
func Serve(ctx context.Context) error {
	return NewServer().Run(ctx, &mcpsdk.StdioTransport{})
}
