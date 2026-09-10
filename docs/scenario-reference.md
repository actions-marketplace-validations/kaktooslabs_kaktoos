# Scenario file reference

Field-by-field reference for `scenario.yml`. Source of truth is the code —
`internal/scenario/types.go` (shape) and `internal/scenario/loader.go`
(validation) — this doc mirrors it. Unknown fields at the scenario or step
level are a load error, not a silent ignore.

## Top level

```yaml
name: Order lookup
tags: [orders, smoke]
workflow_timeout: 30s
idempotency_key: order-lookup-v1
steps: [...]
```

| Field | Type | Required | Notes |
| --- | --- | --- | --- |
| `name` | string | yes | |
| `tags` | []string | no | free-form labels |
| `workflow_timeout` | duration string (`"30s"`) | no | caps the whole scenario run |
| `idempotency_key` | string | no | dedupes repeat runs via `internal/idempotency` |
| `steps` | []Step | yes | executed in order |

## Step

Every step is either an **operation step** (calls a real endpoint) or a
**condition step** (checks a variable, no network call) — exactly one of
`operation` / `condition` must be set, never both, never neither.

```yaml
- name: Get order
  operation: getOrder
  request:
    path: {orderId: "123"}
    query: {expand: "items"}
    headers: {Authorization: "Bearer {{token}}"}
    body: '{"note": "{{note}}"}'
  extract:
    order_total: "$.total"
  assert:
    status: 200
    body:
      - path: "$.currency"
        equals: "USD"
  timeout: 5s
  retry:
    strategy: exponential_backoff
    max_attempts: 3
    initial_delay: 100ms
    max_delay: 2s
    backoff_multiplier: 2.0
    retry_on:
      status_codes: [502, 503]
      network_errors: true
```

| Field | Type | Notes |
| --- | --- | --- |
| `name` | string | shown in output |
| `operation` | string | OpenAPI `operationId`; resolved against the loaded spec |
| `condition` | string | see [Condition syntax](#condition-syntax); mutually exclusive with `operation` |
| `request` | RequestSpec | operation steps only |
| `extract` | map[var]jsonpath | pulls values out of the response body into named variables |
| `assert` | AssertSpec | |
| `timeout` | duration string | per-step; overrides no default |
| `retry` | RetryPolicy | not allowed on condition steps |

### `request`

| Field | Type | Notes |
| --- | --- | --- |
| `path` | map[string]string | fills `{orderId}`-style path params |
| `query` | map[string]string | query string params |
| `headers` | map[string]string | merged over environment headers |
| `body` | string | raw request body, usually JSON as a literal string |

Every value in `path`, `query`, `headers`, and `body` goes through
[variable substitution](#variable-substitution) before the request is built.

### `assert`

| Field | Type | Notes |
| --- | --- | --- |
| `status` | int | exact expected HTTP status |
| `body` | []BodyAssert | one or more body checks |

Each `body` entry:

| Field | Type | Notes |
| --- | --- | --- |
| `path` | string | JSONPath into the response body, e.g. `$.total` |
| `equals` | any | expected value (substituted first if a string) |
| `exists` | bool | path must resolve to something |
| `not_exists` | bool | path must not resolve |

Assertions only check what you write here — they do not enforce the OpenAPI
schema. Use `--schema-mode strict` (see root README) for that; keep the two
independent rather than restating the contract in assertions.

### `extract`

`map[variable_name]jsonpath`, evaluated against the response body. Values are
stored as strings (numbers/bools/objects are stringified) and read back later
with `{{variable_name}}`. A path with no match is a step failure.

### `retry`

| Field | Type | Notes |
| --- | --- | --- |
| `strategy` | `fixed_delay` \| `exponential_backoff` \| `linear_backoff` | required |
| `max_attempts` | int, 1–10 | required |
| `initial_delay` | duration, ≥10ms | required |
| `max_delay` | duration | optional cap |
| `backoff_multiplier` | float > 0 | required only for `exponential_backoff` |
| `retry_on.status_codes` | []int | statuses that trigger a retry |
| `retry_on.network_errors` | bool | retry on transport-level failures |

### Condition syntax

`condition: <lhs> <operator> [rhs]`, space-separated, 2 or 3 tokens.

- `$var` or `$.var` — read a variable from `extract`
- a bare literal or `'quoted'` string — used as-is

Operators:

| Operator | Arity | Notes |
| --- | --- | --- |
| `exists` | unary | `$var exists` |
| `not_exists` | unary | `$var not_exists` |
| `equals` / `not_equals` | binary | string comparison |
| `greater_than` / `less_than` / `greater_than_or_equal` / `less_than_or_equal` | binary | both sides parsed as float64 |

Example: `condition: $order_total greater_than 0`

### Variable substitution

`{{name}}` anywhere in `request.path/query/headers/body` or a condition's
operands. Single-pass, literal replacement — a substituted value is never
re-scanned for further `{{...}}`. Referencing an unset variable is a step
error naming the variable and step.

## What lives outside scenario.yml

- **Base URL, default headers, auth** → `environment.yml` (`--env`)
- **OpenAPI contract, response schema enforcement** → `openapi.yml` +
  `--schema-mode off|warn|strict`
- **Which operation a name resolves to** → the OpenAPI spec's `operationId`s

See the root [README.md](../README.md) for those and for `--scenario-inline`,
`kaktoos mcp`, and the GitHub Action.
