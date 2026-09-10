# Kaktoos

> Independent verification for API integrations — including code written by AI coding agents.

Kaktoos verifies API integrations against **OpenAPI contracts and real API behavior**.

Use Kaktoos to:

- Run multi-step API workflows
- Verify responses against OpenAPI contracts
- Catch integration mistakes independently of application tests
- Expose Kaktoos to AI coding agents through MCP
- Run the same verification locally or in CI/CD
- Keep API verification scenarios version-controlled in Git

```text
AI coding agent / Developer
          │
          ▼
   API integration code
          │
          ▼
       Kaktoos
          │
          ├── Discover OpenAPI operations
          ├── Execute real API workflows
          ├── Validate responses against OpenAPI
          ├── Evaluate assertions
          └── Return structured failures
                    │
                    ▼
              Fix integration
                    │
                    ▼
               Run again
                    │
                    ▼
                  PASS
```

Kaktoos works entirely without AI. AI coding agents are an optional interface through the Model Context Protocol (MCP).

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

---

## Key Capabilities

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

Scenarios describe multi-step API workflows.

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

Kaktoos exposes three MCP tools over stdio.

| Tool | Purpose |
| --- | --- |
| `list_operations` | List the operations declared by an OpenAPI specification |
| `validate_scenario` | Validate a scenario before execution |
| `run_workflow` | Execute a scenario against a real API and return structured results |

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

The action:

1. Installs the requested Kaktoos release
2. Runs Kaktoos
3. Writes a per-step result table to the GitHub Actions job summary
4. Propagates Kaktoos's exit code
5. Maintains a single PR comment instead of creating a new comment on every run

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

JUnit behavior:

- Failed assertions are reported as `<failure>`
- Conditions evaluating to false are reported as `<failure>`
- Infrastructure problems such as network, timeout, variable, or template errors are reported as `<error>`

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

## MCP

Expose Kaktoos to an AI coding agent:

```bash
kaktoos mcp
```

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

**Kaktoos v1.1.0**

Kaktoos currently focuses on:

- Scenario-based API verification
- OpenAPI-powered API execution
- Multi-step API workflows
- Response schema validation
- Git-native API testing
- AI coding agent integration through MCP
- CI/CD verification through GitHub Actions

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
