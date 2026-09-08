---
kind: feature-contracts
version: "1.0"
feature: B05-catalog-groups
project: which-model-desktop
---

# B05-catalog-groups — Contracts

## 1. Package and files

| File | Contents |
|---|---|
| `internal/service/catalog.go` | all §2 methods + unexported helpers (§3) |
| `internal/service/catalog_test.go` | §7 tests |

DTOs (`GroupSummary`, `GroupDetail`, `GroupBenchmark`, `BenchmarkDetail`, `BenchRow`, `CatalogModelDetail`, `CatalogModelProvider`) are D00 CONTRACTS §2 — referenced, never redefined. Extra imports beyond B00's boundary: `internal/catalog/identity` (BenchmarkKey, CleanModelName), `internal/decimal` (RoundHalfUp).

## 2. Exported API (methods on `*Services`)

```go
package service

// Benchmarks returns the full catalogue (SPEC §2.1), sorted ascending.
func (s *Services) Benchmarks(ctx context.Context) ([]string, error)

// BenchmarkDetail returns note, containing groups, and tested rows with
// Value (raw) + Norm (Value/max*100), Norm desc (SPEC §2.4).
func (s *Services) BenchmarkDetail(ctx context.Context, name string) (BenchmarkDetail, error)

// ModelDetail returns every benchmark (model, reasoning) reports. Unknown
// or untested pairs return empty Rows, not not_found.
func (s *Services) ModelDetail(ctx context.Context, model, reasoning string) (ModelScoreDetail, error)

// Models returns one CatalogModel per distinct scored or discovered display name,
// sorted name ascending (SPEC §2.14).
func (s *Services) Models(ctx context.Context) ([]CatalogModel, error)

// Model returns the full model card for a display name or model ID:
// identity, reasoning levels, scores (nil for unscored provider models),
// in_catalog inclusion status, and enabled provider rows with pricing (SPEC §2.15).
func (s *Services) Model(ctx context.Context, name string) (CatalogModelDetail, error)
// Groups returns builtins (benchmarks.toml order) then customs (slug asc);
// InProfiles counts builtin+custom profiles weighting the slug (SPEC §2.5).
func (s *Services) Groups(ctx context.Context) ([]GroupSummary, error)

// GroupDetail returns the full catalogue with On membership + coverage
// (SPEC §2.6).
func (s *Services) GroupDetail(ctx context.Context, slug string) (GroupDetail, error)

// SaveGroup replaces a custom group's member list, optionally renaming it
// (rewriting custom-profile tier2 keys in the same atomic write), then runs
// the re-derive pipeline (SPEC §2.8, §2.10, §2.12).
func (s *Services) SaveGroup(ctx context.Context, slug string, benchmarks []string, renameTo string) error

// DuplicateGroup copies any group (builtin or custom) to <slug>_copy[_N]
// and runs the pipeline (SPEC §2.9).
func (s *Services) DuplicateGroup(ctx context.Context, slug string) (GroupDetail, error)

// DeleteGroup removes a custom group and strips its tier2 key from custom
// profiles in the same write, then runs the pipeline (SPEC §2.9).
func (s *Services) DeleteGroup(ctx context.Context, slug string) error
```

## 3. Unexported helpers (shape fixed for testability)

```go
// sanitizeGroupSlug: trim, lowercase, [^a-z0-9]+ -> "_", strip leading "_".
// "" means "no rename" (SPEC §2.7).
func sanitizeGroupSlug(raw string) string

// rawBenchmarkValues parses the scores CSV's raw benchmark columns
// ("benchmark: <name>", the non-_score member of each pair) into
// benchmark -> (model,reasoning) -> raw value (SPEC §2.2).
func rawBenchmarkValues(scoresCSV []byte) (map[string]map[modelKey]sdecimal.Decimal, error)

// mergedBenchmarksTOML appends the custom groups to the builtin document
// (SPEC §2.10b; shape in §4).
func mergedBenchmarksTOML(builtin []byte, customs map[string][]string) ([]byte, error)

// overlayCustomCategories sets Categories[slug] per SPEC §2.11 (mean of
// present member scores, dedup by identity.BenchmarkKey, min 1, RoundHalfUp 0dp).
func overlayCustomCategories(rows []catalog.ScoreRow, customs map[string][]string)

// rederive runs SPEC §2.10 (a)-(e); returns the failing path on error.
func (s *Services) rederive(ctx context.Context) error
```

