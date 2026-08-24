---
kind: feature-contracts
version: "1.0"
feature: B03-profiles
project: which-model-desktop
---

# B03-profiles — Contracts

## 1. Package and files

| File | Contents |
|---|---|
| `internal/service/profiles.go` | `ProfileService`, `(*Services).Profiles()`, List/Get/Save/Duplicate/Delete/ComplexityScale, `complexityScaleSlugs`, `validateComplexityScale`, init assertion |
| `internal/service/profiles_test.go` | table tests per §6 |

Package `internal/service` (B00 import boundary applies). Uses B02's `dtoWeights`, `engineWeights`, `engineProfile`, sentinels `errValidation`/`errBuiltinReadonly`/`errNotFound`/`errConflict`, and B11's stats aggregation. DTOs are D00 canon (`ProfileSummary`, `ProfileDetail`, `ProfileStats`) — not redefined here.

## 2. Exported API — `internal/service/profiles.go`

```go
package service

// ProfileService is the profiles facet of Services; the host registers it as
// a Wails service. Zero value unusable; obtain via (*Services).Profiles().
type ProfileService struct { s *Services }

// Profiles returns the profiles facet (shares the Services lock and config).
func (s *Services) Profiles() *ProfileService

// List merges the 11 built-ins from pick.Profiles (Builtin true, weights via
// dtoWeights, CoreShare per SPEC §2.2) with [profiles.*] customs, attaches
// Picks/LastUsed from B11 stats, and returns built-ins sorted alphabetically
// by slug followed by customs sorted alphabetically by slug (SPEC §2.4).
func (p *ProfileService) List(ctx context.Context) ([]ProfileSummary, error)

// Get returns one profile from the merged set; unknown slug -> errNotFound.
func (p *ProfileService) Get(ctx context.Context, slug string) (ProfileDetail, error)

// Save creates or replaces the custom profile [profiles.<slug>] after the
// fixed validation order of SPEC §2.6 (messages §5), persists atomically,
// and emits config:changed {"section":"profiles"}. Builtin/Picks/LastUsed
// on d are ignored.
func (p *ProfileService) Save(ctx context.Context, d ProfileDetail) error

// Duplicate copies slug (built-in or custom) to the first free of
// <slug>_copy, <slug>_copy_2, ... in the merged set; persists it as a custom
// (Builtin false, Name = new slug, Picks 0, LastUsed ""), emits
// config:changed {"section":"profiles"}, and returns the new detail.
// Unknown source slug -> errNotFound.
func (p *ProfileService) Duplicate(ctx context.Context, slug string) (ProfileDetail, error)

// Delete removes a custom profile. Unknown slug -> errNotFound; built-in
// slug -> errBuiltinReadonly. Emits config:changed {"section":"profiles"}.
func (p *ProfileService) Delete(ctx context.Context, slug string) error

// ComplexityScale returns a fresh copy of complexityScaleSlugs (never errors).
func (p *ProfileService) ComplexityScale() []string

// complexityScaleSlugs are the popover slider stops 0..4 (SPEC §2.9).
var complexityScaleSlugs = []string{
    "simple_action_execution",
    "simple_implementation",
    "balanced_implementation",
    "research",
    "planning",
}

// validateComplexityScale panics if any slug in scale is missing from
// profiles. Called from init() with (complexityScaleSlugs, pick.Profiles);
// exported to tests via the package-internal name only.
func validateComplexityScale(scale []string, profiles map[string]catalog.Profile)
```

`init()` in `profiles.go` runs `validateComplexityScale(complexityScaleSlugs, pick.Profiles)` — panic message: `complexity scale profile %q is not a built-in profile`.

## 3. Events

| Method | Event | Payload |
|---|---|---|
| Save, Duplicate, Delete (success) | `config:changed` | `{"section": "profiles"}` |
| List, Get, ComplexityScale; any failed mutation | — none — | |

