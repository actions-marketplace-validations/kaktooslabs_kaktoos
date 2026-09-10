# AI-agent verification demo

A five-minute, self-contained demonstration of what Kaktoos is for: an AI
coding agent writes an API integration, Kaktoos checks it against the OpenAPI
contract and the running API, and reports a failure specific enough to fix.

The bug in this demo is the most boring one there is — a field named `amount`
where the contract says `total`. An agent wrote the Orders API handler and the
checkout code that consumes it in the same session, so both spellings agree
with each other. The client runs. The server runs. Any unit test written
alongside them passes, because it was written against the same assumption.
Nothing in the codebase disagrees with itself, which is precisely why this
survives review.

The OpenAPI document disagrees. Kaktoos is the thing that reads it. It calls
the real endpoint, compares the real response to the declared schema, and
reports `required_field_missing` at `$.total` — plus `$.amount` as an
undeclared field, which together name both halves of the mistake. The fix is
one line; the point of the demo is who caught it, and that the catch was
specific enough to act on without a debugger.

## Architecture

```text
       openapi.yml ─────── the contract: `total` is required
            │
            │ read by
            ▼
        Kaktoos ──── GET /orders/123 ────▶  server/  (agent-written API)
            │                                   │
            │ ◀───── {"amount": 100, ...} ──────┘
            │
            ├── status 200?                    ✅
            ├── declared content type?         ✅
            ├── required fields present?       ❌  $.total missing
            └── undeclared fields?             ⚠️  $.amount
                        │
                        ▼
              structured failure
                        │
                        ▼
              fix the field name
                        │
                        ▼
                     PASS

  client/checkout.sh ──▶ server/   also reads `amount`, so it works fine
                                   and proves nothing
```

| File | Role |
| --- | --- |
| `openapi.yml` | The contract. `total` and `currency` are required. |
| `server/main.go` | The agent-written Orders API. Returns `amount` unless `ORDERS_FIXED=1`. |
| `client/checkout.sh` | The agent-written consumer. Reads `amount`, so it succeeds against the broken server. |
| `scenario.yml` | The workflow Kaktoos runs. Asserts only `status: 200` — the contract supplies the rest. |
| `environment.yml` | Points Kaktoos at `http://localhost:8090`. |

Note how little `scenario.yml` asserts. It never mentions `total`. Restating
the contract in the test would defeat the purpose — an agent that got the field
name wrong in the code would get it wrong in the assertion too. The schema
check comes from the OpenAPI document, independently.

## The agent workflow

```text
discover operation      list_operations → getOrder, GET /orders/{orderId}
       ↓
build integration       agent writes the handler and the client
       ↓
Kaktoos verification    kaktoos run --schema-mode strict
       ↓
structured failure      required_field_missing  $.total
       ↓                undeclared_fields       $.amount
fix                     rename amount → total
       ↓
verification passes     ✓ Scenario PASSED
```

## Run it

Requires Go and a Kaktoos binary. From this directory:

### 1. Start the (broken) Orders API

```bash
cd examples/ai-agent-verification
go run ./server
```

It prints which mode it is in and listens on `http://localhost:8090`. Leave it
running; use a second terminal for the rest.

### 2. Watch the integration appear to work

```bash
./client/checkout.sh
```

```text
raw response: {"amount":100,"currency":"USD","id":"123"}
checkout: charging 100 USD ✅
```

Green tick, correct number, real HTTP call. This is the state a lot of
agent-written integrations ship in.

### 3. Verify against the contract

```bash
kaktoos run \
  --openapi openapi.yml \
  --env environment.yml \
  --scenario scenario.yml \
  --schema-mode strict
```

```text
=== Scenario: Order lookup ===
  [FAILED] Get order
    Error: schema violation: property "total" is missing
✗ Scenario FAILED
```

Exit code `1`. For the machine-readable version an agent would consume, add
`--output-format json`:

```json
"schema_violations": [
  {
    "kind": "required_field_missing",
    "path": "$.total",
    "message": "property \"total\" is missing"
  }
],
"undeclared_fields": [
  "$.amount"
]
```

`required_field_missing` at `$.total` says what the contract wanted.
`$.amount` says what arrived instead. That pair is enough to write the fix
without reading the handler.

### 4. Fix it and re-verify

Restart the server in fixed mode (`Ctrl-C` the first one):

```bash
ORDERS_FIXED=1 go run ./server
```

Re-run the exact same Kaktoos command:

```bash
kaktoos run \
  --openapi openapi.yml \
  --env environment.yml \
  --scenario scenario.yml \
  --schema-mode strict
```

```text
=== Scenario: Order lookup ===
  [PASSED] Get order
✓ Scenario PASSED
```

Exit code `0`. Nothing about the scenario, the contract, or the command
changed — only the implementation.

`ORDERS_FIXED=1` stands in for the agent editing one line. In `server/main.go`
it selects `body["total"] = 100` over `body["amount"] = 100`; an env var keeps
the broken → fixed transition to a single keystroke on a screen recording.

## Running it from an AI coding agent

Kaktoos speaks the Model Context Protocol, so an agent can drive the whole loop
itself rather than being told to shell out. Add to your MCP client
configuration:

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

The agent then has three tools:

1. `list_operations` — read `openapi.yml`, find `getOrder` and its parameters.
2. `validate_scenario` — check a scenario before spending a network call on it
   (this catches a wrong `operation:` name immediately).
3. `run_workflow` — execute against the real API. Returns the same
   `schema_violations` shown above, per step, as structured data.

`run_workflow` defaults to `schema_mode: strict`, so an agent gets contract
enforcement without asking for it. The CLI defaults to `off` for backward
compatibility.

Kaktoos works entirely without any of this. MCP is an optional interface; the
CLI commands above are the whole product.

## Reproducing the full broken → fixed flow

Copy-paste, from the repository root:

```bash
cd examples/ai-agent-verification
go build -o /tmp/orders-api ./server     # build once so it can be stopped cleanly

# broken
/tmp/orders-api & pid=$! ; sleep 1
./client/checkout.sh                     # succeeds — proves nothing
kaktoos run --openapi openapi.yml --env environment.yml \
  --scenario scenario.yml --schema-mode strict ; echo "exit=$?"   # exit=1
kill $pid ; sleep 1

# fixed
ORDERS_FIXED=1 /tmp/orders-api & pid=$! ; sleep 1
kaktoos run --openapi openapi.yml --env environment.yml \
  --scenario scenario.yml --schema-mode strict ; echo "exit=$?"   # exit=0
kill $pid
```

Two details that will otherwise cost you the demo. Build the binary rather
than backgrounding `go run ./server`: killing a backgrounded `go run` stops
the wrapper but leaves the compiled child holding port 8090. And capture
`$!` rather than using `kill %1`, which needs job control and does nothing in
a non-interactive shell. Either way the second server fails to bind, and the
"fixed" run silently re-tests the old broken one — a green-looking failure.
Running `go run ./server` in its own terminal and stopping it with `Ctrl-C`
(as in the walkthrough above) has neither problem.

If you have not installed Kaktoos, substitute `go run ../../cmd/kaktoos` for
`kaktoos` in the commands above.

The demo is deterministic: fixed order id, fixed values, no clock or network
dependencies, no state carried between runs.