## 4. Merged-TOML shape (SPEC §2.10b)

Builtin document verbatim, then per custom group (slug asc, deterministic bytes):

```toml
# ... builtin benchmarks.toml content, with benchmark_selection.groups
# extended: groups = [ ...11 builtin ids..., "my_group" ]

[benchmark_groups.my_group]
benchmarks = [
  "SWE-Bench Verified",
  "Terminal-Bench",
]
```

Custom slugs are absent from `score.CategoryMinimumEvidence` (min lookup ⇒ 0), so the merge parses cleanly under strict `ParseBenchmarkConfig`; derived output columns are unchanged (fixed 12 categories — hence the SPEC §2.11 overlay).

## 5. Config keys and paths (read/written; schema owned by B01)

| Key / path | Use |
|---|---|
| `[groups.<slug>] benchmarks = [...]` | custom groups; slug grammar `[a-z0-9_]+` |
| `[profiles.<slug>.tier2]` | rename rewrites / delete strips group keys |
| `<CacheDir>/catalog/available_model_raw_values.csv` | Derive input (read) |
| `<CacheDir>/catalog/available_model_scores.csv` | Derive output (`csvstore.WriteAtomicBytes`) + catalogue/coverage source |
| resolved `benchmarks.toml` (B02 cache) | builtin groups + merge base |

Events: success ⇒ `catalog:changed` `{}`; derive-phase failure ⇒ `config:changed` `{"section":"groups"}` (SPEC §2.12). Reads emit nothing.

## 6. Exact strings

| Where | String |
|---|---|
| `BenchmarkDetail.Note` (always, v1) | `Carried in the model data export. No description recorded for this benchmark yet.` |
| SaveGroup/DeleteGroup on builtin slug | `group %q is built-in and read-only` → `builtin_readonly` |
| unknown group / benchmark name lookup | `no group %q` / `no benchmark %q` → `not_found` |
| unknown model name lookup | `no model %q` → `not_found` |
| SaveGroup unknown member (first offender) | `unknown benchmark %q in group %q` → `validation_failed` |
| rename collision | `group %q already exists` → `conflict` |
| derive-phase failure | message contains the failing path → `io_error` |

## 7. Test fixtures (`catalog_test.go`; helper `newTestServices` from B02)

