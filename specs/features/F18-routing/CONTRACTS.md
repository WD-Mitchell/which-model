---
kind: feature-contracts
version: "1.0"
feature: F18-routing
project: which-model
---

# F18-routing — Contracts

## 1. Package and files

| File | Contents |
|---|---|
| `internal/routing/types.go` | `Route`, `Provenance` — canonical, verbatim from `specs/global/CONTRACTS.md §3.1` |
| `internal/routing/build.go` | `ModelEntry`, `UserDeclaredRoute`, `ProviderInput`, `Input`, `UnroutedModel`, `BuildResult`, `AmbiguityError`, `ProduceRoutes` |
| `internal/routing/windows.go` | `BindWindowIDs` |
| `internal/routing/store.go` | `Table`, `SaveTable`, `LoadTable`, `Table.Stale`, `Table.ProvenanceCounts` |

Import boundary (global CONTRACTS §8): `internal/routing` MAY import `internal/catalog/identity`, `internal/usage` (types only), stdlib, `encoding/json`, `os`, `path/filepath`, `crypto/sha256` (test-only), `time`. MUST NOT import `internal/pick`, `internal/config`, or any `internal/catalog/*` package other than `identity`.

## 2. Canonical types (verbatim from `specs/global/CONTRACTS.md §3.1`)

```go
package routing

type Provenance string

const (
    ProvenanceProviderLive Provenance = "provider_live"
    ProvenanceModelsDev    Provenance = "models_dev"
    ProvenanceUserDeclared Provenance = "user_declared"
)

type Route struct {
    Provider   string     `json:"provider"`
    ModelID    string     `json:"model_id"`
    Model      string     `json:"model"`
    Reasoning  string     `json:"reasoning"`
    WindowIDs  []string   `json:"window_ids"`
    Provenance Provenance `json:"provenance"`
}
```

## 3. Exported API — `internal/routing/build.go`

```go
// ModelEntry is one provider-native model, from the models.dev catalogue
// (F08) or a live provider enumeration.
type ModelEntry struct {
    ModelID   string   // provider-native id, e.g. "claude-opus-4-5-20251101"
    Name      string   // display name BEFORE cleaning (F07 CleanModelName applied by routing)
    Reasoning []string // declared effort levels (models.dev reasoning_options[].values); empty = non-reasoning model
}

// UserDeclaredRoute is a hand-authored route (routes.toml or `routes add`).
// Model/Reasoning are trusted operator input; no catalog match is required.
type UserDeclaredRoute struct {
    Provider  string
    ModelID   string
    Model     string   // catalog display name, already cleaned
    Reasoning string   // may be "default"
    WindowIDs []string // explicit gating windows; empty = derive via BindWindowIDs
}

type ProviderInput struct {
    Provider         string
    Kind             usage.Kind           // from the F11 descriptor
    ModelsDev        []ModelEntry         // models.dev catalogue records (F08)
    LiveModels       []ModelEntry         // live enumeration; nil = no credentialed enumeration exists for this provider
    UserDeclared     []UserDeclaredRoute
    ExcludedModelIDs []string             // providers.toml excluded_models
    Windows          []usage.WindowSpec   // descriptor windows (F11) for BindWindowIDs
}

type Input struct {
    Providers   []ProviderInput
    CatalogRows []identity.Identity      // every scores-CSV identity (Model already cleaned, Reasoning already collapsed)
    Degraded    bool                     // usage disabled at any level: live source skipped, one warning (SPEC §2.13)
}

// UnroutedModel is one provider-native model (or level) skipped because no
// catalog row matched. Surfaced, never silent (SPEC §2.7).
type UnroutedModel struct {
    Provider  string
    ModelID   string
    Name      string // cleaned catalog name
    Reasoning string // level that failed; "" when the whole model failed
    Reason    string // always "no_catalog_row"
}

type BuildResult struct {
    Routes   []Route
    Unrouted []UnroutedModel
    Warnings []string          // exact strings below
    Errors   map[string]error  // provider id -> hard error; nil when no provider errored
}

// ProduceRoutes derives the route table for every configured provider.
// Precedence per (Provider, ModelID): user_declared > provider_live > models_dev.
// Sources are unioned; excluded_models filters auto sources only; only
// KindSubscription and KindAPIKeyBilling providers are auto-derived (SPEC §2.4).
// When in.Degraded is true, LiveModels are ignored for every provider and
// exactly one degraded-source warning is emitted (SPEC §2.13).
// The returned error is non-nil iff len(Errors) > 0 and is the first
// *AmbiguityError in provider input order, then models.dev record order.
// On ambiguity for a provider, its auto-derived routes are absent from
// Routes; user-declared routes for the provider are kept (SPEC §2.8).
func ProduceRoutes(in Input) (BuildResult, error)

// AmbiguityError is the fail-loud result when an effort-less provider model
// matches multiple catalog identities (SPEC §2.8). Candidates lists EVERY
// matched catalog identity, in catalog order.
type AmbiguityError struct {
    Provider   string
    ModelID    string
    Name       string               // cleaned catalog name that matched
    Candidates []identity.Identity
}

func (e *AmbiguityError) Error() string
```

