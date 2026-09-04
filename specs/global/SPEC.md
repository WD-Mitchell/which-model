---
kind: global-spec
version: "1.0"
project: which-model
module: github.com/WD-Mitchell/which-model
binary: which-model
aliases: [wm, wmodel, whichm]
go_version: "1.23"
---

# which-model — Global Specification

## 1. Purpose

`which-model` is a Go CLI that answers: **given this task, which exact model and reasoning effort should I dispatch to, on which provider, given what allowance is left?**

It merges two prototypes:

1. `usage-allowance-checks/` — provider usage/allowance reporting (3 providers, Node)
2. `available-model-data-export/` — model scoring and ranking (Python pipeline + ranker)

The merge artifact is the **route** — `(provider, model_id) → (catalog_model, reasoning)` — which neither prototype has. Without it, usage and scores are two unrelated datasets.

## 2. Architecture layers

```
Layer 0: Foundation    — config, decimal, output, http, security
Layer 1a: Catalog      — csvstore, identity, collectors, scoring, ranking
Layer 1b: Usage        — types, credentials, cache, fetch, provider adapters
Layer 2: Routing       — provider ↔ model join
Layer 3: Selection     — bands, strategies, usage toggle
Layer 4: CLI           — cobra commands
Layer 5: Integration   — agent skills, hooks, publishing
```

**Dependency rule:** `pick → routing → {usage, catalog}`, never upward. `usage` and `catalog` do not know about each other; `routing` is the only place they meet.

## 3. Canonical Go packages

| Package | Layer | Purpose |
|---|---|---|
| `internal/config` | 0 | TOML config, env, flag resolution |
| `internal/decimal` | 0 | `shopspring/decimal` wrappers, `ROUND_HALF_UP` |
| `internal/output` | 0 | JSON/text/schema renderers |
| `internal/httpkit` | 0 | Shared HTTP: retries, redirect rejection, body bounding |
| `internal/security` | 0 | Token validation, bounded file I/O, canary harness |
| `internal/catalog/csvstore` | 1a | Atomic CSV read/write/merge/backup |
| `internal/catalog/identity` | 1a | Model name cleaning, identity keys, effort parsing |
| `internal/catalog/fetch` | 1a | AA v2, models.dev, AA page collectors |
| `internal/catalog/score` | 1a | Normalizer/Aggregator, category composites |
| `internal/pick` | 1a | Profiles, ranking, tier1/tier2 combination |
| `internal/usage` | 1b | Window/Snapshot types, Descriptor, registry |
| `internal/usage/credential` | 1b | File, env, keychain, cookie, CLI resolvers |
| `internal/usage/cache` | 1b | Per-provider TTL cache |
| `internal/usage/fetch` | 1b | Concurrent fan-out, partial failure |
| `internal/usage/provider/<id>` | 1b | One package per provider adapter |
| `internal/routing` | 2 | Route production, provenance, staleness |
| `internal/pick/band` | 3 | Pressure, band evaluation, gating |
| `internal/pick/strategy` | 3 | Six strategies + state file |
| `cmd/which-model` | 4 | Cobra command tree |
| `pkg/whichmodel` | — | Public library surface (future GUI) |

## 4. Build variants

| Tag | Effect |
|---|---|
| (default) | Full binary, all features |
| `nousage` | Usage subsystem compiled out; `internal/usage/**` replaced by stubs returning `ErrUsageCompiledOut` |

## 5. Exit codes (fixed, every command)

| Code | Meaning |
|---|---|
| 0 | Success |
| 1 | Runtime error |
| 2 | Argument/config error |
| 3 | No viable candidate after filtering |
| 4 | All eligible providers band-gated |
| 5 | Authentication required |

## 6. Security invariants

Inherited from `docs/plan/research/usage-allowance-checks-spec.md` §9. Non-negotiable:

1. Exact HTTPS endpoint allow-lists (no prefix/origin matching)
2. Redirects hard-fail (never followed)
3. Bodies bounded: 1 MiB credentials, 256 KiB responses (checked twice)
4. Opaque-token validation: length-bounded, single-line, no control chars
5. No credential material in any error, log, or output (canary-tested)
6. Permission warnings, never auto-remediation
7. Identity display opt-in (`--show-identity`) only
8. Configured-fallback origins require exact per-invocation trust
9. No background polling unless explicitly started

## 7. Testing strategy

- **TDD:** every task writes tests before implementation
- **Golden files:** `testdata/` fixtures for CSV, JSON response shapes, CLI output
- **Canary tokens:** every credential-touching path tested with a canary that must never appear in output
- **Table-driven:** Go `testing` with subtests, no external test framework
- **Build-matrix CI:** both default and `-tags nousage` on every change

## 8. Plan references

| Document | Path |
|---|---|
| Master plan | `docs/plan/README.md` |
| Provider matrix | `docs/plan/annex-a-provider-matrix.md` |
| Catalog port | `docs/plan/annex-b-catalog-port.md` |
| Agent integration | `docs/plan/annex-c-agent-integration.md` |
| CLI reference | `docs/plan/annex-d-cli-reference.md` |
| Usage checker spec | `docs/plan/research/usage-allowance-checks-spec.md` |
| Pipeline spec | `docs/plan/research/model-data-pipeline-spec.md` |
| Provider survey | `docs/plan/research/codexbar-provider-survey.md` |

## 9. npm fallback distribution (#161)

When the platform optional package is unavailable, postinstall may fetch the version-matched release binary. Decode only `checksums.txt` as UTF-8 and parse sha256sum LF/CRLF records (including the binary marker). Hash and write the binary as unchanged bytes, with executable mode on Unix. Missing, malformed or mismatched checksums and download failures must leave no installed fallback and warn without failing npm installation. Existing optional binaries and `WHICH_MODEL_SKIP_DOWNLOAD=1` perform no fallback requests. Validate with `node --test npm/which-model/install.test.js` and the existing npm smoke test.
