# Kaktoos

> Engineering context and independent verification for AI coding agents.
> Understand before you change. Verify after you change.

Kaktoos gives AI coding agents grounded context about an existing engineering
system before they design or implement a change — services, APIs, workflows,
dependencies, ownership. After a change is made, Kaktoos analyzes it,
identifies potential impact, and can run existing verification workflows
against the affected behavior.

Kaktoos also verifies API integrations against **OpenAPI contracts and real
API behavior**. That verification engine works independently of AI and can be
used locally or in CI/CD — it's what runs under the hood on both sides of the
before/after loop.

Use Kaktoos to:

- Give an AI agent grounded engineering context before it designs a change
- Analyze a change/PR and identify what it could affect, then run the existing workflows that verify it
- Run multi-step API workflows
- Verify responses against OpenAPI contracts
- Catch integration mistakes independently of application tests
- Expose Kaktoos to AI coding agents through MCP
- Run the same verification locally or in CI/CD
- Keep API verification scenarios version-controlled in Git

```text
        AI coding agent / Developer
                    │
        ┌───────────┴───────────┐
        │                       │
        ▼                       ▼
  BEFORE CHANGE             AFTER CHANGE
        │                       │
   Kaktoos MCP                  PR
        │                       │
        ▼                       ▼
  Engineering               Change
    Context                Analysis
        │                       │
        ▼                       ▼
   AI designs             Potential
   the change               Impact
                                │
                                ▼
                          Existing Kaktoos
                            Workflows
                                │
                                ▼
                           Verification
                                │
                                ▼
                            Evidence
```

Kaktoos works entirely without AI. AI coding agents are an optional interface through the Model Context Protocol (MCP). Kaktoos is not a knowledge base, a chatbot, or a coding agent — it grounds an agent's understanding of the existing system and independently checks its work.

---

# Contents

**Start here**