**Exact warning and error strings (golden-testable; no variation):**

| Where | String |
|---|---|
| degraded source (emitted exactly once per `ProduceRoutes` call, only when `Input.Degraded` is true and at least one provider is processed) | `live provider model lists unavailable; routes built from models-dev and user-declared sources only` |
| unrouted whole model | `unrouted provider model <provider>/<modelID> (<name>): no catalog row matches` |
| unrouted single level | `unrouted provider model <provider>/<modelID> (<name>, <level>): no catalog row matches` |
| duplicate user-declared entry (first wins) | `duplicate user-declared route for <provider>/<modelID>; keeping first` |
| `AmbiguityError.Error()` | `ambiguous route for <provider>/<modelID>: <name> matches catalog identities [(<model>, <reasoning>), (<model>, <reasoning>)] that declared effort levels cannot disambiguate; add a manual override in routes.toml` |

`AmbiguityError.Error()` formatting rule: `ambiguous route for %s/%s: %s matches catalog identities [%s] that declared effort levels cannot disambiguate; add a manual override in routes.toml`, where the identity list is each candidate rendered as `(<Model>, <Reasoning>)` joined by `", "`.

## 4. Exported API — `internal/routing/windows.go`

```go
// BindWindowIDs derives the gating windows for one route from the provider's
// descriptor windows (F11 usage.WindowSpec, file internal/usage/descriptor.go).
// Account-level windows (empty ModelScope) are included unconditionally; a
// model-scoped window is included when any ModelScope entry is a
// case-insensitive exact or substring match of modelID or model. A route
// matching zero scoped windows still gets the account-level windows; a route
// matching several gets all of them (annex-b §7.3).
// Result order: account-level windows first, then matched scoped windows,
// both in descriptor declaration order; duplicates removed (first occurrence
// wins).
func BindWindowIDs(providerWindows []usage.WindowSpec, modelID, model string) []string
```

## 5. Exported API — `internal/routing/store.go`

```go
const TableSchemaVersion = "1.0"

// Table is the persisted route table (routes.json, SPEC §2.11).
type Table struct {
    SchemaVersion string            `json:"schema_version"`
    ScoresHash    string            `json:"scores_sha256"` // lowercase hex sha256 of the scores-CSV bytes the table was built against
    RefreshedAt   map[string]string `json:"refreshed_at"`  // provider id -> RFC3339 timestamp of last successful derivation
    Routes        []Route           `json:"routes"`
}

// SaveTable writes the table atomically: CreateTemp in the target's
// directory, write, close, Rename over the target; parent directories are
// created as needed. Returns the first error encountered.
func SaveTable(path string, t Table) error

// LoadTable reads and decodes the table. A missing file yields an error for
// which os.IsNotExist(err) == true; any other read/decode failure returns a
// descriptive error (SPEC §3).
func LoadTable(path string) (Table, error)

// Stale reports whether the table was built against a scores CSV different
// from currentScoresHash (SPEC §2.12). Staleness is a warning, never an error.
func (t Table) Stale(currentScoresHash string) bool

// ProvenanceCounts tallies routes per provenance, for `routes verify`
// (annex-b §7.1a). Only provenance values present in the table appear.
func (t Table) ProvenanceCounts() map[Provenance]int
```

