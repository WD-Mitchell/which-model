---
kind: feature-contracts
version: "1.0"
feature: B04-pick
project: which-model-desktop
---

# B04-pick — Contracts

## 1. Package and files

| File | Contents |
|---|---|
| `internal/service/pick.go` | `Rank`, `RecordPick`, `CatalogLine`, unexported availability/provider-resolution helpers |
| `internal/service/pick_test.go` | §5 tests |

Package `internal/service` (B00 import boundary applies). Uses B02's `engineProfile`, `engineWeights`, `round2`, `toErrorDTO`, cached scores rows + routes table; D00's `ParseRouteKey`/`FormatRouteKey`; B11's history append. MUST NOT import `pkg/whichmodel`.

## 2. Exported API

```go
package service

// Rank resolves the effective profile (SPEC §2.1), builds the availability
// set (SPEC §2.4), runs pick.Rank, and maps the top Holds candidates.
// Overrides non-nil => ephemeral: no persistence, no history, no event
// (D00 §2.2). Empty availability or *pick.NoCandidatesError =>
// RankResponse{Total: 0}, nil error (SPEC §2.5). Read-only; emits nothing.
func (s *Services) Rank(ctx context.Context, req RankRequest) (RankResponse, error)

// RecordPick appends one history line to <StateDir>/pick/history.jsonl and
// emits pick:recorded{profile_slug, route_key} (SPEC §2.10). routeKey must
// match the D00 route-key grammar; profileSlug must resolve (builtin or
// [profiles.*]). Write failure -> io_error, no event.
func (s *Services) RecordPick(ctx context.Context, profileSlug, routeKey string) error

// CatalogLine returns the popover catalog summary (SPEC §2.11). Read-only:
// never seeds harnesses, never emits.
func (s *Services) CatalogLine(ctx context.Context) (CatalogSummary, error)
```

DTOs (`RankRequest`, `RankedModel`, `RankResponse`, `CatalogSummary`) are D00 CONTRACTS §2 — referenced, never redefined.

## 3. Internal helpers (shape pinned for the B06/B09 cross-tests)

```go
// availableIdentities: routes table ∩ enabled providers − [routes.disabled]
// (B00 CONTRACTS §6.3), deduplicated, order-independent (set semantics).
func (s *Services) availableIdentities() []pick.Identity

// resolveRoute picks the highest-priority enabled, non-disabled route for
// one catalog (model, reasoning); ok == false never occurs for a candidate
// that passed availability (SPEC §2.7).
func (s *Services) resolveRoute(model, reasoning string) (r routing.Route, ok bool)
```

## 4. History entry JSON schema (normative for the line RecordPick writes)

The Go struct is re-declared in B11's `internal/service/history.go` (B00 §2.3) with a comment naming the source kept in sync by convention: `pkg/whichmodel/pick.go` `HistoryEntry`/`Evidence`. Field names are EXACTLY the CLI's:

```json
{
  "ulid": "01J...26-char ULID (github.com/oklog/ulid/v2)",
  "ts": "RFC3339 UTC, e.g. 2026-08-18T12:00:00Z",
  "profile": "<profileSlug>",
  "strategy": "gui",
  "candidate_id": "<routeKey, D00 grammar provider/model_id@reasoning>",
  "final_score": 0,
  "excluded_count": 0,
  "evidence": {
    "profile": "<profileSlug>",
    "score_inputs": {},
    "route_provenance": "user_declared",
    "excluded_candidates": []
  }
}
```

Fixed values for GUI entries: `strategy` always `"gui"`; `final_score` 0; `excluded_count` 0; `evidence` carries only its four always-serialized fields (`band`/`snapshot_age_seconds`/`confidence`/`last_verified` omitted — CLI omitempty semantics preserved). GUI entries are aggregation-grade (B11 reads `ts` + `profile`), not explain-grade. One `json.Marshal` line + `"\n"`, single append write, file `0600`, dir `0700`.

## 5. Test fixtures (`pick_test.go`; TDD-first per D00 process rules)

Fixture scores CSV (`WithScoresCSV`; header columns are the subset `pick.Tier1ScoreColumn` + `software_engineering_score` need):

```csv
model,reasoning,intelligence_index_score,cost_per_intelligence_index_task_usd_score,median_end_to_end_response_time_seconds_score,software_engineering_score
alpha,high,90,60,70,80
alpha,low,70,90,90,60
beta,high,85,70,75,
gamma,medium,90,60,70,80
```

Fixture routes table (`WithRoutes`): providers `claude` (priority 1) and `codex` (priority 2), both enabled in config; routes `claude/alpha-1 → (alpha, high)`, `claude/alpha-1 → (alpha, low)`, `claude/beta-1 → (beta, high)`, `codex/alpha-1x → (alpha, high)`, `codex/gamma-1 → (gamma, medium)`.

