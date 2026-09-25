# Engineering context and change impact

**Understand before you change. Verify after you change.**

Kaktoos can answer the two questions that bracket a change:

- **Before:** "I'm about to add scheduled payment cancellation — what already
  exists in this system that touches that?"
- **After:** "This PR changed `payment-service/fee.go` — what could it affect,
  and which existing Kaktoos workflows should I run?"

```text
BEFORE CHANGE
  Feature/request → engineering context → related components, APIs,
  workflows, dependencies → the agent designs the change

AFTER CHANGE
  PR/commit → change analysis → impact analysis → relevant Kaktoos workflows
  → verification → evidence
```

This is not a knowledge base and not an inference engine. Every relationship
Kaktoos reports is traceable to a file, a config line, an OpenAPI operation, or
a scenario, and every one of them carries the reason it exists.

## Changed vs potential impact

Kaktoos separates two very different statements, and never conflates them:

| Section | What it means |
| --- | --- |
| **Changed** | Fact. These files are in the diff. |
| **Potential impact** | Reachability. These entities are connected to the changed files through declared or inferred relationships. This tells you **what to verify** — it is never a claim that something is broken. |

Ownership is reported separately from impact: CODEOWNERS tells you who to loop
in, not what the change affected. It is contextual information, not a guarantee
of who is responsible.

## `kaktoos.yaml`

Optional. Without it, Kaktoos falls back to convention (any directory holding
an `openapi.yml`/`openapi.yaml` is a service named after the directory), and
marks every relationship it derived that way as `inferred`.

```yaml
services:
  - name: payment-service
    paths: ["payment-service/**"]
    openapi: payment-service/openapi.yml
    scenarios: payment-service/scenarios/
    depends_on: [transaction-service]
```

| Field | Required | Meaning |
| --- | --- | --- |
| `name` | yes | Service identity; becomes `service:<name>` |
| `paths` | yes | Globs mapping files to this service |
| `openapi` | no | Spec whose operations this service exposes |
| `scenarios` | no | Directory of Kaktoos scenarios verifying it |
| `depends_on` | no | Other service names this one consumes |

Unknown top-level or per-service fields are a load error, matching the same
strictness scenario files use.

## `kaktoos impact`

```bash
kaktoos impact                                    # working tree
kaktoos impact --commit <sha>
kaktoos impact --base main --head feature/payment
kaktoos impact --pr 184                           # requires the gh CLI
kaktoos impact --repo ./monorepo --format json
kaktoos impact --why                              # show the relation chain
```

| Flag | Default | Meaning |
| --- | --- | --- |
| `--repo` | `.` | Repository root to analyze |
| `--commit` | | Analyze a single commit |
| `--base` / `--head` | | Analyze a range (used together) |
| `--pr` | | Analyze a GitHub PR by number (needs `gh`) |
| `--format` | `text` | `text` or `json` |
| `--why` | `false` | Print why each entity is potentially affected |
| `--verify` | `false` | Run the workflows the plan selected |
| `--openapi` / `--env` | | Required with `--verify` |

Only one of `--commit`, `--base/--head`, `--pr` may be given. Usage and config
errors exit `2`.

Example:

```text
Change: HEAD

Changed
  payment-service/fee.go

Potential impact (reachable from the change — not proof of a defect)
  operation    GET /payments/{id}
  operation    POST /payments
  service      payment-service
  workflow     payment-create

Owners
  @payments-team (payment-service/fee.go)

Verification (existing Kaktoos workflows)
  payment-create (payment-service/scenarios/payment-create.yml)

Related work (from branch/commit text)
  PROJ-42
```

With `--why`, each entry is followed by the chain that reached it:

```text
  operation    POST /payments
               via contains: matches paths glob payment-service/**
               via exposes: declared in payment-service/openapi.yml
```

`[inferred]` marks a relationship derived by convention rather than declared.

## Verifying what the change affected

`--verify` runs the scenarios the plan selected through the same engine as
`kaktoos run` — there is no second execution path, and no scenario is ever
generated:

```bash
kaktoos impact --verify \
  --openapi payment-service/openapi.yml \
  --env environments/local.yml
```

It adopts `run`'s exit code. When no existing workflow covers the potentially
affected behavior, it says so and exits `0` — that is a real answer, not a
failure.

## GitHub Action

Both inputs are opt-in and default to `false`; with them off, the action
behaves exactly as before.

```yaml
- uses: kaktooslabs/kaktoos@v1
  with:
    openapi: openapi.yml
    env: environments/ci.yml
    scenario: scenarios/payment.yml
    impact: "true"          # add a Changed/Potential/Owners/Verification summary
    impact-verify: "true"   # run only the workflows the plan selected
```

With `impact-verify: "true"` and nothing to verify, the job passes and says so.

## Related work and documents

Work items and documentation come from providers behind interfaces. Today the
shipped work-item provider extracts issue keys (`PROJ-42`) from the branch name
and commit/PR text — deterministic, no credentials. No documentation provider
ships yet, so those lookups report *unavailable* rather than implying no
documentation exists. Adding a Jira or Confluence provider later touches no
core code.

## See also

- [MCP tools](mcp.md) — the same core exposed to AI coding agents
- [Verification patterns](scenarios.md) — what a workflow should assert
- [Scenario file reference](scenario-reference.md)
