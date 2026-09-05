---
kind: feature-spec
version: "1.0"
feature: F23-cmd-catalog
project: which-model
---

# F23 — Catalog Command

Depends on: F06, F08, F09, F22

## Purpose

F23 implements `which-model catalog` (annex-d §2.2–§2.6): the two refresh stages —
Collect (orchestrated by F23 from the F08 fetch primitives) and Derive (F09) — with strict
stage ordering and offline rules (master plan §7.5), the read-only views `list` and
`providers` over the on-disk catalog, and the `workflow` stub whose implementation is owned
by F30. F23 also owns the `[catalog]` config section schema (F01 DECISION B) and the
collector cache for the models.dev catalogue. Everything registers through F22's command
registry; F23's files are all under `pkg/whichmodel/`.

## Behaviour

1. **Command tree** (`pkg/whichmodel/catalog_cmd.go`): `func init() { register(NewCatalogCmd) }`
   plus `func NewCatalogCmd() *cobra.Command` (order position 2 in `commandOrder`); subcommands
   `refresh`, `benchmarks`, `scores`, `list`, `providers`, `workflow` attach inside that file
   (Main DECISION A: one file = one registration unit). F23 adds no other registered commands.

2. **Stage model** (master plan §7.5, table in CONTRACTS):

   | Stage | Work | Inputs | Output | Needs AA key | Network |
   |---|---|---|---|---|---|
   | Collect | F23 orchestration over F08 primitives | models.dev API + Artificial Analysis v2 + optional aa_page | raw CSV (`available_model_raw_values.csv`) | yes | yes |
   | Derive | F09 `score.Derive` | raw CSV + `benchmarks.toml` + active Normalizer/Aggregator | scores CSV (`available_model_scores.csv`) | no | no |

   Flag→stage mapping (annex-d §1.2/§1.6): `--refresh-benchmarks`→Collect,
   `--refresh-scores`→Derive, `--refresh`→both, `--refresh-usage`→neither (usage cache is
   F24's refresh target, not a catalog stage). Global refresh flags trigger their stages on
   ANY catalog subcommand (e.g. `catalog list --refresh-scores` derives first), deduplicated
   against the subcommand's own stage.

3. **Stage execution** (`pkg/whichmodel/catalog_stages.go`):
   `runStages(ctx, runner, resolveKey, repoRoot, g, stages, collectOpts, deriveOpts)`.
   - Stages always execute in fixed order **Collect then Derive** (master plan rule: never
     reverse); a Collect failure aborts before Derive runs (exit code of the Collect error).
   - `--offline` with Collect in the stage set → `UsageError("Collect requires network access; incompatible with --offline")`
     (exit 2, exact annex-d §2.3 golden text). Derive under `--offline` is allowed.
   - When Collect is staged, the AA key is resolved first via the injected
     `AAKeyResolver` (default `aa.LoadAAAPIKey`; `repoRoot` = the git root found by walking
     up from the CWD); any resolution error →
     `UsageError("ARTIFICIAL_ANALYSIS_API is not set; the Collect stage requires an Artificial Analysis API key")`
     (exit 2, exact annex-d §2.3 golden text). The resolved key is passed in
     `CollectOptions.AAKey` (single resolution per run).
   - `defaultRunner` adapts to the pinned F06/F08/F09/F05 surfaces (Behaviour 4–6); tests
     inject a fake `StageRunner` via the package-level `newRunner` seam.

4. **Collect orchestration** (`pkg/whichmodel/catalog_collect.go`, `defaultRunner.Collect` —
   F23-owned; F08 is the fetch library only, per the F08 author). Local validation completes
   before any network call and before any file mutation (annex-b §9.4 fail-fast ordering):
   1. Provider set: when `co.ProviderConfigPath` is non-empty,
      `loadProviderConfig(co.ProviderConfigPath)` (F23's strict loader, annex-b §6.5);
      otherwise all provider ids discovered in the models.dev catalogue are used. A validated
      non-empty `co.Providers` restricts either set.
   2. Benchmark selection: `loadBenchmarkConfig(co.BenchmarksPath)` (strict, §6.3/§6.5) →
      expanded benchmark name list (group lists in declared order, then the direct list,
      deduplicated keeping first occurrence — the §6.3 expansion).
   3. models.dev provider catalogue: read the cache at `co.CatalogueCachePath` when its mtime
      is within `cache_ttl` (parsed from `[catalog] cache_ttl`, default 24h; invalid value →
      exit 2); otherwise `modelsdev.FetchModelsDevProvidersFrom(client, modelsdev.ProvidersURL)`
      (client = `httpkit.NewClient(httpkit.WithTimeout(Global.Timeout))`, injectable for
      tests) and write the cache (atomic temp+rename). The cached payload is the
      post-deprecated-filter catalogue, keyed by provider id. Only this api.json payload is
      cached (Decision D23); models.json is fetched fresh every Collect.
   4. models.dev benchmark evidence: `modelsdev.FetchModelsDevBenchmarksFrom(client, modelsdev.BenchmarksURL, expandedNames)`
      where `expandedNames` = the step-2 expanded benchmark names (annex-b §2.3 line 108:
      F08 pre-seeds its extraction map `{name: nil for name in selected_names}` — models are
      NOT filtered at fetch time; each returned `BenchmarkRecord` is one model's evidence rows
      for the selected benchmark names). The F23 merge still scopes cells to the step-2 names
      (defense in depth).
   5. AA v2: `aa.FetchAAv2(aa.AAV2Client(), co.AAKey)` (20s client is F08's pin). The
      fallback to the free endpoint is triggered ONLY by `*httpkit.Error` with
      `StatusCode == 403` (via `errors.As`); 401 propagates as `Error{Code: "unauthorized"}`.
      Error message text is never matched (F04 sanitizes it — the codes/status are the
      contract). `*httpkit.Error` codes flow through F22's exit mapping (§1.6 table:
      unauthorized → exit 5).
   6. Optional pages: when `co.AddAAPage`, `aa.FetchAAPage(aa.AAV2Client(), slug, false)`
      per AA model slug; page errors skip that model's page data (best-effort enrichment,
      Decision D6) — they never fail the collect.
   7. Fresh rows (Decision D7): the models.dev catalogue is the identity source. One row per
      (catalogue model, effort level): `model` = `CleanModelName`-equivalent of catalogue
      `Name` (fallback `ModelID`), `reasoning` = the normalized level (empty levels →
      `["high"]`). AA metrics attach when the AA slug's final path segment equals the
      catalogue `ModelID`; unmatched AA models are dropped; unmatched catalogue models get
      blank metric cells. Page metrics fill `time_per_intelligence_index_task_seconds` /
      `cost_per_intelligence_index_task_usd` only when the API v2 values are absent. Metric
      cells are decimal strings (`decimal.Decimal.String()`). Benchmark cells: the AA
      `Benchmarks` map value wins (keys are already generated column names); otherwise the
      models.dev evidence for the benchmark name matching the row's effort bucket (Effort ""
      applies to every level), scoped to the step-2 names; otherwise blank.
   8. Header = `csvstore.RawCoreColumns` + sorted benchmark columns; read the existing raw CSV
      (`csvstore.Read`; absent → empty); full refresh → `csvstore.MergeRows(existing, fresh)`;
      `--provider` subset → `csvstore.MergePartialRefresh(existing, fresh, refreshedModelIDs, true)`
      (unselected providers' rows preserved); `csvstore.ValidateRawRows`; if the file exists →
      `csvstore.Backup(co.OutPath, csvstore.DefaultBackupKeep)`; `csvstore.WriteAtomic(co.OutPath, merged, nil)`
      (the raw CSV carries no provenance line — it is the source of truth; Decision D8).
   9. `CollectResult{Providers: len(ids), Models: len(merged), RawCSVPath: co.OutPath}`
      (Decision D9: Models = rows written, effort levels included).

5. **Derive adapter** (`defaultRunner.Derive`):
   - `security.ReadBoundedFile(o.InPath, csvstore.MaxCsvBytes)` for the raw CSV; missing file →
     exit 1 error naming the path plus the fix command:
     `raw CSV not found at <path>; run 'which-model catalog benchmarks' (or '--refresh-benchmarks') to collect it`.
   - `security.ReadBoundedFile(o.BenchmarksPath, maxConfigBytes)` (maxConfigBytes = 1 MiB,
     F23 constant) for `benchmarks.toml`.
   - Normalizer/aggregator resolution (F09): `score.ResolveNormalizer(name)` /
     `score.ResolveAggregator(name)` where names come from `[scoring] normalizer|aggregator`
     (defaults "minmax-linear" / "weighted-arithmetic-mean"); unknown-name errors →
     `UsageError` (exit 2).
   - `score.Derive(raw, bench, norm, agg)` → `[]byte` (the closed-schema scores CSV incl. the
     §6.2a provenance line — F09's bytes are golden-tested and MUST NOT be re-serialized).
   - If the target exists: `csvstore.Backup(o.OutPath, csvstore.DefaultBackupKeep)`.
   - `csvstore.WriteAtomicBytes(o.OutPath, derived)` (F06 CONTRACTS §4.2: opaque bytes,
     temp+fsync+CAS+rename; no provenance parsing, no backup inside).
   - Row count for reporting: `countRows(derived)` = number of non-comment lines minus the
     header line (cells never contain embedded newlines in F09's output); `< 0` → error.
   - Missing benchmarks.toml → exit 1:
     `benchmarks config not found at <path>; provide benchmarks.toml or set catalog.benchmark_config_path`.

6. **Staleness warning** (annex-d §1.6 rule 2, master plan §7.5): after any command that
   writes or reads the scores CSV, F23 checks `csvstore.StaleCheck(scoresPath, rawPath)`
   (compares the scores provenance `raw_sha256` against the raw file hash; F06 semantics:
   provenance-unknown → not stale). Stale → `output.WriteWarning(Stderr, csvstore.StaleWarning(scoresPath, rawPath))`
   with the exact text `warning: scores CSV is stale relative to raw CSV; run 'which-model catalog scores' (or '--refresh-scores') to rebuild`.
   **Staleness is a warning, never an error: exit stays 0** (Decision D1). Suppressed by
   `--quiet` or `catalog.warn_on_stale_scores = false`. `StaleCheck` errors (e.g. missing raw)
   suppress the warning silently (verbose-only note).

7. **`catalog refresh`**: stages = union(global refresh flags, {Collect, Derive}); runs
   Behaviour 3; text output per stage: `collected N providers, M models -> <raw path>`
   (exact annex-d §2.4 wording) and `derived N rows -> <scores path>`; JSON (`--json`):
   `{"collect": {"providers": N, "models": M, "raw_csv_path": "..."}, "derive": {"rows": N, "scores_csv_path": "..."}}`
   inside the F03 envelope (stages that did not run are omitted). Flags: `--provider`
   (repeatable), `--provider-config <path>`, `--benchmarks <path>`, `--add aa_page`,
   `--out <path>` (scores output; raw output stays the configured raw path).

8. **`catalog benchmarks`**: stages = {Collect} plus global flags; runs Collect, prints the
   collect line (Behaviour 7), then, if the scores CSV exists, emits the staleness warning
   (Behaviour 6). JSON: `{"providers": N, "models": M, "raw_csv_path": "..."}` + envelope.
   `--add aa_page` applies.

9. **`catalog scores`**: stages = {Derive} plus global flags; offline-safe; prints
   `derived N rows -> <path>`; JSON: `{"rows": N, "scores_csv_path": "..."}` + envelope.
   Flags: `--benchmarks <path>`, `--out <path>`, `--in <path>` (raw CSV input override).

10. **`catalog list`** (annex-d §2.2): read-only view over the on-disk scores CSV
    (`csvstore.Read`). Missing scores file → exit 1:
    `scores CSV not found at <path>; run 'which-model catalog refresh' (or '--refresh-scores') to generate it`.
    Staleness warning per Behaviour 6. Row fields looked up by exact header name: `model`,
    `reasoning`, `intelligence_index`, `cost_per_intelligence_index_task_usd` (annex-d §2.2
    example field names; absolute columns of the §4.0a scores schema); a missing column omits
    the field (JSON) and renders `-` (text). Filters: `--reasoning <value>` (repeatable; row
    must match one of the given exact values), `--min-score <int 0..100>` (integer part of
    `intelligence_index`; blank/unparseable rows are excluded when the filter is active).
    **Default sort: `intelligence_index` descending, ties by `model` ascending; rows without
    a parseable index sort last** (Decision D2). Text: F03 `output.RenderTable` with headers
    `model, reasoning, intelligence_index, cost_per_intelligence_index_task_usd`; JSON: a
    bare array `[{"model": ..., "reasoning": ..., "intelligence_index": "63.1", "cost_per_intelligence_index_task_usd": "2.34"}, ...]`
    (bare-array precedent: `[]Snapshot` in global CONTRACTS).

11. **`catalog providers`** (annex-d §2.5): read-only view. F23 loads the provider config
    (Behaviour 12) and reads the on-disk cached models.dev catalogue at the resolved cache
    path (the same file Behaviour 4 writes; read at any age — the view never refreshes;
    missing file → exit 1:
    `provider catalogue not found at <path>; run 'which-model catalog benchmarks' (or '--refresh-benchmarks') to collect it`).
    Per provider (ids sorted alphabetically — TOML map order is non-deterministic, Decision
    D3): catalogue models filtered by provider id, minus `excluded_models` entries from the
    provider config (the excluded count is the config list length, Decision D10). Text
    `%-16s %-9s %s` → `anthropic       12 models   3 excluded` and with exclusions
    `1 excluded (grok-4.5)` (comma-separated); providers missing from the catalogue render
    `0 models`. JSON (legacy-compatible, annex-d §2.5):
    `{"<provider_id>": [{"id": ..., "name": ..., "reasoning": [...]}, ...], ...}` inside the
    F03 envelope. Flags: `--provider <id>` (repeatable subset), `--provider-config <path>`.
    Missing provider-config file → exit 1:
    `provider config not found at <path>; provide providers.toml at the repo root or set catalog.provider_config_path`.
    Invalid content (unknown keys, blank ids) → exit 2 (`ConfigError` semantics).

12. **Config loaders** (`pkg/whichmodel/catalog_config.go`): F23 owns the two strict TOML
    loaders (annex-b §6.5; F06/F08 have none):
    - `loadProviderConfig(path)`: `[providers.<id>]` with `excluded_models = [...]`; provider
      ids non-blank; excluded entries non-blank and de-duplicated (a duplicate entry is a
      hard error); unknown keys rejected via `toml.MetaData.Undecoded()`. Returns
      `map[string][]string` (id → excluded models). F23's loader validates `--provider` ids
      against the configured set (unknown id → `UsageError`, exit 2) before any stage runs.
      Collect does not call the loader when the resolved path is blank; it selects every
      provider discovered in the models.dev catalogue with no exclusions.
    - `loadBenchmarkConfig(path)`: `[benchmark_selection] groups = [...]` + `benchmarks = [...]`,
      each group name backed by a `[benchmark_groups.<name>]` table with a `benchmarks`
      string-array; blank/duplicate entries within the same list are hard errors (duplicates
      across different lists are allowed — dedup happens at expansion); unknown keys
      rejected. Returns the expanded benchmark name list: group lists in declared order, then
      the direct list, deduplicated keeping first occurrence (§6.3 expansion).
    Path resolution: `catalog.provider_config_path` / `catalog.benchmark_config_path` if set;
    else the repo-relative default: walk up from the current working directory to the nearest
    `.git` boundary looking for `providers.toml` / `benchmarks.toml`. The same walk-up finds
    the repo root for `aa.LoadAAAPIKey`.

13. **`[catalog]` section schema** (F01 DECISION B; owned by F23):
    `pkg/whichmodel/catalog_config.go`:

    ```go
    type CatalogConfig = catalog.Config

// internal/catalog.Config (shared schema):
type Config struct {
        RawCSVPath          string `toml:"raw_csv_path"`
        ScoresCSVPath       string `toml:"scores_csv_path"`
        ProviderConfigPath  string `toml:"provider_config_path"`
        BenchmarkConfigPath string `toml:"benchmark_config_path"`
        CacheTTL            string `toml:"cache_ttl"`            // default "24h"; models.dev catalogue freshness
        WarnOnStaleScores   bool   `toml:"warn_on_stale_scores"` // default true
    Publish            PublishConfig `toml:"publish"`
    }
    ```

    Defaults: `cache_ttl = "24h"`, `warn_on_stale_scores = true`, paths empty. Loaded via
    `cfg.UnmarshalKey("catalog", &c)` (decode-into-defaults; unknown keys → `ConfigError`,
    exit 2; env overrides `WHICH_MODEL_CATALOG_*` applied by F01). Path resolution
    (`resolveCatalogPaths`, `ResolvedCatalog`): empty `raw_csv_path`/`scores_csv_path` →
    `<CacheDir>/catalog/available_model_raw_values.csv` / `available_model_scores.csv`
    (annex-d §2.4 example paths; CacheDir from `config.ResolvePaths`); the models.dev
    catalogue cache → `<CacheDir>/catalog/modelsdev_providers.json`; empty provider path →
    Behaviour 12 walk-up; empty benchmark path → same walk-up for `benchmarks.toml`.

14. **`catalog workflow`** (annex-d §2.6): registered with `--write <file>` / `--check <file>`
    (mutually exclusive; both → exit 2) and `--out <dir>`; the body returns
    `CodedError{Code: "workflow_unavailable", Message: "catalog workflow generation is provided by feature F30 (publishing)"}`
    → exit 1. The generator implementation and the `[catalog.publish]` schema are F30's
    (F30 consumes `cfg.UnmarshalKey("catalog", ...)` per Main DECISION B).

15. **Config loading**: every catalog subcommand loads `config.Load(LoadOptions{Path: Global.ConfigPath})`
    then `loadCatalogConfig`; config errors map to exit 2 via F22 `ExitCodeFor`
    (`ConfigError.ExitCode() == 2`).

## Error behaviour

| Condition | Exit | Code | Message (exact) |
|---|---|---|---|
| Collect staged with `--offline` | 2 | arguments | `Collect requires network access; incompatible with --offline` |
| Collect staged, AA key unresolved | 2 | arguments | `ARTIFICIAL_ANALYSIS_API is not set; the Collect stage requires an Artificial Analysis API key` |
| Unknown `--add` value | 2 | arguments | `unknown --add value "x" (supported: aa_page)` |
| Unknown `--provider` id | 2 | arguments | `unknown provider "x" (configured: <ids>)` |
| Unknown normalizer/aggregator name | 2 | arguments | F09's typed error message |
| Invalid `catalog.cache_ttl` | 2 | config | `invalid catalog.cache_ttl "x"` |
| `--write` and `--check` together | 2 | arguments | `--write and --check are mutually exclusive` |
| Invalid config (F01/F23 section) | 2 | config | ConfigError message |
| Raw CSV missing (derive/list) | 1 | error | `raw CSV not found at <path>; run 'which-model catalog benchmarks' (or '--refresh-benchmarks') to collect it` |
| Scores CSV missing (list) | 1 | error | `scores CSV not found at <path>; run 'which-model catalog refresh' (or '--refresh-scores') to generate it` |
| benchmarks.toml missing | 1 | error | `benchmarks config not found at <path>; provide benchmarks.toml or set catalog.benchmark_config_path` |
| explicitly configured providers.toml missing | 1 | error | `provider config not found at <path>; provide providers.toml at the repo root or set catalog.provider_config_path` |
| models.dev cache missing (providers) | 1 | error | `provider catalogue not found at <path>; run 'which-model catalog benchmarks' (or '--refresh-benchmarks') to collect it` |
| Collect runtime failure (network, HTTP, parse) | per code | F08 primitive's error | `*httpkit.Error` codes map via F22's §1.6 table (unauthorized → 5); other codes → 1 |
| workflow stub | 1 | workflow_unavailable | `catalog workflow generation is provided by feature F30 (publishing)` |

Staleness is always a warning (exit 0). Scores/raw corruption inside `csvstore.Read` →
F06's error, exit 1.

## Decisions

| # | Decision | Value | Rationale |
|---|---|---|---|
| D1 | Staleness severity | warning on stderr, exit 0; suppressed by `--quiet` or `catalog.warn_on_stale_scores=false` | master plan §7.5 ("mismatch at read time warns, exit 0") + annex-d §1.6 rule 2 |
| D2 | list default sort | `intelligence_index` desc, then `model` asc; unparseable index sorts last | assignment requirement; annex-d §2.2 table implies index-first |
| D3 | providers ordering | ids sorted alphabetically | BurntSushi TOML maps are unordered; deterministic output required |
| D4 | providers model source | the on-disk cached models.dev payload (F23's own read; any age) | raw CSV has no provider column; annex-d §2.3 says list/providers are read-only over on-disk data |
| D5 | providers `--json` | per-provider array of `{id,name,reasoning}` inside the envelope | annex-d §2.5 legacy-compatible shape |
| D6 | aa_page best-effort | `aa.FetchAAPage(client, slug, false)`; page errors skip only that model's page data | opt-in enrichment must not fail the collect |
| D7 | merge identity | models.dev catalogue is the identity source; AA slug final segment == catalogue ModelID; unmatched AA dropped, unmatched catalogue blank; benchmark cells: AA map value wins, else models.dev evidence per effort bucket (Effort "" = all levels), scoped to the benchmarks.toml expansion; `MergeRows`/`MergePartialRefresh` (F06) preserve existing values | annex-b §2.2/§2.3/§3 merge semantics; update_raw_values.py merges rather than replaces |
| D8 | raw CSV provenance | `csvstore.WriteAtomic(path, rows, nil)` — no provenance line in the raw CSV | the raw CSV is the source of truth; scores provenance records its hash |
| D9 | collect counters | `Providers` = selected ids processed, or all distinct catalogue provider ids when no provider config exists; `Models` = rows written after merge (effort levels included) | deterministic; matches the annex-d §2.4 line format |
| D10 | excluded count | `len(excluded_models)` from providers.toml, independent of catalogue membership | matches the legacy providers command semantics |
| D11 | models.dev client | `httpkit.NewClient(httpkit.WithTimeout(Global.Timeout))` (default 10s); AA client fixed at 20s (`aa.AAV2Client`, F08 pin) | annex-d DefaultTimeoutSec for the general client; AA's slowness pinned by F08 |
| D12 | Derive write path | F09 bytes → `csvstore.Backup` (if exists) → `csvstore.WriteAtomicBytes` — never re-serialized | F09 author: Derive output is closed-schema, golden-tested; F06 author: backup kept out of WriteAtomicBytes, `DefaultBackupKeep` exported |
| D13 | AA-key failure mapping | any `aa.LoadAAAPIKey` error → the exact annex-d golden message, exit 2 | annex-d §2.3 fixes the CLI text; malformed-.env cases keep the same message |
| D14 | Derive row count | `countRows` = non-comment lines − header | F09's cells never contain newlines; cheap and deterministic |
| D15 | `--provider` validation | F23's own strict providers.toml parse; unknown id → exit 2 before any stage | F23 needs the ids for the providers view and the collect provider set anyway |
| D16 | `[catalog]` schema location | `pkg/whichmodel/catalog_config.go` | only CLI commands consume it; F06/F08 take values via options, not the struct |
| D17 | workflow stub | refusal `workflow_unavailable`, exit 1; F30 owns generator + `[catalog.publish]` | Main DECISION B boundary |
| D18 | list field lookup | exact header names from the annex-d §2.2 example; missing column omits the field | scores schema §4.0a keeps absolute columns; headers are the contract |
| D19 | F08 boundary | F08 = fetch library only (`fetch/modelsdev`, `fetch/aa`); F23 owns collect orchestration, the strict providers.toml/benchmarks.toml loaders (annex-b §6.5; F06/F09 have none), the merge, and the models.dev cache | F08 author's correction; F23 assembles per its SPEC out-of-scope |
| D20 | catalogue cache | `<CacheDir>/catalog/modelsdev_providers.json`; Collect reuses within `cache_ttl` (mtime), writes atomically (temp+rename); providers reads at any age | annex-b §8 cache_ttl semantics ("trusted before Collect re-fetches") |
| D21 | benchmark matching | provider model ↔ models.dev benchmark record matched by `CanonicalID == "<Provider>/<ModelID>"`, fallback bare `ModelID`; cell value = evidence `Score` for the row's effort bucket (highest-wins already applied by F08) | annex-b §2.3 canonical id format; deterministic, no fuzzy matching |
| D22 | benchmark source priority | AA v2 `AAModel.Benchmarks` wins; else models.dev evidence; else blank | annex-b §4.5 AA-preferred rule applied at the raw-cell level |
| D23 | models.json not cached | only the api.json provider payload is cached; benchmark evidence is fetched fresh each Collect | no read-only view consumes models.json; keeps the cache single-purpose |

## Out of scope

- The fetch primitives themselves (models.dev/AA v2 HTTP, pagination invariants, `httpkit`
  client construction for AA) — F08 (`internal/catalog/fetch/modelsdev`, `internal/catalog/fetch/aa`).
- The scoring math, benchmarks.toml schema, `[scoring]` section — F09.
- CSV read/write/staleness primitives — F06 (F23 uses them only).
- The `workflow` generator and `[catalog.publish]` schema — F30 (D17).
- `--refresh-usage` semantics — F24's usage cache; F23 treats it as a no-op stage.
- The aa_page data as its own subcommand — `--add aa_page` only (annex-d §2.3).
- Cached-catalogue freshness for the `providers` view — the view reads at any age (D4).

### Shared catalog schema correction (#166)

All catalog consumers decode the complete `catalog` table through F01's strict,
table-only `UnmarshalKey`. `internal/catalog.Config` owns all existing catalog
fields plus `Publish PublishConfig` (`toml:"publish"`); `catalog.PublishConfig`
owns the existing nested publishing fields. `whichmodel.CatalogConfig` and
`publish.PublishConfig` remain public aliases. Unknown catalog and publish keys
are errors; valid nested publishing settings coexist with configured raw/scores
paths, including environment-only overrides. Publishing seeds nested defaults,
then reads raw artifact paths from the decoded root. Pick validates catalog
configuration before loading scores and propagates config errors as exit 2.
Empty consumer paths retain their previous defaults. This corrects scalar
accessor guidance that contradicted F01's table-only contract.

## Correction — explicit full rebuild

`catalog refresh --rebuild` sets `CollectOptions.Rebuild bool`. It bypasses the model-catalog TTL and does not read or merge previous raw observations, so collection and derivation rebuild data from the current configured sources. Existing raw data is backed up before replacement. `--rebuild` with a `--provider` subset is a usage error. Ordinary refresh retains its existing merge policy. This explicit option implements the 2026-09-05 request for desktop Refresh data to perform a full rebuild.

Pinned tests: `TestCatalogRebuildBypassesCachedModelsAndOldRawData` (fresh-but-stale model cache and corrupt old raw CSV are replaced from current sources), `TestCatalogRefreshRebuildFlagReachesCollector` (flag reaches the collector).