Fixture request: `ProfileSlug` of a custom profile with `CoreShare 60`, tier1 `{intelligence: 4, cost: 3, speed: 3}`, tier2 `{software_engineering: 5}`, `Holds 5`.

1. **Deterministic ranking (golden).** Expected `RankResponse` exactly — `Total 4`, candidates in this order (asserts the engine tie-break AND the no-tier-2 asymmetry AND provider priority):

| Rank | ModelName | Reasoning | Score | Provider | RouteKey |
|---|---|---|---|---|---|
| 1 | beta | high | 77.50 | claude | `claude/beta-1@high` |
| 2 | alpha | high | 77.00 | claude | `claude/alpha-1@high` |
| 3 | gamma | medium | 77.00 | codex | `codex/gamma-1@medium` |
| 4 | alpha | low | 73.20 | claude | `claude/alpha-1@low` |

   (beta keeps raw tier-1 77.50 with no tier-2 evidence; alpha@high vs gamma@medium tie on every score key and break on casefolded model asc; alpha@high resolves `claude` over `codex` by priority.) Run twice ⇒ identical responses (SPEC §2.9).
2. **Disabled route re-resolves provider.** With `[routes.disabled] claude = ["alpha-1@high"]`: alpha@high stays ranked (still available via codex) with `Provider "codex"`, `RouteKey "codex/alpha-1x@high"`; with BOTH providers' alpha@high routes disabled, alpha@high drops out and `Total` is 3.
3. **Empty availability.** All providers disabled (or routes table empty) ⇒ `RankResponse{Candidates: [], Total: 0}`, nil error, `pick.RankWithCategories` not reached (no `RankingError`).
4. **Overrides don't persist.** `Rank` with non-nil `Overrides` (and with an invalid override for the rejection path): read `config.toml` bytes before/after — byte-identical; `history.jsonl` absent/unchanged; recorder shows zero events. Invalid override (CoreShare 57; weight 6; tier2 key `"bogus"`) ⇒ `validation_failed`, checks hit in SPEC §2.1 order.
5. **Holds.** `Holds 0` uses `[gui].holds`; `Holds 3` truncates to 3 with `Total 4`; `Holds 4` ⇒ `validation_failed`.
6. **RecordPick appends + emits.** Two calls append exactly two lines; each unmarshals with the §4 field names (`ulid` 26 chars, `ts` RFC3339, `strategy "gui"`, `candidate_id` = the route key); recorder shows exactly one `pick:recorded` per call with `{profile_slug, route_key}`. Bad grammar (`"claude/x"`, `"a@b"`) ⇒ `validation_failed`, zero lines, zero events; unknown profile ⇒ `not_found`; unwritable state dir ⇒ `io_error`, zero events.
7. **CatalogLine.** Fixture ⇒ `{Models: 4, ProvidersOn: 2, Harnesses: 18}` (empty `[harnesses]` ⇒ builtin seed count) and `config.toml` bytes unchanged after the call; disabling codex ⇒ `ProvidersOn 1`.

## 6. Config keys, events, error codes

- **Config keys owned: none.** Reads `[profiles.*]`, `[providers.*]`, `[routes.disabled]`, `[harnesses.*]`, `[gui].holds` (owned by B01/B03/B06/B07/B10). Writes none.
- **Events:** `pick:recorded` only (D00 CONTRACTS §3), from `RecordPick` exclusively.
- **Error codes used:** `validation_failed`, `not_found`, `io_error` (D00 CONTRACTS §4). None added.

## 7. External symbols referenced

| Symbol | Source |
|---|---|
| `pick.RankWithCategories`, `pick.Identity`, `pick.Profiles`, `pick.CategoryNames`, `*pick.NoCandidatesError`, `*pick.RankingError` | `internal/pick` (`rank.go`, `availability.go`, `profiles.go`, `errors`) |
| `catalog.ScoreRow`, `catalog.Profile` | `internal/catalog/types.go` |
| `routing.Table`, `routing.Route` | `internal/routing` (loaded/cached by B02) |
| `engineProfile`, `engineWeights`, `round2`, `toErrorDTO`, `newTestServices`, `WithConfigTOML`, `WithScoresCSV`, `WithRoutes` | B02 (B00 CONTRACTS §4–5) |
| `ParseRouteKey`, `FormatRouteKey`, D00 DTOs | `internal/service/dto.go` (D00 §7) |
| History entry struct + append primitive | B11 `internal/service/history.go` (shape pinned in §4 until B11's contract lands) |

## Review correction — #185

Rank supplies exactly `pick.CategoryNames` union the current `[groups.*]` slugs to `RankWithCategories`, under `Services.mu.RLock`. Saved profiles and ephemeral overrides use the same vocabulary. Custom-group scoring, availability filtering, tie-breaks, and history behavior are unchanged. Regression: a configured group's composite can change the winner; absent group keys remain invalid. CLI `Rank` and `ValidateProfile` retain their static vocabulary.
