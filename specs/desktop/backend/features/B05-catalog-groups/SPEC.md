---
kind: feature-spec
version: "1.0"
feature: B05-catalog-groups
project: which-model-desktop
---

# B05-catalog-groups — CatalogService (Benchmarks & Benchmark Groups)

## 1. Purpose

`CatalogService` (methods on `*Services` in `internal/service/catalog.go`) backs the Settings "Benchmark groups" page and its two detail views (group detail, benchmark detail): the benchmark catalogue with per-benchmark coverage, the merged builtin+custom group list, and the custom-group mutations (save/rename, duplicate, delete) that persist `[groups.*]`, rewrite dependent profile weights, re-derive the scores CSV, and refresh the in-memory catalog.

Depends on: B02 (Services, caches, error mapper, test helper). Inherits D00, B00.

## 2. Behaviour

1. **Full benchmark catalogue.** The catalogue is the union, deduplicated by exact display name, of: (a) every benchmark listed in the parsed benchmarks config (`score.ParseBenchmarkConfig` over the resolved `benchmarks.toml`) — all `[benchmark_groups.*]` tables plus `benchmark_selection.benchmarks`, alias-resolved to canonical display names; (b) every member of every `[groups.*]` custom group; (c) every key of every cached `catalog.ScoreRow.Benchmarks` map. `Benchmarks(ctx)` returns it sorted ascending (`sort.Strings`, byte order — mirrors the mockup's `ALL_BENCH` ordering).

2. **Raw benchmark values.** `ScoreRow.Benchmarks` holds derived `_score` values only; the scores CSV additionally carries each benchmark's RAW column (`benchmark: <name>` preceding `benchmark: <name>_score`). B05 parses those raw cells from the cached scores CSV bytes into an unexported per-benchmark map. A row is **tested** for a benchmark iff its raw cell is non-blank.

3. **Coverage.** For a benchmark: `Covered` = count of (model, reasoning) rows tested per §2.2; `CoverageTotal` = total (model, reasoning) rows in the cached scores CSV. (Mockup: `coverage(b)` → "n / total" and percent.)

4. **BenchmarkDetail.** Unknown name (not in §2.1 catalogue) → `not_found`. `Note`: `benchmarks.toml` carries no per-benchmark metadata (verified — group membership lists only), so `Note` is ALWAYS the exact fallback string `Carried in the model data export. No description recorded for this benchmark yet.` (mockup `BENCH_NOTE` fallback; the field stays so richer export metadata can populate it later without a contract change). `Groups` = slugs of every group (builtin then custom, in §2.5 list order) whose member list contains the name. `Rows` = tested rows only: `Value` = raw cell as float64, `Norm` = `Value / max(Value over tested rows) × 100` rounded half-up to an integer-valued float; sorted `Norm` descending, ties by model asc then reasoning asc.

5. **Groups list.** Builtins come from the parsed benchmarks config: one `GroupSummary{Builtin: true}` per `benchmark_selection.groups` entry, in declared file order (the real builtin source — never hardcoded). Customs come from `[groups.*]`, appended after builtins sorted by slug. `BenchmarkCount` = distinct member names. `InProfiles` = count of profiles — builtin `pick.Profiles` AND `[profiles.*]` customs — whose tier-2 weight for the group slug is > 0 (mockup `grSummary`).

6. **GroupDetail.** Unknown slug → `not_found`. `Benchmarks` = the FULL catalogue (§2.1) sorted ascending, each entry with `On` = membership in this group and coverage per §2.3. (The UI applies its own on-first ordering and filtering.)

7. **Slug sanitisation** (mockup `onGrRename`, exactly): trim → lowercase → replace every `[^a-z0-9]+` run with `_` → strip leading `_`. Empty result ⇒ "no rename requested". The sanitised result is what is validated and stored.

8. **SaveGroup(ctx, slug, benchmarks, renameTo).** Customs only. Fixed check order: (1) slug names a builtin → `builtin_readonly`; (2) slug not a custom group → `not_found`; (3) every entry of `benchmarks` is in the §2.1 catalogue, else `validation_failed` naming the first unknown entry; (4) `renameTo` sanitised per §2.7 — empty or equal to `slug` ⇒ no rename; otherwise a collision with ANY existing group slug (builtin or custom) → `conflict`. Then one atomic config write replacing the group's member list under its (possibly new) slug and, on rename, rewriting the tier-2 weight key `slug → newSlug` inside every `[profiles.<p>.tier2]` that carries it. Builtin profiles live in code and only weight engine categories — they never reference custom slugs, so no builtin is ever touched. Then the re-derive pipeline (§2.10).

9. **DuplicateGroup / DeleteGroup.** `DuplicateGroup(slug)` works on builtin OR custom source: new custom slug `<slug>_copy`, then `<slug>_copy_2`, `_copy_3`… first free; copies the member list; persists and runs §2.10; returns the new `GroupDetail`. `DeleteGroup(slug)`: builtin → `builtin_readonly`; unknown → `not_found`; removes `[groups.<slug>]` AND deletes the tier-2 key `<slug>` from every custom profile that carries it, in the same atomic write; then §2.10.

10. **Re-derive pipeline** (runs after every successful group persist, as one operation): (a) read raw CSV bytes from `<CacheDir>/catalog/available_model_raw_values.csv`; (b) build merged TOML (CONTRACTS §4): the resolved `benchmarks.toml` document with every remaining custom group appended as a `[benchmark_groups.<slug>]` table and its slug appended to `benchmark_selection.groups`; (c) `score.Derive(rawCSV, mergedTOML, score.DefaultNormalizer(), score.DefaultAggregator())`; (d) `csvstore.WriteAtomicBytes(<CacheDir>/catalog/available_model_scores.csv, derived)`; (e) reload the catalog caches (score rows, raw benchmark values §2.2, overlay §2.11); (f) emit `catalog:changed` (the one event for the whole mutation).

11. **Custom-group category overlay.** `score.Derive` emits only the fixed 12 category columns, so custom-group composites never appear as CSV columns. At every catalog (re)load, B05 overlays: for each custom group and each `ScoreRow`, `Categories[slug]` = mean of the member benchmarks' derived scores present in `row.Benchmarks` (deduped by `identity.BenchmarkKey`, minimum 1 populated evidence, `RoundHalfUp` 0dp — the `score.CategoryScores` arithmetic); absent when no member scored. This is what makes tier-2 weights on custom slugs effective in B04's Rank; profile validation (B03) accepts `pick.CategoryNames ∪ customGroupSlugs`.

12. **Derive-failure semantics.** If any step of §2.10 (a)–(d) fails — including a missing raw CSV — the group mutation is ALREADY persisted and stays persisted: the method returns `io_error` with the failing path in the message, the in-memory catalog caches keep their previous contents, `catalog:changed` is NOT emitted, and `config:changed` (`{"section":"groups"}`) is emitted instead so the UI still refetches the saved group list. A failed CONFIG write (before the pipeline) leaves everything untouched and emits nothing (B00 §2.2).

13. **ModelDetail.** Inverse of BenchmarkDetail: given a model display name and reasoning level (cleaned and collapsed the same way as the scores CSV), return every catalogue benchmark that pair reports. Unknown or untested pairs return empty `Rows`, not `not_found`, so Settings can open any catalogue combo. `Norm` is value/max×100 against every tested model of that benchmark, matching BenchmarkDetail.

## 3. Error behaviour

- All boundary errors map via `toErrorDTO` (B00 §3): `builtin_readonly`, `not_found`, `conflict`, `validation_failed` per §2.8/§2.9; file failures → `io_error` naming the path.
- Read methods (`Benchmarks`, `BenchmarkDetail`, `Groups`, `GroupDetail`) never mutate and never emit.
- Validation check order within `SaveGroup` is fixed (§2.8) so messages are golden-testable; exact strings in CONTRACTS §6.
- `ctx` cancellation during the pipeline maps to `io_error` with the §2.12 partial-persist semantics.

## 4. Decisions

| Decision | Value | Rationale |
|---|---|---|
| Builtin group source | `benchmark_selection.groups` of the parsed benchmarks.toml, file order | The real engine source (`score.ParseBenchmarkConfig`); hardcoding 11 names would drift |
| Benchmark note | Always the fixed fallback string | Verified: benchmarks.toml carries no descriptions; DTO field retained for future export metadata |
| Raw values re-parsed from scores CSV | Unexported raw-pair parse in catalog.go | `ScoreRow.Benchmarks` holds derived scores; D00 `BenchRow.Value` is the raw result; no engine change needed |
| Save vs derive failure | Persist group, report derive failure as `io_error`, catalog cache unchanged, emit `config:changed` only | The user's edit is never silently lost; catalog stays consistent (old scores) until a later save/refresh re-derives; rollback would discard intent over a transient IO fault |
| Custom-group ranking path | Service-side `Categories[slug]` overlay at load (§2.11) | `Derive`'s output schema is the fixed 12-category CSV; overlay reuses `CategoryScores` arithmetic without forking the engine |
| Re-derive on every group mutation | Always run §2.10, even when bytes may be identical | Keeps scores CSV provenance consistent with the merged config; mtime change is the observable contract of the verify test |
| Rename rewrites profiles | Custom `[profiles.*.tier2]` keys rewritten in the same atomic write; builtins untouched | One durable write per mutation (D00 §2.3); builtin profiles cannot reference custom slugs by construction |
| Duplicate allowed on builtins | Yes ("Duplicate & edit" affordance) | Mockup `onGrDuplicate`; the copy is custom and fully editable |

## 5. Out of scope

- `[groups.*]` TOML schema + accessors — B01. Catalog cache construction and `newTestServices` — B02. Profile validation accepting custom slugs — B03. Rank consumption of `Categories` — B04. Groups/benchmarks UI — U09.
