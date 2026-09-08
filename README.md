# Kaktoos

> Test how your APIs work together.

**Kaktoos is a Git-native, scenario-based API regression testing CLI powered by OpenAPI.**

Define your API specification and multi-step test scenarios as version-controlled files, then validate and run them locally or in CI/CD.

```text
Create Customer
      ↓
Create Account
      ↓
Make Payment
      ↓
Verify Payment
```

**Git tracks what changed. Kaktoos tells you whether it still works.**

## Why Kaktoos?

Testing an individual API endpoint is easy.

Testing how multiple APIs work together is where things become complicated.

Kaktoos lets developers and QA engineers define repeatable API workflows as version-controlled scenarios.

### Git-native

Keep your API specification, scenarios, and environments in Git:

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

### OpenAPI-powered

Kaktoos uses your OpenAPI specification to resolve API operations.

Your OpenAPI specification remains the contract, while Kaktoos scenarios describe how those APIs should work together.

### CI/CD ready

Run the same scenarios locally or in your CI/CD pipeline.

A failed scenario produces a non-zero exit code, allowing your API regression tests to fail a CI job.

### No server required

Kaktoos runs as a standalone CLI. There is no Kaktoos server, database, or account required.

## How It Works

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
     ├── Evaluate assertions
     └── Report results
```

## Installation

Kaktoos is distributed as a standalone binary. You don't need Go installed.

### Download a release

Download the latest release for your operating system from:

https://github.com/KaktoosLabs/kaktoos/releases

Supported platforms:

- macOS ARM64
- macOS Intel
- Linux ARM64
- Linux AMD64
- Windows ARM64
- Windows AMD64

### Go developers

If you already have Go installed:

```bash
go install github.com/KaktoosLabs/kaktoos/cmd/kaktoos@v1.0.0
```

### Build from source

```bash
git clone https://github.com/KaktoosLabs/kaktoos.git
cd kaktoos
go build ./cmd/kaktoos
```

## Quick Start

### 1. Validate your test configuration

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

### 2. Run a scenario

```bash
kaktoos run \
  --openapi api-spec.yml \
  --env environments/local.yml \
  --scenario scenarios/test.yml
```

### 3. Check the version

```bash
kaktoos version
```

Example:

```text
Kaktoos v1.0.0
```

## Example

### OpenAPI Specification

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

### Environment

```yaml
name: local

base_url: http://localhost:8080

variables:
  user_type: individual

headers:
  Content-Type: application/json
```

### Scenario

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

## Configuration

### OpenAPI

Kaktoos uses an OpenAPI 3.x specification with `operationId` values for API operations.

The `operationId` is used by scenarios to identify the API operation to execute.

### Environment

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

### Scenario

Scenarios describe multi-step API workflows.

Each step can define:

- API operation
- request path parameters
- query parameters
- headers
- request body
- response value extraction
- assertions

Variables can be extracted from one response and used in subsequent steps.

## Assertions

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

## CI/CD

Kaktoos is designed to run in CI/CD pipelines.

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

### Exit codes

```text
0 = all scenarios passed
1 = assertion or execution failure
2 = configuration or load error
3 = unexpected error
```

## Commands

### Validate

Validate the OpenAPI specification, environment, and scenario:

```bash
kaktoos validate
```

### Run

Execute API test scenarios:

```bash
kaktoos run
```

### Version

Display the installed Kaktoos version:

```bash
kaktoos version
```

## Project Structure

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

## Examples

See the [`examples/`](examples/) directory for complete working examples.

## Development

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

## Project Status

**Kaktoos v1.0.0 — Initial Public Release**

Kaktoos is currently focused on scenario-based API regression testing using OpenAPI and Git.

The next improvements will be driven by real-world usage and feedback.

If you use Kaktoos, we'd love to hear:

- What API workflows are you testing?
- What do you currently use for API regression testing?
- What is difficult about your current approach?
- What would make Kaktoos more useful?

Open an issue or discussion on GitHub.

## Contributing

Contributions, bug reports, and ideas are welcome.

Please open an issue or pull request on GitHub.

## License

MIT License