| Test | Assertion |
|---|---|
| `TestBenchmarksUnion` | list = sorted union of fixture benchmarks.toml names, `[groups.*]` members, and score-row benchmark keys; deduped |
| `TestCoverageGolden` | golden `Covered`/`CoverageTotal` per benchmark computed from the fixture scores CSV raw cells (counts stated literally in the test) |
| `TestBenchmarkDetail` | fallback Note verbatim; Groups slugs correct; Rows tested-only, Norm = value/max×100 (max row ⇒ 100), Norm desc; unknown name → `not_found` |
| `TestGroupsList` | builtins in benchmarks.toml declared order with `Builtin:true`; customs after, slug asc; `InProfiles` counts builtin + custom profiles with weight > 0 |
| `TestSaveGroupDeriveRankChanges` | save custom group + weight it in a custom profile ⇒ scores CSV rewritten (content/mtime differs), `Categories[slug]` overlaid, B04 Rank order changes vs before; exactly one `catalog:changed` |
| `TestRenameRewritesProfileWeights` | `renameTo` sanitised (`"My Group! "` → `my_group`); tier2 key moved in every referencing custom profile in the same config write; collision → `conflict`, no write, no event |
| `TestDeleteStripsWeights` | `[groups.<slug>]` gone; tier2 key stripped from custom profiles; re-derive ran; one `catalog:changed` |
| `TestCatalogModels` | golden scores CSV → 3 models (Claude Opus 5, GPT-5.6 Sol, Kimi K2.7 Code); Opus has id `claude-opus-5`, intel 100, provider_count 1; Kimi has empty id and provider_count 0 |
| `TestCatalogModelCard` | scored model lookup returns full scores, identity, enabled providers, and `InCatalog: true`; unknown name returns `not_found` |
| `TestCatalogModelCardUnscoredProviderModel` | unscored provider model returns synthesized `CatalogModelDetail` with `InCatalog: false`, `Intelligence/Cost/Speed: nil`, enabled providers, and models.dev pricing; lookup by `ModelID` resolves |
| `TestCatalogModelCardUnscoredDisabledProvider` | unscored model on disabled provider returns `InCatalog: false`, `ProviderCount: 0`, and empty providers |
| `TestCatalogModelCardLookupByModelID` | scored model lookup by its model ID resolves with full scores and `InCatalog: true` |
| `TestCatalogModelCardProviderIDResolvesScoredModel` | provider model ID whose name matches a scored model directly resolves to that scored model with `InCatalog: true` and full scores |
| `TestCatalogModelCardDisabledLevelStillExposed` | a level disabled via `[routes.disabled]` remains in the provider row's `Reasoning` and `RouteKeys` (SPEC §2.15: exposure, not enablement, governs levels); provider-level enablement is still filtered |
| `TestCatalogModelsProviderListingContributes` | an ENABLED provider's models.dev listing with no route raises `ProviderCount` 0→1 and sets `ModelID` (SPEC §2.14 source (b)) |
| `TestCatalogModelsDisabledProviderListingIgnored` | the same listing under a DISABLED provider contributes nothing; `ProviderCount` stays 0 and `ModelID` empty |
| `TestCatalogModelCardSharedModelIDNoCrossProviderLeak` | two unrelated models sharing a generic provider id (`default`) do not cross-join: the unscored card keeps one provider row and the scored neighbour is unaffected |
| `TestCatalogModelCardTwoUnscoredShareModelID` | two UNSCORED models sharing `default` across providers keep separate cards in both directions; provider scoping, not a scored-name heuristic, arbitrates |
| `TestBuiltinMutationRejected` | SaveGroup/DeleteGroup on a builtin slug → `builtin_readonly`; config untouched; zero events; DuplicateGroup on the same slug succeeds as `<slug>_copy` |
| `TestDeriveFailurePersistsGroup` | raw CSV removed: SaveGroup → `io_error` naming the raw path; `[groups.<slug>]` persisted; catalog cache unchanged; `config:changed` emitted, no `catalog:changed` |

Verify: `go test ./internal/service/ -run 'TestBenchmark|TestCoverage|TestGroups|TestSaveGroup|TestRename|TestDelete|TestBuiltin|TestDerive|TestCatalogModel'`.

## 8. External symbols referenced

| Symbol | Source |
|---|---|
| `score.ParseBenchmarkConfig`, `score.Derive`, `score.DefaultNormalizer/DefaultAggregator`, `score.CategoryMinimumEvidence` | `internal/catalog/score` |
| `csvstore.WriteAtomicBytes`, `csvstore.BenchmarkColumnPrefix` | `internal/catalog/csvstore` |
| `catalog.ScoreRow` (`Benchmarks`, `Categories`) | `internal/catalog` |
| `identity.BenchmarkKey` | `internal/catalog/identity` |
| `wdecimal.RoundHalfUp` | `internal/decimal` |
| `pick.Profiles`, `pick.CategoryNames` | `internal/pick` (InProfiles, overlay namespace) |

## New-model discovery regressions

- `TestCatalogListsUnbenchmarkedModelsImmediately`: Codex GPT-6-Astra and models.dev GLM-5.3 appear with nil scores and correct makers; hidden Codex entries stay hidden; editing the local model cache is reflected on the next read without a restart.
- `TestCatalogDiscoveryKeepsUnbenchmarkedLiveModels`: an enabled CLI's unscored model survives refresh in the provider inventory and appears in Models.
- `TestMakerUsesCompaniesNotUnknownModelNames`: GLM variants and provider-qualified ids map to Z.AI; unknown model names map to Other.
- Correction to `TestCatalogModels`: golden fixture now lists five identities (the three scored models plus route-only Claude Sonnet 4 and GPT-4.2); the provider-count filter retains four. Top-scored-effort assertions are unchanged.


## Correction — Custom use-case groups (2026-09-05)

Saved and ephemeral use cases may weight configured custom groups, as intended
by B05. Save validates with `pick.ValidateProfileWithCategories`; Rank calls
`pick.RankWithOptions`, supplying canonical categories union config group slugs
and the saved partial-data policy. Unregistered categories
remain invalid. See `specs/features/F10-ranking/CONTRACTS.md` §Correction.