- [Engineering Context and Change Impact](docs/impact.md) — understand before you change, verify after
- [Why Kaktoos?](#why-kaktoos)
- [Key Capabilities](#key-capabilities)
- [How It Works](#how-it-works)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Example](#example)

**Writing scenarios**

- [Configuration](#configuration)
- [Assertions](#assertions)
- [Response Schema Validation](#response-schema-validation)
- [Inline Scenarios](#inline-scenarios)
- [Scenario file reference](docs/scenario-reference.md) — every field
- [Verification patterns](docs/scenarios.md) — contract, read-after-write state, cleanup

**AI coding agents**

- [Using Kaktoos from an AI Coding Agent](#using-kaktoos-from-an-ai-coding-agent)
- [Typical AI Agent Workflow](#typical-ai-agent-workflow)
- [Typical AI Feature Development Workflow](#typical-ai-feature-development-workflow)
- [MCP Client Configuration](#mcp-client-configuration)
- [MCP tools reference](docs/mcp.md) — all 12 tools

**Automation**

- [GitHub Action](#github-action)
- [CI/CD](#cicd)
- [Workflow Automation](#workflow-automation)
- [Machine-Readable Output](#machine-readable-output)
- [Execution Traces](#execution-traces)
- [Webhooks](#webhooks)

**Reference**

- [Project Structure](#project-structure)
- [Commands](#commands)
- [Examples](#examples)
- [Development](#development)
- [Project Status](#project-status)
- [Contributing](#contributing)
- [License](#license)

---

## Why Kaktoos?

Testing an individual API endpoint is easy.

Testing whether multiple APIs still work **together** is where things become complicated.

For example:

```text
Create Customer
      ↓
Create Account
      ↓
Make Payment
      ↓
Verify Payment
```

A single endpoint test may pass while the complete workflow is broken.

Kaktoos lets developers and QA engineers define repeatable API workflows as version-controlled scenarios and execute them against real APIs.

The OpenAPI specification provides the API contract. Kaktoos verifies that the real API behavior and workflow still match that contract.

> **Git tracks what changed. Kaktoos tells you whether it still works.**

## The other half: changing a system you don't fully understand

An AI coding agent can find code without understanding the engineering system
around it. Searching a repo surfaces the file; it doesn't surface:

- related services
- consumers of the API being changed
- existing verification workflows that cover the behavior
- declared dependencies
- ownership
- related engineering work
- architecture and documentation context

Kaktoos exposes that context through MCP **before** an agent designs a change,
so the design starts from what already exists rather than from a guess.

**After** implementation, Kaktoos analyzes the resulting change or PR, reports
what it could affect, and runs the existing verification workflows covering
that behavior.

Kaktoos does not prove business correctness and does not decide the right
architecture. It supplies grounded context and independent evidence; the design
decision stays with the developer or agent.

---

## Key Capabilities

### Engineering context and change impact

Before an AI agent designs a change, it can ask Kaktoos what already exists:
services, APIs, workflows, dependencies, owners, related work — derived from
the repository and the engineering sources available to it, never invented.

After the change, `kaktoos impact` (or the `analyze_change` MCP tool) reports
what actually changed and what could be affected, clearly separated:

| | Meaning |
| --- | --- |
| **Changed** | Fact — what the diff actually touched |
| **Potential impact** | Reachability — what is connected to the change and worth verifying. Not a claim that anything is broken |

It then points at the **existing** Kaktoos workflows that verify that
behavior — it never generates one.

```text
Feature request
     ↓
Kaktoos MCP
     ↓
Existing system context
     ↓
AI designs change
     ↓
PR
     ↓
kaktoos impact
     ↓
Potential impact
     ↓
Existing verification workflows
     ↓
Actual verification
     ↓
Evidence
```

Every relationship Kaktoos reports carries a reason (`why`) and is flagged
`inferred` when it came from convention rather than an explicit declaration.
Kaktoos matches names, paths, operations and declared dependencies — it does
not claim semantic understanding of your code.

This is not a knowledge base or a second workflow engine. It's the same
deterministic core as the rest of Kaktoos, reused. See
[docs/impact.md](docs/impact.md) and [docs/mcp.md](docs/mcp.md).

### Multi-step API workflows

Define workflows where data extracted from one API response is used by subsequent requests.

```text
Create user
     │
     │ $.id
     ▼
  userId
     │
     ▼
GET /users/{{userId}}
```

### OpenAPI-powered

Kaktoos uses your OpenAPI specification to resolve API operations.

Your OpenAPI specification remains the contract, while Kaktoos scenarios describe how those APIs should work together.

### Independent response verification

Kaktoos can validate API responses against the OpenAPI response schema, in addition to explicit assertions defined in scenarios.

This can catch:

- Undeclared status codes
- Content-Type mismatches
- Missing required fields
- Incorrect response types
- Failed enum/pattern constraints
- Other schema violations

### State verification, not just responses

A `200` proves the API answered. It does not prove anything was stored.

Kaktoos verifies observable state with steps you already know how to write —
write, extract the id, read it back, assert on what came back, then clean up:

```text
POST /orders  → 200 {"id":"o-1"}     ← the API says it worked
     │ $.id
     ▼
GET /orders/o-1 → 404                ← the state says otherwise
     │
     ▼
FAILED: assertion_failed
```

Cleanup steps marked `always_run: true` still execute after a failure, so test
data is removed even when the verification in the middle failed.

See [docs/scenarios.md](docs/scenarios.md).

### Deterministic failure classification

Every failed step carries one machine-readable `failure_category`, computed
once by the engine and identical in the CLI, JSON, JUnit, MCP, and the GitHub
Action — no consumer parses error text:

| Category | Meaning |
| --- | --- |
| `contract_mismatch` | Response violated the OpenAPI schema (strict mode) |
| `assertion_failed` | A scenario `assert` or `condition` failed |
| `auth_failure` | HTTP 401 or 403 |
| `rate_limited` | HTTP 429 |
| `server_error` | HTTP 5xx |
| `transport_failure` | No usable response (connection refused, DNS) |
| `timeout` | Step or workflow deadline exceeded |
| `config_error` | Unknown operation, unresolved variable, bad JSONPath |

A 401 that also fails `assert: {status: 200}` reports as `auth_failure` — the
cause, not the symptom.

Each failure also carries structured evidence: method, URL, status, request and
response headers and bodies, with `Authorization`, `Cookie`, `X-Api-Key` and
friends redacted to `***`.

### AI coding agent integration

Kaktoos exposes an MCP server that allows AI coding agents to:

1. Discover API operations from OpenAPI
2. Construct scenarios
3. Validate scenarios before execution
4. Execute workflows against real APIs
5. Receive structured failures
6. Fix the integration
7. Run verification again

### Git-native

Keep your API specification, scenarios, and environments in Git.

```text
my-api-tests/
├── openapi.yml
├── kaktoos.yaml
├── environments/
│   ├── local.yml
│   └── staging.yml
└── scenarios/
    ├── customer/
    ├── account/
    └── payment/
```

API test changes can be reviewed through normal Git commits and pull requests.

### CI/CD ready

Run the same scenarios locally or in your CI/CD pipeline.

A failed scenario produces a non-zero exit code, allowing API regression tests to fail a CI job.

### No server required

Kaktoos runs as a standalone CLI.

There is no Kaktoos server, database, or account required for local verification.

---

# How It Works

Kaktoos combines an OpenAPI specification, an environment, and one or more scenarios.

```text
openapi.yml
     +
scenario.yml
     +
environment.yml
     │
     ▼
  Kaktoos
     │
     ├── Validate configuration
     ├── Resolve OpenAPI operations
     ├── Execute API requests
     ├── Extract response values
     ├── Substitute variables
     ├── Validate response schemas
     ├── Evaluate assertions
     ├── Classify failures + capture redacted evidence
     └── Report results
```

---

# Installation

Kaktoos is distributed as a standalone binary. You don't need Go installed.

## Download a release

Download the latest release for your operating system from:

https://github.com/KaktoosLabs/kaktoos/releases

Supported platforms:

- macOS ARM64
- macOS Intel
- Linux ARM64
- Linux AMD64
- Windows ARM64
- Windows AMD64

## Go developers

If you already have Go installed:

```bash
go install github.com/KaktoosLabs/kaktoos/cmd/kaktoos@v1.0.0
```

## Build from source

```bash
git clone https://github.com/KaktoosLabs/kaktoos.git
cd kaktoos
go build ./cmd/kaktoos
```

## macOS Note

Kaktoos is currently not signed and notarized by Apple. As a result, macOS may display a security warning when running the prebuilt binary.

If you already have Go installed, you can alternatively install Kaktoos with:

```bash
go install github.com/KaktoosLabs/kaktoos/cmd/kaktoos@v1.0.1
```

---

# Quick Start

## 1. Validate your test configuration

```bash
kaktoos validate \
  --openapi api-spec.yml \
  --env environments/local.yml \
  --scenario scenarios/test.yml
```

Example output:

```text
✓ Environment file valid
✓ OpenAPI spec valid
✓ Scenario file valid
All files valid!
```

## 2. Run a scenario

```bash
kaktoos run \
  --openapi api-spec.yml \
  --env environments/local.yml \
  --scenario scenarios/test.yml
```

## 3. Check the version

```bash
kaktoos version
```

Example:

```text
Kaktoos v1.0.0
```

---

# Example

## OpenAPI Specification

Kaktoos uses an OpenAPI 3.x specification with `operationId` values for API operations.

```yaml
openapi: 3.0.3

info:
  title: Example API
  version: 1.0.0

paths:
  /users:
    post:
      operationId: createUser
      summary: Create a user
      responses:
        "201":
          description: User created

  /users/{id}:
    get:
      operationId: getUser
      summary: Get a user
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
      responses:
        "200":
          description: User retrieved
```

## Environment

Environment files define the target API and reusable configuration.

```yaml
name: local

base_url: http://localhost:8080

variables:
  user_type: individual

headers:
  Content-Type: application/json
```

## Scenario

Scenarios describe multi-step API workflows. See
[docs/scenario-reference.md](docs/scenario-reference.md) for every field, and
[docs/scenarios.md](docs/scenarios.md) for contract verification,
read-after-write state verification, and cleanup.

```yaml
name: User Management Test

steps:
  - name: Create user
    operation: createUser
    request:
      body: |
        {
          "name": "Test User"
        }
    extract:
      userId: $.id
    assert:
      status: 201
      body:
        - path: $.name
          equals: "Test User"

  - name: Get user
    operation: getUser
    request:
      path:
        id: "{{userId}}"
    assert:
      status: 200
      body:
        - path: $.id
          equals: "{{userId}}"
        - path: $.name
          equals: "Test User"
```

The value extracted from the first request can be used by subsequent steps:

```text
Create user
    │
    │ $.id
    ▼
 userId
    │
    ▼
GET /users/{{userId}}
```

---

# Configuration

## OpenAPI

Kaktoos uses an OpenAPI 3.x specification with `operationId` values for API operations.

The `operationId` is used by scenarios to identify the API operation to execute.

## Environment

Environment files define the target API and reusable configuration.

```yaml
name: local

base_url: http://localhost:8080

variables:
  user_type: individual

headers:
  Content-Type: application/json
```

Secrets can be provided through environment variables rather than committing them to Git.

## Scenario

Scenarios describe multi-step API workflows.

Each step can define:

- API operation
- Request path parameters
- Query parameters
- Headers
- Request body
- Response value extraction
- Assertions

Variables can be extracted from one response and used in subsequent steps.

## kaktoos.yaml

Optional. Declares services so `kaktoos impact` and the MCP context tools can
map changed files to APIs, workflows, and owners. Without it, Kaktoos falls
back to convention and marks the derived relationships `inferred`. See
[docs/impact.md](docs/impact.md#kaktoosyaml).

```yaml
services:
  - name: payment-service
    paths: ["payment-service/**"]
    openapi: payment-service/openapi.yml
    scenarios: payment-service/scenarios/
    depends_on: [transaction-service]
```

---

# Assertions

Kaktoos currently supports:

- HTTP status assertions
- JSONPath equality
- JSONPath exists
- JSONPath not exists

Example:

```yaml
assert:
  status: 200
  body:
    - path: $.status
      equals: SUCCESS
```

Expected assertion values can also use extracted variables:

```yaml
assert:
  body:
    - path: $.id
      equals: "{{userId}}"
```

---

# Response Schema Validation

Kaktoos can check every response against the schema declared in your OpenAPI specification — not just the assertions you wrote by hand.

```bash
kaktoos run ... --schema-mode warn
kaktoos run ... --schema-mode strict
kaktoos run ... --schema-mode off
```

The default is `off`, so existing scenarios behave exactly as before.

`strict` is recommended for CI and agent-driven verification.

## Schema modes

### `off`

No response schema validation is performed.

### `warn`

Schema violations are reported but do not fail the step.

### `strict`

Schema violations are reported and fail the step.

## Violation types

| Kind | Meaning |
| --- | --- |
| `status_undeclared` | The API returned a status code the spec does not declare for that operation |
| `content_type_mismatch` | The response `Content-Type` is not among those the spec declares |
| `required_field_missing` | A field the schema marks `required` is absent |
| `schema_mismatch` | Any other schema failure such as wrong type, failed enum, pattern, min/max |

`warn` and `strict` report identical violations. Only the resulting step status differs.

## Undeclared response fields

Fields present in the response but absent from the schema are reported separately as `undeclared_fields`.

They are informational and never fail a run.

This usually means the API has evolved ahead of its OpenAPI specification.

## Schema implementation

Schema validation uses `kin-openapi`'s JSON Schema validator.

Composed schemas such as `allOf`, `oneOf`, and `anyOf` use the behavior provided natively by `kin-openapi`.

---

# Inline Scenarios

A scenario can be passed as a YAML string instead of a file.

This is useful for generated or one-off checks, including AI coding agent workflows.

```bash
kaktoos run \
  --openapi openapi.yml \
  --env env.yml \
  --schema-mode strict \
  --scenario-inline '
name: smoke
steps:
  - name: get user
    operation: getUser
    request:
      path:
        id: "123"
    assert:
      status: 200
'
```

`--scenario-inline` is repeatable and can be combined with `--scenario`.

At least one of `--scenario` or `--scenario-inline` is required.

`kaktoos validate` accepts the same flag.

---

# Using Kaktoos from an AI Coding Agent

Kaktoos works entirely without AI.

MCP is an **optional interface**. The core Kaktoos engine has no dependency on MCP or on any AI provider.

Every CLI command works whether or not an AI coding agent is involved.

```text
AI coding agent
      │
      ▼
   Kaktoos MCP
      │
      ├── Discover API contract
      ├── Validate scenario
      ├── Execute workflow
      └── Read structured result
               │
               ▼
          Fix integration
               │
               ▼
            Verify again
```

## Start the MCP server

```bash
kaktoos mcp
```

Kaktoos exposes twelve MCP tools over stdio, in three groups. Full request and
response shapes are in [docs/mcp.md](docs/mcp.md).

**Verification (execution)**

| Tool | Purpose |
| --- | --- |
| `list_operations` | List the operations declared by an OpenAPI specification |
| `validate_scenario` | Validate a scenario before execution |
| `run_workflow` | Execute a scenario against a real API and return structured results |

**Engineering context (before you change)**

| Tool | Purpose |
| --- | --- |
| `get_related_context` | Given a plain-language description, return the services, APIs, workflows, owners, related work and docs already in the repo that touch it |
| `get_dependencies` | For an entity: what it depends on, what depends on it, what it exposes |
| `get_owners` | CODEOWNERS lookup for a set of paths (contextual, not a guarantee) |
| `get_related_work` | Work items related to a change (issue keys from branch/commit text today) |
| `get_related_documents` | Documentation related to a set of terms |
| `propose_change` | Grounded existing-system context for a proposed change; returns no design of its own |

**Change impact (after you change)**

| Tool | Purpose |
| --- | --- |
| `analyze_change` | Analyze a change (working tree, commit, base/head range, or PR) — what changed vs. what could be affected |
| `get_verification_plan` | Existing Kaktoos workflows that verify the potentially affected behavior; never generates one |
| `run_verification` | Execute a scenario named by the plan, through the same engine `kaktoos run` uses |

`get_related_work` and `get_related_documents` sit behind provider interfaces.
Today only local, deterministic signals ship: issue keys extracted from branch
and commit text for work items, and no document provider at all — that tool
reports `available: false` with a reason rather than pretending no
documentation exists. A Jira or Confluence provider can be added later behind
the same interface. Jira and Confluence are context sources, not part of the
product.

### `list_operations`

Lists operations declared by the OpenAPI specification, including:

- HTTP method
- Path
- Operation ID
- Parameters
- Declared response codes

An optional substring filter can narrow the results.

### `validate_scenario`

Validates a scenario without making network calls.

When an OpenAPI specification is provided, Kaktoos also checks that referenced operations exist.

### `run_workflow`

Executes a scenario against a real API.

Results include per-step outcomes and distinguish between:

- HTTP failures
- Assertion failures
- Schema violations
- Different schema violation kinds

`run_workflow` defaults to:

```text
schema_mode = strict
```

This makes contract enforcement the default for agent-driven verification.

The CLI itself continues to default to:

```text
schema_mode = off
```

for backward compatibility.

---

# Typical AI Agent Workflow

A typical agent loop looks like this:

```text
1. list_operations
        ↓
2. Discover the real API contract
        ↓
3. Build a scenario inline
        ↓
4. validate_scenario
        ↓
5. run_workflow
        ↓
6. Read structured failure
        ↓
7. Fix integration code
        ↓
8. run_workflow again
        ↓
9. PASS
```

For example, if an API returns:

```json
{
  "id": "123",
  "amount": 100
}
```

while the OpenAPI contract requires:

```yaml
required:
  - id
  - total
```

Kaktoos can report:

```text
required_field_missing
$.total
```

The agent can then use that information to investigate and fix the integration.

---

# Typical AI Feature Development Workflow

The loop above verifies an integration. This one brackets a feature change.

1. A developer gives the agent a feature request.
2. The agent asks Kaktoos for related engineering context (`get_related_context` or `propose_change`).
3. Kaktoos returns the relevant services, APIs, workflows, dependencies, ownership and whatever related context is available — grounded in the repository.
4. The agent uses that context to design the change.
5. The agent implements the change.
6. The agent opens a PR.
7. Kaktoos analyzes the change (`analyze_change` / `kaktoos impact`).
8. Kaktoos reports potential impact.
9. Kaktoos identifies the existing verification workflows covering it (`get_verification_plan`).
10. Kaktoos runs them (`run_verification` / `kaktoos impact --verify`).
11. The agent investigates the structured failures, if any.

```text
Feature request
      ↓
get_related_context / propose_change
      ↓
Services · APIs · workflows · dependencies · owners
      ↓
Agent designs and implements the change      ← Kaktoos writes no code
      ↓
PR
      ↓
analyze_change
      ↓
Changed  +  Potential impact
      ↓
get_verification_plan        ← existing workflows only
      ↓
run_verification
      ↓
Evidence
```

Kaktoos does not write the implementation and does not create the PR. It
supplies context on the way in and evidence on the way out.

---

# MCP Client Configuration

A compatible MCP client can start Kaktoos using:

```json
{
  "mcpServers": {
    "kaktoos": {
      "command": "kaktoos",
      "args": ["mcp"]
    }
  }
}
```

The exact configuration location depends on the AI coding agent or MCP client being used.

---

# GitHub Action

Kaktoos provides a composite GitHub Action for running API verification in CI.

```yaml
- uses: kaktooslabs/kaktoos@v1
  with:
    openapi: openapi.yml
    env: environments/ci.yml
    scenario: scenarios/smoke.yml
    schema-mode: strict
```

## Inputs

| Input | Default | Description |
| --- | --- | --- |
| `openapi` | *required* | Path to the OpenAPI specification |
| `env` | *required* | Path to the environment file |
| `scenario` | `""` | Scenario path(s), space-separated |
| `schema-mode` | `warn` | `off`, `warn`, or `strict` |
| `version` | action ref | Kaktoos release to install |
| `comment` | `true` | Post/update a PR comment with the result |
| `impact` | `false` | Add a Changed / Potential impact / Owners / Verification summary for the PR |
| `impact-verify` | `false` | With `impact`, run only the existing workflows the impact plan selected |

The action:

1. Installs the requested Kaktoos release
2. Runs Kaktoos
3. Writes a per-step result table to the GitHub Actions job summary
4. Propagates Kaktoos's exit code
5. Maintains a single PR comment instead of creating a new comment on every run

## Impact analysis in CI

Both impact inputs are opt-in and default to `false`; with them off, the action
behaves exactly as before.

With `impact: true`, on a pull request the action runs
`kaktoos impact --pr <number> --format json` and appends a **Changed /
Potential impact / Owners / Verification** section to the GitHub Actions job
summary. If impact analysis fails, it emits a warning and the run continues.
The PR comment itself is unchanged — it still reports the verification result
only.

With `impact-verify: true`, the scenarios selected by the impact plan replace
the `scenario` input for that run, so only the workflows covering the
potentially affected behavior execute. They run through the **existing** run
step and the existing engine — impact verification introduces no second
verification path. If the plan selects nothing, the action reports "nothing to
verify" and passes.

## Fork Pull Requests

Fork pull requests receive verification, but do not receive a Kaktoos PR comment.

Kaktoos deliberately does not use `pull_request_target` for commenting because that can execute with repository secrets in scope against code originating from an untrusted fork.

The verification itself can still run on fork pull requests.

No secrets are read or echoed by the Kaktoos action.

The comment step uses only the ambient `GITHUB_TOKEN`.

---

# CI/CD

Kaktoos can run directly as a CLI in CI/CD pipelines.

Example GitHub Actions workflow:

```yaml
name: API Regression

on:
  pull_request:

jobs:
  api-test:
    runs-on: ubuntu-latest

    steps:
      - uses: actions/checkout@v4

      - name: Install Kaktoos
        # Download the Kaktoos release binary

      - name: Validate API tests
        run: kaktoos validate

      - name: Run API regression tests
        run: kaktoos run
```

A failed scenario returns a non-zero exit code, allowing the CI pipeline to fail.

## Exit codes

```text
0 = all scenarios passed
1 = assertion or execution failure
2 = configuration or load error
3 = unexpected error
```

---

# Workflow Automation

## Filtering by Tag

Scenarios may declare `tags:`.

Filter which scenarios run:

```bash
kaktoos run \
  --openapi openapi.yml \
  --env env.yml \
  --scenario tests/*.yml \
  --tag smoke \
  --exclude-tag slow
```

`--tag` and `--exclude-tag` are repeatable.

Inclusion is applied first, then exclusion.

Scenarios with no tags are matched by neither flag.

If no scenario matches, Kaktoos exits `2`.

---

# Machine-Readable Output

Kaktoos supports machine-readable output for CI and automation.

```bash
kaktoos run ... --output-format json
```

or:

```bash
kaktoos run ... --output-format junit
```

The default output format is human-readable text.

A failed step in JSON output carries `failure_category` and redacted `evidence`:

```json
{
  "name": "read back",
  "status": "FAILED",
  "failure_category": "assertion_failed",
  "evidence": {
    "method": "GET",
    "url": "https://api.example.com/orders/o-1",
    "status_code": 404,
    "request_headers": {"Authorization": "***"},
    "response_body": "{\"error\":\"not found\"}"
  }
}
```

Both keys are omitted on passing steps, so existing passing output is unchanged.

JUnit behavior, routed by `failure_category`:

- `assertion_failed` and `contract_mismatch` are reported as `<failure>`
- Everything else (`auth_failure`, `rate_limited`, `server_error`, `transport_failure`, `timeout`, `config_error`) is reported as `<error>`

---

# Execution Traces

Kaktoos can produce execution traces:

```bash
kaktoos run ... --trace --trace-format json
```

Traces are written to stderr, so they do not mix with machine-readable output written to stdout.

Sensitive headers are redacted unless:

```bash
--trace-sensitive
```

is explicitly passed.

---

# Webhooks

Kaktoos can run workflows in response to incoming webhooks.

```bash
kaktoos serve \
  --config webhooks.yml \
  --port 8080 \
  --host 0.0.0.0
```

Example configuration:

```yaml
server:
  port: 8080
  host: 0.0.0.0

routes:
  - path: /hooks/orders/{id}
    method: POST

    auth:
      type: hmac_sha256
      # or bearer_token, basic
      secret: your-shared-secret
      header: X-Hub-Signature-256

    workflow:
      openapi: openapi.yml
      environment: env.yml
      scenario: workflows/order-check.yml

    variable_mapping:
      orderId: path.id
      customer: body.customer.name
```

Requests are authenticated before the workflow runs.

Request bodies are capped at 10 MB.

`path.*` and `body.*` values become workflow variables.

Environment variables take precedence over webhook variables when names collide.

The webhook response is HTTP `200` whether the workflow passes or fails. The response body contains the workflow result.

HTTP `500` is reserved for infrastructure failures.

---

# Project Structure

A typical Kaktoos test repository:

```text
my-api-tests/
├── openapi.yml
├── kaktoos.yaml
│
├── environments/
│   ├── local.yml
│   └── staging.yml
│
└── scenarios/
    ├── customer/
    │   ├── create.yml
    │   └── update.yml
    │
    ├── account/
    │   └── opening.yml
    │
    └── payment/
        ├── successful-payment.yml
        └── insufficient-balance.yml
```

This repository can be versioned normally with Git.

```text
API change
    ↓
Update OpenAPI
    ↓
Update scenarios
    ↓
Git commit / Pull Request
    ↓
Kaktoos validate
    ↓
Kaktoos run
    ↓
Regression result
```

---

# Commands

## Validate

Validate the OpenAPI specification, environment, and scenario:

```bash
kaktoos validate
```

Inline scenarios can also be used:

```bash
kaktoos validate \
  --openapi openapi.yml \
  --env env.yml \
  --scenario-inline '...'
```

## Run

Execute API test scenarios:

```bash
kaktoos run
```

Enable response schema validation:

```bash
kaktoos run --schema-mode strict
```

## Impact

Analyze what a change touches and which workflows verify it:

```bash
kaktoos impact                                    # working tree
kaktoos impact --commit <sha>
kaktoos impact --base main --head feature/payment
kaktoos impact --pr 184                           # requires the gh CLI
kaktoos impact --format json --why
```

Run the workflows the plan selected:

```bash
kaktoos impact --verify --openapi openapi.yml --env env.yml
```

See [docs/impact.md](docs/impact.md).

## MCP

Expose Kaktoos to an AI coding agent:

```bash
kaktoos mcp
```

All tools are documented in [docs/mcp.md](docs/mcp.md).

## Serve

Run workflows in response to incoming webhooks:

```bash
kaktoos serve \
  --config webhooks.yml \
  --port 8080 \
  --host 0.0.0.0
```

## Version

Display the installed Kaktoos version:

```bash
kaktoos version
```

---

# Examples

See the [`examples/`](examples/) directory for complete working examples.

---

# Development

Run unit tests:

```bash
go test ./...
```

Run end-to-end tests:

```bash
go test -tags e2e ./e2e/...
```

Build:

```bash
go build ./cmd/kaktoos
```

Run static analysis:

```bash
go vet ./...
```

Format the source:

```bash
gofmt -w .
```

---

# Project Status

**Kaktoos v1.3.0**

Kaktoos currently focuses on:

- Scenario-based API verification
- OpenAPI-powered API execution
- Multi-step API workflows
- Response schema validation
- Read-after-write state verification with `always_run` cleanup
- Deterministic failure classification with redacted evidence, shared across CLI, JSON, JUnit, MCP, and the GitHub Action
- Git-native API testing
- AI coding agent integration through MCP
- CI/CD verification through GitHub Actions
- Engineering context and change impact analysis (`kaktoos impact`, MCP context tools)

The core engine works independently of AI and external services.

The next improvements will be driven by real-world usage and feedback.

If you use Kaktoos, we'd especially like to learn:

- What API workflows are you testing?
- Are you using Kaktoos manually, in CI, or through an AI coding agent?
- What problems did Kaktoos catch?
- What did your existing API testing workflow miss?
- Did the MCP workflow make AI-generated API integrations easier to verify?
- What are you currently using for API regression testing?
- What is difficult about your current approach?
- What would make Kaktoos more useful?

Open an issue or discussion on GitHub.

---

# Contributing

Contributions, bug reports, and ideas are welcome.

Please open an issue or pull request on GitHub.

---

# License

MIT License
