---
kind: feature-spec
version: "1.0"
feature: B00-backend
project: which-model-desktop
---

# B00-backend — Service Layer

## 1. Purpose

`internal/service` is the single programmatic surface the desktop shell (and, later, `which-model serve`) mounts over the engine. It wraps `internal/pick`, `internal/routing`, `internal/catalog/*`, `internal/usage/*`, and `internal/config` with typed, concurrency-safe, event-emitting methods returning the D00 DTOs. It also owns the new config sections (via B01) that give the GUI durable state the CLI never needed.

Inherits: `specs/global/*` (decimal discipline inside; layering; security), `specs/desktop/global/*` (DTOs, events, error codes, boundary rounding).

## 2. Behaviour

1. **Construction.** `service.New(paths config.Paths, cfg *config.Config, emit EmitFunc) (*Services, error)` builds the one `Services` value the host owns. `New` loads the catalog caches eagerly (scores CSV, benchmarks config, routes table) and fails fast with a typed error naming the missing path — it never degrades silently to an empty catalog.

2. **Locking.** One `sync.RWMutex` guards config + caches. Reads take RLock. Every write method: (a) validate inputs; (b) take Lock; (c) mutate a copy of the raw config document; (d) `config.MarshalTOML` → `config.AtomicWriteFile`; (e) swap in-memory state; (f) release; (g) emit exactly one event. A failed write leaves in-memory state untouched.

3. **No CLI wrappers.** Nothing in `internal/service` imports `pkg/whichmodel`. Where the CLI has needed behaviour (history entry shape, provider ordering), the struct/logic is re-declared here per the owning feature's contract, with a comment naming the CLI source file kept in sync by convention.

4. **DTO discipline.** Methods accept/return only D00 CONTRACTS §2 types plus Go builtins. `decimal.Decimal` never crosses the boundary; conversion (round 2dp scores, integer percents, int weights 0–5 ↔ decimal 1–5) happens inside this package at the last step.

5. **Error mapping.** Internal errors pass through `toErrorDTO(err)` at the boundary. The mapping table lives in B02's contract; sentinel error values/types declared per feature map to specific codes; everything else → `io_error` with a sanitised message.

6. **Events.** Emitted via the `EmitFunc` injected at construction. The service layer never blocks on emit; the host guarantees emit is non-blocking. Exactly-one-event-per-mutation (D00 §2.4) is enforced by convention and asserted in tests via a recording EmitFunc.

7. **Usage isolation.** Only B08's file imports `internal/usage/fetch`. Provider blank-imports live in the host (D00 §2.10), never here. When usage is disabled (config or backend "off"), usage-dependent fields degrade per each feature's contract (e.g. `ProviderInfo.Session = nil`, `LimitsLine = "usage off"`); no method fails outright for usage being off except `usage.Snapshots` (→ `usage_unavailable`).

8. **Paths.** All file locations derive from the injected `config.Paths` (`UserConfigFile`, `CacheDir`, `StateDir`); nothing in this package calls `os.UserHomeDir` or hardcodes a path. Test setups use `t.TempDir()` trees.

9. **Testing.** Table tests per feature file, mirroring `internal/config`'s style. The shared helper `newTestServices(t *testing.T, opts ...TestOption)` (owned by B02) builds a temp config/cache/state tree with fixture scores CSV (`internal/catalog/score/testdata/scores_golden.csv` copied), fixture `benchmarks.toml`/`providers.toml` (from `available-model-data-export/`), a synthetic routes table covering ≥2 providers × ≥2 models × ≥2 reasoning levels, and a recording EmitFunc; options override config TOML content and fixture files.

## 3. Error behaviour

- Validation failures name the offending field and value in the message; check order within a method is fixed and documented in the feature contract so messages are golden-testable.
- Concurrent mutation is serialised, never rejected: there is no optimistic-locking error code.
- Context cancellation propagates: long operations (usage fetch, derive) honour `ctx` and return `ctx.Err()` wrapped, mapped to `io_error` (fetch: `usage_unavailable`).

## 4. Decisions

| Decision | Value | Rationale |
|---|---|---|
| One Services struct, injected paths + emit | `New(paths, cfg, emit)` | Testable without Wails; `serve` can construct the same value |
| Whole-document config writes | copy raw doc → MarshalTOML → AtomicWriteFile | Preserves unknown keys; one durable write per mutation |
| Re-declare vs import CLI shapes | Re-declare (history entry) | `pkg/whichmodel` is unsafe to link and writer-shaped |
| Weight int mapping | DTO 0–5 ints; TOML stores 1–5; 0 deletes the key; decimal conversion at Rank time | Matches mockup semantics ("0 = ignored") and engine validation ((0,5] weights) |
| Eager catalog load, fail fast | `New` errors on missing scores CSV | The GUI must surface "run `which-model catalog refresh`" rather than an empty picker |

## 5. Files (area-level)

| File | Owner |
|---|---|
| `internal/service/service.go`, `dto.go`, `events.go`, `testhelp_test.go` | B02 |
| `internal/service/profiles.go` (+`_test`) | B03 |
| `internal/service/pick.go` (+`_test`) | B04 |
| `internal/service/catalog.go` (+`_test`) | B05 |
| `internal/service/providers.go` (+`_test`) | B06 |
| `internal/service/harness.go` (+`_test`) | B07 |
| `internal/service/usage.go` (+`_test`) | B08 |
| `internal/service/favourites.go` (+`_test`) | B09 |
| `internal/service/settings.go` (+`_test`) | B10 |
| `internal/service/history.go` (+`_test`) | B11 |
| `internal/config/gui.go` (+`_test`), `internal/config/write.go` (+`_test`) | B01 |
