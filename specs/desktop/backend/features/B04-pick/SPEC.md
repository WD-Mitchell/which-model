---
kind: feature-spec
version: "1.0"
feature: B04-pick
project: which-model-desktop
---

# B04-pick — PickService (Rank, RecordPick, CatalogLine)

## 1. Purpose

`internal/service/pick.go` is the popover's ranking surface: it resolves a profile (saved or ephemeral overrides), builds the live availability set from the routes table and provider config, runs the engine's `pick.RankWithCategories`, and maps the result to the D00 `RankResponse` with a route key per candidate. It also records explicit picks to `<StateDir>/pick/history.jsonl` (feeding B11's per-profile stats) and produces the popover's catalog line.

Depends on: B02 (Services, helpers, error mapping), B03 (profile resolution), B06 (provider/route config semantics), B11 (history append + stats). Inherits D00 + B00; DTOs are D00 CONTRACTS §2 verbatim.

## 2. Behaviour

1. **Profile resolution.** `Rank` resolves the effective profile first: (a) `RankRequest.Overrides` non-nil ⇒ the overrides ARE the profile — validated exactly as a B03 save EXCEPT slug/name rules (`Slug`/`Name` are ignored: any value, including empty, passes; no builtin-collision or grammar check); (b) otherwise the profile is loaded by `ProfileSlug` — builtins from `pick.Profiles` and customs from `[profiles.*]`, per B03's merge rule; unknown slug → `not_found`. Override validation checks, in order: `CoreShare` in 10..90 step 5; tier-1 keys ⊆ {intelligence, cost, speed} with at least one weight ≥ 1; every weight in 0..5 (0 = key ignored/dropped); tier-2 keys ⊆ `pick.CategoryNames` ∪ custom group slugs (B05). Failures → `validation_failed`.

2. **Ephemeral overrides (D00 §2.2).** A `Rank` call — with or without `Overrides` — persists NOTHING: no config write, no history line, no event. Overrides exist only for the duration of the call.

3. **Engine conversion.** The resolved DTO profile becomes a `catalog.Profile` via B02's `engineProfile`: `Tier1Share = CoreShare`, `Tier2Share = 100 − CoreShare` (percent values — `pick.RankWithCategories` combines as `tier·share÷100`), int weights 1..5 → `decimal.Decimal` via `engineWeights` (0-valued keys removed).

4. **Availability set (B00 CONTRACTS §6.3, verbatim).** `available []pick.Identity` is built from the routes table (`routing.LoadTable`, cached by B02): one `Identity{Model: route.Model, Reasoning: route.Reasoning}` per route whose provider is enabled (`providers.<id>.enabled == true`; absent ⇒ disabled) AND whose `model_id@reasoning` is not listed under `[routes.disabled].<provider>`. Duplicates (same catalog identity via several providers) are deduplicated.

5. **Empty availability is not an error.** When the set is empty, `Rank` returns `RankResponse{Candidates: [], Total: 0}` WITHOUT calling `pick.RankWithCategories` (the engine rejects a non-nil empty filter). The popover renders this as "Enable a provider" / "every provider is switched off" (mockup `pickName`/`pickMeta` empty-route branch). Likewise, a `*pick.NoCandidatesError` from the engine (all rows cut by tier-1 or availability filtering) maps to `RankResponse{Total: 0}`, not an error.

6. **Ranking and mapping.** `Rank` calls `pick.RankWithOptions(rows, profile, available, categories, options)` on the cached scores rows. The `Recommendation` is rank 1; `Alternatives` follow in engine order (ranks 2..n). `Candidates` is the first `Holds` entries; `Total = Result.CandidateCount` (pre-truncation count). Per candidate: `Score = round2(Total)` (the only place the 2dp boundary rounding happens); `Rank` is 1-based; `ModelName` = catalog model name; `Reasoning` from the engine row.

7. **Provider and route key per candidate.** Each candidate's `Provider` is the highest-priority (lowest `priority`, ties by id ascending — B00 §6.1) ENABLED provider that routes that exact `(model, reasoning)` and is not disabled for it under `[routes.disabled]` — the mockup's `results()` first-match rule. `ModelID` is that route's provider-native `routing.Route.ModelID`; `RouteKey = FormatRouteKey(Provider, ModelID, Reasoning)`. A candidate that survived §2.4 always resolves a provider by construction.

8. **Holds.** `RankRequest.Holds` 0 ⇒ use `[gui].holds`; otherwise it must be one of {3, 5, 10} → else `validation_failed`. Fewer candidates than Holds returns them all.

9. **Determinism.** Identical inputs (config, scores CSV, routes table, request) ⇒ byte-identical `RankResponse`, including ordering: the engine's 7-key tie-break (`rank.go` `rankLess`) is total and the service applies no re-sorting, no map-iteration-dependent ordering.

10. **RecordPick.** `RecordPick(ctx, profileSlug, routeKey)` validates `routeKey` grammar (`ParseRouteKey` → `validation_failed`) and that `profileSlug` resolves per §2.1(b) (→ `not_found`), then appends ONE JSONL line to `<StateDir>/pick/history.jsonl` (path from injected `config.Paths`; dir created `0700`, file `0600` append-create) using the CLI-compatible entry shape re-declared in B11's `history.go` (B00 §2.3; CONTRACTS §4), then emits exactly one `pick:recorded` event with `{"profile_slug", "route_key"}`. Unlike the CLI (which warns and swallows), a write failure IS returned (`io_error`) and no event is emitted. Callers: the frontend after a copy-mode launch, and B07's `Launch`.

11. **CatalogLine.** `CatalogLine(ctx)` returns `CatalogSummary{Models, ProvidersOn, Harnesses}`: `Models` = number of distinct `(model, reasoning)` scores rows (= `len(rows)`; `score.ParseScoresCSV` enforces uniqueness); `ProvidersOn` = count of enabled providers; `Harnesses` = harness count per B07's merge rule (`[harnesses.*]` entries when present, else the 4 builtin seeds) — read-only: `CatalogLine` never triggers B07's first-run seed write. Renders as the mockup's `catalogLine` ("N models · M providers on · K harnesses").

## 3. Error behaviour

- `Rank`: override validation → `validation_failed` (check order fixed in §2.1); unknown `ProfileSlug` → `not_found`; bad `Holds` → `validation_failed`; `pick.ValidateProfile`/`*pick.RankingError` → `validation_failed`; `*pick.NoCandidatesError` and empty availability → `RankResponse{Total: 0}`, nil error; anything else → `io_error`.
- `RecordPick`: bad route key → `validation_failed`; unknown profile → `not_found`; append failure → `io_error`, no event, no partial line (single `write` of one `\n`-terminated line).
- `CatalogLine`: cannot fail once `Services` is constructed (B00 §2.1 loads eagerly); returns cached state.
- Zero events on every read and every failure path (B00 §6.5).

## 4. Decisions

| Decision | Value | Rationale |
|---|---|---|
| Empty availability / no candidates | `RankResponse{Total: 0}`, nil error | The popover's "Enable a provider" state is a normal render, not a fault (mockup empty-route branch); engine's non-nil-empty-filter error is a CLI concern |
| Overrides validation scope | Save-equivalent minus slug/name rules | Ephemeral profiles have no identity; weight/share integrity still guards the engine precondition |
| Share conversion | `Tier1Share = CoreShare` (percent), not `CoreShare/100` | `pick.RankWithCategories` multiplies by share then divides by 100 (`rank.go` step 7) |
| Provider resolution | First enabled, non-disabled provider by priority order per candidate | Mockup `results()` `order.find(...)`; B00 §6.1/§6.3 make it deterministic |
| History entry shape | CLI `HistoryEntry` field names re-declared in B11; `strategy: "gui"`; `candidate_id` = D00 route key | One history file for CLI + GUI (B00 §2.3); route key keeps reasoning, which the CLI's `provider:model_id` form drops |
| History write failure | Returned as `io_error` (CLI swallows) | GUI surfaces a toast; silent loss of pick counts is invisible in a GUI |
| CatalogLine harness count | Config entries, else builtin seed count; never writes | A read method must not mutate (B00 §2.2 write discipline) |

## 5. Out of scope

- Launch/substitution and the `Launch` → `RecordPick` call chain — B07. Per-profile pick aggregation (`Picks`/`LastUsed`) and the append primitive — B11. Availability config mutation (`[routes.disabled]`, enable/priority) — B06. Profile persistence — B03. Frontend hold/carousel behaviour — U04/U05.

## Review correction — #185

Rank supplies exactly `pick.CategoryNames` union the current `[groups.*]` slugs to `RankWithCategories`, under `Services.mu.RLock`. Saved profiles and ephemeral overrides use the same vocabulary. Custom-group scoring, availability filtering, tie-breaks, and history behavior are unchanged. Regression: a configured group's composite can change the winner; absent group keys remain invalid. CLI `Rank` and `ValidateProfile` retain their static vocabulary.

## Incomplete benchmark recommendations (2026-09-05)

`gui.allow_incomplete_recommendations` / `GUISettings.allow_incomplete_recommendations` is a persisted boolean, default false. General displays **Allow recommendations with incomplete benchmarks**. Saving it emits the existing settings event and invalidates ranking immediately. The rank service passes it as `pick.RankOptions.AllowIncomplete`; enabling it uses available core scores, disabling it requires complete core scores. Catalog scores remain visible in either mode.

Both carousel and list show `Missing benchmark data: <axes>. Ranked using available scores.` for partial recommendations, using absent intelligence/cost/speed fields in that order; measured zero is present data. No RankedModel schema extension is needed. Tests must cover off→on→off persistence and ranking, blank speed preservation, and the warning in both layouts.


## Correction — Custom use-case groups (2026-09-05)

Saved and ephemeral use cases may weight configured custom groups, as intended
by B05. Save validates with `pick.ValidateProfileWithCategories`; Rank calls
`pick.RankWithOptions`, supplying canonical categories union config group slugs
and the saved partial-data policy. Unregistered categories
remain invalid. See `specs/features/F10-ranking/CONTRACTS.md` §Correction.