## 6. JSON shapes

`routes.json` (the only file F18 emits; path resolved by the caller to `<cache_dir>/routes.json` per `docs/plan/annex-d-cli-reference.md` §4.5):

```json
{
  "schema_version": "1.0",
  "scores_sha256": "1f2e…64-hex…",
  "refreshed_at": { "claude": "2026-08-07T17:03:12Z" },
  "routes": [
    {
      "provider": "claude",
      "model_id": "claude-opus-4-5-20251101",
      "model": "Claude Opus 5",
      "reasoning": "default",
      "window_ids": ["5h", "sevenDayOpus"],
      "provenance": "models_dev"
    }
  ]
}
```

## 7. Config file owned: `routes.toml`

| Key | Shape | Default | Meaning |
|---|---|---|---|
| `<provider>` (table per usage provider id) | `[<provider>]` with one key per provider-native model id | absent | auto-derivation input (SPEC §2.3) |
| `"<model_id>"` (quoted, since ids may contain dots) | `"model|reasoning"` identity string (annex-b §5.7 identity syntax) | — | catalog identity the provider-native id joins to |

```toml
# <config_dir>/routes.toml — hand-authored user-declared routes (SPEC §2.10)
[claude]
"claude-opus-4-5-20251101" = "Claude Opus 5|default"

[openrouter]
"openai/gpt-5" = "GPT-5|high"
```

The JSON array-of-objects form of the same identity syntax (`{"model","reasoning"}` objects or `["model","reasoning"]` pairs) is accepted as equivalent by the caller's parser (annex-b §5.7); F18 consumes the parsed `[]UserDeclaredRoute`, never the file. Explicit per-route windows arrive via `which-model routes add --window` (annex-d §2.6), passed through `UserDeclaredRoute.WindowIDs`; `routes.toml` itself carries no window syntax.

## 8. Flags and error codes

- **Flags owned:** none — `which-model routes` flags belong to F27.
- **Exit codes added:** none — F27 maps `AmbiguityError` (runtime error path) and staleness (warning) to its own exits.
- **Error types added:** `AmbiguityError` (satisfies `error`). `ProduceRoutes` never returns `AmbiguityError` for a degraded source set (SPEC §2.13).

## 9. External symbols referenced (cross-feature)

| Symbol | Source | Used by |
|---|---|---|
| `identity.CleanModelName(value string) string` | `specs/features/F07-identity/CONTRACTS.md §2`, package `internal/catalog/identity` | SPEC §2.6 name cleaning |
| `identity.Identity{Model, Reasoning string}` | same | `CatalogRows`, `AmbiguityError.Candidates` |
| `identity.IdentityKey(model, reasoning string) Identity` | same | SPEC §2.6 join equality |
| `identity.CollapseReasoning(level string) string` | same | SPEC §2.6 level collapse ("default"→"high") |
| `usage.Kind`, `KindSubscription`, `KindAPIKeyBilling`, `KindGateway`, `KindLocalTool` | `specs/global/CONTRACTS.md §1.3`, `specs/features/F11-usage-types/CONTRACTS.md`, file `internal/usage/types.go` | SPEC §2.4 eligibility |
| `usage.WindowSpec{ID, Label, Unit, Optional, ModelScope []string}` | `specs/features/F11-usage-types/CONTRACTS.md` (task F11-T3), file `internal/usage/descriptor.go` | `BindWindowIDs` |

## 10. Notes for consumers

- F27 (`which-model routes refresh`) resolves `<cache_dir>` and the scores-CSV hash via F01 config and F06 csvstore provenance, calls `ProduceRoutes`, persists via `SaveTable` per the SPEC §2.8/§3 rules, and renders `ProvenanceCounts` + `Unrouted` in `verify`/`list`.
- `internal/pick` (F20/F26) consumes `Table.Routes` as the availability set: a `(Model, Reasoning)` identity is available iff at least one route carries it (master plan §4.3); `routes verify` reports coverage so unrouted score rows are listed, never silently dropped (global SPEC §8 M3 done-when).
- F18 compiles under `-tags nousage` unchanged (SPEC §5).