Exactly one event per successful mutation (D00 §2.4), asserted via B02's `emitRecorder`.

## 4. Config keys touched

Reads and writes `[profiles.<slug>]` (`core_share`, `[profiles.<slug>.tier1]`, `[profiles.<slug>.tier2]`) — schema, defaults, and decode validation are owned by B01; B03 only supplies validated values (weights 1–5, 0-valued keys omitted; `core_share` multiple of 5 in 10..90).

## 5. Error messages (exact, checked in SPEC §2.6 order)

| # | Condition | Sentinel | Message |
|---|---|---|---|
| 1 | slug empty / bad grammar | `errValidation` | `profile slug %q must match [a-z0-9_]+` |
| 2 | slug is a built-in slug | `errBuiltinReadonly` | `profile %q is built-in and read-only` |
| 3 | name equals a built-in's name, slug differs | `errConflict` | `profile name %q conflicts with built-in profile %q` |
| 4 | CoreShare out of range / not step 5 | `errValidation` | `core_share %d must be between 10 and 90 in steps of 5` |
| 5 | weight > 5 | `errValidation` | from B02 `engineWeights` |
| 6 | `pick.ValidateProfile` failure | `errValidation` | engine `RankingError` message verbatim |
| — | Get/Duplicate/Delete unknown slug | `errNotFound` | `profile %q not found` |
| — | Delete built-in | `errBuiltinReadonly` | `profile %q is built-in and read-only` |

## 6. Test fixtures (`profiles_test.go`; helper `newTestServices` from B02)

1. **List canon**: default fixture → 11 built-ins alphabetical by slug (`balanced_implementation` first, `ui_ux` last), `Builtin` true, `CoreShare` matches each `Tier1Share` (e.g. `simple_implementation` = 80), all weights ints 1–5.
2. **Round-trip Save→List**: Save a custom (`my_profile`, CoreShare 65, full tier1, one tier2 key) → List shows it after built-ins with identical fields; config.toml on disk contains the section; recorder saw exactly one `config:changed{profiles}`.
3. **Builtin Save rejection**: Save with slug `planning` → `builtin_readonly`, message table row 2, no write, no event.
4. **Builtin name conflict**: Save slug `planning2`, Name `planning` → `conflict`, row 3.
5. **Validation order golden**: table over rows 1/4/5/6 (bad slug, CoreShare 63, weight 6, tier1 missing `cost` → engine message `tier 1 weights must include intelligence, cost, and speed (missing cost)`).
6. **Duplicate suffixing**: Duplicate `planning` → `planning_copy` (Builtin false, weights/CoreShare copied, Picks 0); Duplicate `planning` again → `planning_copy_2`; each emits one event.
7. **Delete**: custom deleted (section gone, one event); Delete `research` → `builtin_readonly`; Delete unknown → `not_found`; failures emit nothing.
8. **Stats attach**: fixture history.jsonl with 2 picks for `research` → List row carries `Picks: 2` and RFC3339 `LastUsed`; unpicked rows are `0`/`""`.
9. **Complexity-scale panic**: `ComplexityScale()` returns the 5 slugs in order and a mutated return does not affect the next call; `validateComplexityScale([]string{"nope"}, pick.Profiles)` panics with the §2 message (asserted via recover).

Verify: `go test ./internal/service/ -run TestProfile`.

## 7. External symbols referenced

| Symbol | Source |
|---|---|
| `pick.Profiles`, `pick.ValidateProfile` | `internal/pick/{profiles,profile}.go` |
| `catalog.Profile{Tier1Share, Tier2Share, Tier1Weights, Tier2Weights}` | `internal/catalog/types.go` |
| `dtoWeights`, `engineWeights`, `engineProfile`, sentinels, `newTestServices`, `emitRecorder` | B02 CONTRACTS |
| `[profiles.*]` TOML accessors | B01 CONTRACTS |
| profile stats aggregation (`ProfileStats`) | B11 CONTRACTS |
