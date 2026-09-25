# Verification patterns

How to write scenarios that verify more than "the API answered". Field-by-field
syntax lives in [scenario-reference.md](scenario-reference.md).

## Contract verification

The response body is checked against your OpenAPI spec automatically — nothing
to declare in the scenario. Pick the mode on the command line:

```bash
kaktoos run --openapi openapi.yml --env env.yml --scenario scenario.yml --schema-mode strict
```

```yaml
name: contract
steps:
  - name: get order
    operation: getOrder
    request:
      path: {id: "42"}
    assert:
      status: 200
```

Under `--schema-mode strict`, a response that violates the declared schema —
wrong types, missing required fields, or a `200 text/html` login page where
JSON was declared — fails the step with category `contract_mismatch`, even
though the status assertion passed. Under `warn` the violations are printed
but never fail the step or mask the real failure cause.

## Read-after-write state verification

A `200` from a POST proves the API answered; it does not prove anything was
stored. Verify observable state by reading it back:

```yaml
name: order-state
steps:
  - name: create
    operation: createOrder
    request:
      body: {sku: "widget", qty: 1}
    assert:
      status: 200
    extract:
      order_id: $.id          # capture the id the API claims it created

  - name: read back
    operation: getOrder
    request:
      path: {id: "{{order_id}}"}
    assert:
      status: 200
      body:
        - path: $.sku
          equals: widget

  - name: cleanup
    operation: deleteOrder
    always_run: true          # runs even if "read back" failed
    request:
      path: {id: "{{order_id}}"}
    assert:
      status: 204
```

If the POST returns `200` but persists nothing, the read-back gets a `404`,
its `status: 200` assertion fails, and Kaktoos reports a structured
`assertion_failed` on the *read back* step — the failure is caught by
observable state, not by trusting the write's own response. The JSON output
carries the evidence (method, URL, status, redacted headers, bodies) for that
request.

## Cleanup with `always_run`

Normally every step after a failure is skipped. A step marked
`always_run: true` executes anyway, so test data gets deleted even when the
verification in the middle failed. Rules:

- Its result is recorded like any other step, but a passing cleanup can never
  turn a failed scenario back into a passing one.
- Only valid on operation steps, not `condition` steps.
- Sensitive headers (`Authorization`, `Cookie`, `X-Api-Key`, …) are redacted
  to `***` in all captured evidence.

## Failure categories

Every failed step carries exactly one `failure_category`, computed by the
engine and identical across text, JSON, JUnit, MCP, and the GitHub Action:

| Category | Meaning |
| --- | --- |
| `contract_mismatch` | Response violated the OpenAPI schema under `--schema-mode strict` (includes `200 text/html` fallbacks) |
| `assertion_failed` | A scenario `assert` or `condition` failed — including state-not-persisted read-backs |
| `auth_failure` | HTTP 401 or 403 |
| `rate_limited` | HTTP 429 |
| `server_error` | HTTP 5xx |
| `transport_failure` | No usable response (connection refused, DNS, body read error) |
| `timeout` | Step or workflow deadline exceeded |
| `config_error` | Scenario problem: unknown operation, unresolved variable, bad JSONPath, bad condition |

Precedence, highest first: timeout → transport → config → status-derived
(auth/rate-limit/5xx) → contract → assertion. So a 401 that also fails
`assert: {status: 200}` reports as `auth_failure` — the auth problem is the
cause, the failed assertion is the symptom.

## Verifying what a change affects

A scenario written here becomes one of the workflows `kaktoos impact` and the
`get_verification_plan` MCP tool select when the operations or service it
covers show up in a change's potential impact. See
[docs/impact.md](impact.md).
