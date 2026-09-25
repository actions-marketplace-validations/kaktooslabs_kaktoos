# MCP tools

Kaktoos exposes its core over the Model Context Protocol so an AI coding agent
can get engineering context and run verification without shelling out. MCP is
an interface to the same deterministic core the CLI uses — no logic lives only
in the MCP layer.

## Start the server

```bash
kaktoos mcp
```

See the README's [MCP Client Configuration](../README.md#mcp-client-configuration)
section for wiring it into a specific client.

## Tools

### Verification (execution)

| Tool | Purpose |
| --- | --- |
| `list_operations` | List the API operations declared in an OpenAPI spec |
| `validate_scenario` | Check a scenario YAML is well-formed; no execution |
| `run_workflow` | Execute a scenario, report per-step outcomes |

### Engineering context (before you change)

| Tool | Purpose |
| --- | --- |
| `get_related_context` | Given a plain-language description, return the services, APIs, workflows, owners, related work and docs already in the repo that touch it |
| `get_dependencies` | Given an entity id, return what it depends on, what depends on it, and what it exposes |
| `get_owners` | CODEOWNERS lookup for a set of paths |
| `get_related_work` | Work items related to a change (issue keys from branch/commit text today; a Jira provider can be added later) |
| `get_related_documents` | Documentation related to a set of terms (reports "unavailable" — no provider ships yet) |
| `propose_change` | Grounded existing-system context for a proposed change; returns no design of its own |

### Change impact (after you change)

| Tool | Purpose |
| --- | --- |
| `analyze_change` | Analyze a change (working tree, commit, base/head range, or PR) — what actually changed vs. what could be affected |
| `get_verification_plan` | Existing Kaktoos workflows that verify the potentially affected behavior; never generates one |
| `run_verification` | Execute a scenario named by the plan; delegates to `run_workflow` |

## `get_related_context`

Takes a natural-language description, tokenizes it, and matches those tokens
against entity names and attributes already discovered from the repo — no
inference about intent, no invented relationships. Every matched or related
entity is grounded in a file, config line, OpenAPI operation, or scenario.

```json
{"query": "scheduled payment cancellation", "repo": "."}
```

Response shape:

```json
{
  "matched": [{"kind": "service", "id": "service:payment-service", "name": "payment-service"}],
  "related": [{"kind": "operation", "id": "operation:POST /payments", "name": "POST /payments"}],
  "workflows": [{"kind": "workflow", "id": "workflow:payment-create", "name": "payment-create"}],
  "owners": ["@payments-team"],
  "explanations": [{"from": "...", "to": "...", "kind": "exposes", "why": "declared in payment-service/openapi.yml", "inferred": false}],
  "notes": []
}
```

When nothing matches, `matched` and `related` are empty and `notes` says so
explicitly — Kaktoos never invents a relationship to fill the gap.

## `analyze_change` / `get_verification_plan`

Both take the same selector shape as `kaktoos impact`:

```json
{"repo": ".", "commit": "abc123"}
{"repo": ".", "base": "main", "head": "feature/x"}
{"repo": ".", "pr": 184}
{"repo": "."}   // working tree
```

`analyze_change` returns `changed`, `potential` (with each entity's relation
`path`), `owners`, `workflows`, `related_work_keys`, and a `disclaimer` field
that states the Changed/Potential distinction in words, so a client rendering
the raw JSON still surfaces it:

> "Changed lists what the diff actually touched. Potential impact is what is
> reachable from those files through declared and inferred relationships — it
> indicates what to verify, not what is broken."

`get_verification_plan` returns just the `workflows` an agent should run next,
plus guidance to call `run_verification` on each — it only ever selects
scenarios that already exist.

## `run_verification`

```json
{"scenario_path": "payment-service/scenarios/payment-create.yml", "openapi_path": "payment-service/openapi.yml"}
```

Reads the named scenario file and executes it through the same path as
`run_workflow` (and therefore the same engine `kaktoos run` uses). Returns
pass/fail per step with `failure_category` and evidence — nothing is
re-implemented here.

## Typical before/after agent flow

```text
propose_change("cancel a scheduled payment")
        │
        ▼ grounded context: services, APIs, workflows, owners
   agent designs and implements the change
        │
        ▼
analyze_change()                       # what did I touch, what could it affect?
        │
        ▼
get_verification_plan()                # which existing workflows cover that?
        │
        ▼
run_verification(scenario_path=...)    # prove it still works
```

## Providers

`get_related_work` and `get_related_documents` are backed by interfaces
(`WorkItemProvider`, `DocumentProvider`), not a database. Today:

- Work items: issue keys (`[A-Z][A-Z0-9]+-\d+`) extracted from branch names and
  commit/PR text. Deterministic, no network calls, no credentials.
- Documents: no provider ships yet — the tool reports `available: false` with
  a reason rather than pretending no documentation exists.

A Jira or Confluence provider can be added later behind the same interface
without touching any tool's request/response shape.

## See also

- [Engineering context and change impact](impact.md) — the CLI equivalent and
  full concept explanation
