---
kind: feature-contracts
version: "1.0"
feature: F23-cmd-catalog
project: which-model
---

# F23 — Contracts

## Packages

| Package | Path | Status |
|---|---|---|
| `whichmodel` (catalog files only) | `pkg/whichmodel/catalog_*.go` | F23-owned |
| `config` (`internal/config`) | F01 | consumed verbatim |
| `output` (`internal/output`) | F03 | consumed verbatim |
| `httpkit` (`internal/httpkit`) | F04 | consumed verbatim |
| `security` (`internal/security`) | F05 | consumed verbatim |
| `csvstore` (`internal/catalog/csvstore`) | F06 | consumed verbatim |
| `modelsdev` (`internal/catalog/fetch/modelsdev`) | F08 | consumed verbatim |
| `aa` (`internal/catalog/fetch/aa`) | F08 | consumed verbatim |
| `score` (`internal/catalog/score`) | F09 | consumed verbatim |

F23 adds no `internal/` packages; all F23 files live in `pkg/whichmodel/`.

## Stage table (master plan §7.5)

| Stage | Implemented by | Inputs | Output | Needs AA key | Network |
|---|---|---|---|---|---|
| Collect | F23 orchestration over F08 primitives | models.dev API + Artificial Analysis v2 + optional aa_page | `available_model_raw_values.csv` | yes | yes |
| Derive | F09 `score.Derive` | raw CSV + `benchmarks.toml` + active Normalizer/Aggregator | `available_model_scores.csv` | no | no |

Flag→stage mapping (annex-d §1.2/§1.6): `--refresh-benchmarks`→Collect,
`--refresh-scores`→Derive, `--refresh`→Collect+Derive, `--refresh-usage`→no catalog stage.
Global flags union with the subcommand's own stage; execution order is always
Collect-then-Derive (never reversed; Collect failure aborts before Derive).

## Exported API — package `whichmodel` (F23-owned files)

### `catalog_cmd.go`

```go
func NewCatalogCmd() *cobra.Command // Use "catalog"; registered via init(); order position 2
// subcommands: refresh, benchmarks, scores, list, providers, workflow
```

### `catalog_config.go` — F23-owned `[catalog]` section schema (F01 DECISION B)

```go
type CatalogConfig struct {
    RawCSVPath          string `toml:"raw_csv_path"`
    ScoresCSVPath       string `toml:"scores_csv_path"`
    ProviderConfigPath  string `toml:"provider_config_path"`
    BenchmarkConfigPath string `toml:"benchmark_config_path"`
    CacheTTL            string `toml:"cache_ttl"`            // default "24h"; models.dev catalogue freshness
    WarnOnStaleScores   bool   `toml:"warn_on_stale_scores"` // default true
}

func DefaultCatalogConfig() CatalogConfig

type ResolvedCatalog struct {
    RawCSVPath, ScoresCSVPath, ProviderConfigPath, BenchmarkConfigPath string
    CatalogueCachePath                                                  string // <CacheDir>/catalog/modelsdev_providers.json
}

func loadCatalogConfig(cfg *config.Config) (CatalogConfig, error)
// cfg.UnmarshalKey("catalog", &c) into DefaultCatalogConfig(); errors propagate (exit 2)

func resolveCatalogPaths(c CatalogConfig, paths config.Paths, cwd string) ResolvedCatalog
// empty raw/scores -> <CacheDir>/catalog/available_model_raw_values.csv | available_model_scores.csv
// empty provider/benchmark -> walk up from cwd to nearest .git boundary for providers.toml | benchmarks.toml
// CatalogueCachePath always <CacheDir>/catalog/modelsdev_providers.json

func loadProviderConfig(path string) (map[string][]string, error)
// strict TOML (annex-b §6.5): [providers.<id>] excluded_models = [...]; blank ids rejected;
// blank/duplicate excluded entries rejected; unknown keys rejected (toml.MetaData.Undecoded);
// returns id -> excluded model ids

func loadBenchmarkConfig(path string) ([]string, error)
// strict TOML (annex-b §6.5/§6.3): [benchmark_selection] groups + benchmarks; every group
// name backed by a [benchmark_groups.<name>] table; blank/duplicate entries within one list
// rejected; unknown keys rejected; returns the EXPANDED name list (groups in declared order,
// then direct list, deduplicated keeping first occurrence)

func findRepoRoot(cwd string) string // first dir from cwd upward containing .git (dir or file); "" if none

func parseCacheTTL(s string) (time.Duration, error) // wraps time.ParseDuration; error -> ConfigError(KindInvalidValue, key "catalog.cache_ttl")

func loadConfig() (*config.Config, error) // config.Load(LoadOptions{Path: Global.ConfigPath})
```

### `catalog_stages.go` — stage runner (test seam: `var newRunner = func() StageRunner { return &defaultRunner{} }`)

```go
type Stage int
const ( StageCollect Stage = iota; StageDerive )

type CollectOptions struct {
    Providers           []string
    ProviderConfigPath  string
    BenchmarksPath      string // benchmarks.toml for the §6.3 selection expansion
    AddAAPage           bool
    OutPath             string
    Timeout             time.Duration
    CacheTTL            time.Duration // catalog.cache_ttl; invalid values rejected at config load
    AAKey               string
    CatalogueCachePath  string
}
type CollectResult struct { Providers, Models int; RawCSVPath string }

type DeriveOptions struct {
    InPath, OutPath, BenchmarksPath, Normalizer, Aggregator string
}
type DeriveResult struct { Rows int; ScoresCSVPath string }

type StageRunner interface {
    Collect(ctx context.Context, o CollectOptions) (CollectResult, error)
    Derive(ctx context.Context, o DeriveOptions) (DeriveResult, error)
}

type AAKeyResolver func(repoRoot string) (string, error) // default: aa.LoadAAAPIKey

type stageReport struct {
    Collect *CollectResult
    Derive  *DeriveResult
}

func runStages(ctx context.Context, r StageRunner, resolveKey AAKeyResolver, repoRoot string,
    g *GlobalFlags, stages []Stage, co CollectOptions, do DeriveOptions) (stageReport, error)
// fixed order Collect-then-Derive; offline+Collect -> UsageError (annex-d §2.3 golden);
// AA-key failure -> UsageError (annex-d §2.3 golden); key passed via co.AAKey

func stageSet(g *GlobalFlags, sub []Stage) []Stage // union of global refresh flags + subcommand stages, canonical order

type defaultRunner struct{}
func (defaultRunner) Collect(ctx context.Context, o CollectOptions) (CollectResult, error) // see catalog_collect.go
func (defaultRunner) Derive(ctx context.Context, o DeriveOptions) (DeriveResult, error)    // see Behaviour 5 of SPEC.md

const maxConfigBytes = 1 << 20 // benchmarks.toml bound

func countRows(derived []byte) (int, error) // non-comment lines minus header; error when < 0
func resolveNormalizerName(name string) (score.Normalizer, error) // score.ResolveNormalizer; unknown -> UsageError
func resolveAggregatorName(name string) (score.Aggregator, error) // score.ResolveAggregator; unknown -> UsageError
func warnIfStale(scoresPath, rawPath string, quiet bool, warnOnStale bool)
// csvstore.StaleCheck -> csvstore.StaleWarning -> output.WriteWarning(Stderr, ...);
// StaleCheck errors suppress the warning; quiet/warnOnStale=false suppress it
```

### `catalog_collect.go` — F23-owned Collect orchestration (F08 = fetch library only)

```go
// buildFreshRows merges the models.dev catalogue, AA v2 (+ optional page) data and
// models.dev benchmark evidence into fresh raw rows per (model, effort level).
// Catalogue is the identity source (SPEC D7); benchmark cells: AA map value wins,
// else models.dev evidence for the effort bucket, scoped to expandedNames (SPEC D21/D22).
// Header = csvstore.RawCoreColumns + sorted benchmark columns.
func buildFreshRows(catalogue []modelsdev.ProviderModel, benchmarks []modelsdev.BenchmarkRecord,
    aaModels []aa.AAModel, pages map[string]aa.PageMetrics, expandedNames []string) ([]csvstore.Row, error)

// mergeWithExisting applies the F06 merge semantics: full refresh -> MergeRows;
// subset refresh -> MergePartialRefresh(existing, fresh, refreshedModelIDs, true).
func mergeWithExisting(existing []csvstore.Row, fresh []csvstore.Row, refreshedModelIDs []string) ([]csvstore.Row, error)

// catalogueCache: readCache(path) ([]modelsdev.ProviderModel, bool, error)  // ok=false when missing
// writeCache(path string, catalogue []modelsdev.ProviderModel) error       // atomic temp+rename
// cacheFresh(path string, ttl time.Duration) bool                          // mtime + ttl > now
```

### `catalog_flags.go`

```go
type catalogFlags struct { // subcommand-local flags
    Providers         []string // --provider (repeatable)
    ProviderConfig    string   // --provider-config
    Benchmarks        string   // --benchmarks
    In, Out           string   // --in, --out
    Add               []string // --add (repeatable; only "aa_page" valid)
    Reasoning         []string // --reasoning (repeatable, list)
    MinScore          int      // --min-score (0..100)
    Write, Check      string   // --write, --check (workflow)
}

func (f *catalogFlags) Bind(cmd *cobra.Command)
func validateAdd(values []string) error
func validateProviders(ids []string, configured map[string][]string) error
func validateWorkflowFlags(f *catalogFlags) error
```

### `catalog_providers.go` — providers view (test seam: `var cacheReader = readCache`)

```go
type providerRow struct{ ID string; Models []modelsdev.ProviderModel; Excluded []string }
func renderProviders(configured map[string][]string, catalogue []modelsdev.ProviderModel, subset []string) ([]providerRow, error)
func providersText(w io.Writer, rows []providerRow)
func providersJSON(rows []providerRow) map[string]any
```

## Consumption contract — F08 `internal/catalog/fetch/modelsdev` (cited verbatim)

```go
const ProvidersURL = "https://models.dev/api.json"
const BenchmarksURL = "https://models.dev/models.json"

type ProviderModel struct {
    Provider, ModelID, Name, Status, BaseModel string
    Reasoning      bool
    EffortLevels   []string // "none"/"default" already normalized to "high"
}
type BenchmarkRecord struct {
    CanonicalID, Name string
    Benchmarks        []BenchmarkEvidence
}
type BenchmarkEvidence struct {
    Name  string
    Score decimal.Decimal
    Effort string // "" = non-effort-scoped, else normalized level
}

func FetchModelsDevProviders() ([]ProviderModel, error)         // one unauthenticated GET; drops status=="deprecated"
func FetchModelsDevBenchmarks(selectedNames []string) ([]BenchmarkRecord, error)
func FetchModelsDevProvidersFrom(client *httpkit.Client, url string) ([]ProviderModel, error)
func FetchModelsDevBenchmarksFrom(client *httpkit.Client, url string, selectedNames []string) ([]BenchmarkRecord, error)
// selectedNames = BENCHMARK NAMES from benchmarks.toml §6.3 (annex-b §2.3 extraction map),
// never model ids — models are not filtered at fetch time; each BenchmarkRecord is one
// model's evidence rows for the selected names. models.json is a SEPARATE GET from api.json.
// the From variants are the injectable cores F23's tests drive with httptest servers
```

## Consumption contract — F08 `internal/catalog/fetch/aa` (cited verbatim)

```go
const APIKeyEnv = "ARTIFICIAL_ANALYSIS_API"
const MaxPages = 100

func LoadAAAPIKey(repoRoot string) (string, error)
// env first, then <repoRoot>/.env (KEY=VALUE scanner; duplicate key = hard error);
// missing names both sources; no ctx, no config file

func AAV2Client() *httpkit.Client // httpkit.NewClient(httpkit.WithTimeout(20 * time.Second))

func FetchAAv2(client *httpkit.Client, apiKey string) ([]AAModel, error)
// x-api-key header; page loop with the 4 hard invariants; 403-only fallback to FreeURL
// detected via `errors.As(err, &he); he.StatusCode == 403` — never by message text
// (F04 sanitizes Error()); 401 = no fallback, propagates as *httpkit.Error{Code:"unauthorized"}

func FetchAAPage(client *httpkit.Client, slug string, requireFallbackCost bool) (*PageMetrics, error)
// no auth headers; currentModel markers

type AAModel struct {
    Slug                  string
    IntelligenceIndex     *decimal.Decimal
    CodingIndex           *decimal.Decimal
    AgenticIndex          *decimal.Decimal
    MedianResponseSeconds *decimal.Decimal
    CostPerTaskUSD        *decimal.Decimal
    Benchmarks            map[string]decimal.Decimal // keyed by generated column name, fraction×100, highest-wins
}
type PageMetrics struct {
    Slug                              string
    TimePerIntelligenceTaskSeconds    *decimal.Decimal
    FallbackCostUSD                   *decimal.Decimal
}
```

## Consumption contract — F09 `internal/catalog/score` (cited verbatim)

```go
func Derive(rawCSV []byte, benchmarksTOML []byte, normalizer Normalizer, aggregator Aggregator) ([]byte, error)
// output = scores CSV text; first line: "# which-model-scores-provenance raw_sha256=<hex> normalizer=<n> aggregator=<a>"

func ResolveNormalizer(name string) (Normalizer, error) // "minmax-linear" default; typed error on unknown name
func ResolveAggregator(name string) (Aggregator, error) // "weighted-arithmetic-mean" default; typed error on unknown name

type ScoringConfig struct {
    Normalizer string `toml:"normalizer"` // default "minmax-linear"
    Aggregator string `toml:"aggregator"` // default "weighted-arithmetic-mean"
}
func DefaultScoringConfig() ScoringConfig
// loaded by F23 via cfg.UnmarshalKey("scoring", &cfg)
```

## Consumption contract — F06 `internal/catalog/csvstore` (cited verbatim; F06 CONTRACTS §1–§4)

```go
const (
    BenchmarkColumnPrefix = "benchmark:"
    ProvenancePrefix      = "# which-model-scores-provenance"
    MaxCsvBytes           = 16 << 20
    DefaultBackupKeep     = 5
)

var RawCoreColumns = []string{
    "model", "reasoning", "intelligence_index",
    "time_per_intelligence_index_task_seconds",
    "cost_per_intelligence_index_task_usd",
    "median_end_to_end_response_time_seconds",
    "artificial_analysis_coding_index",
    "artificial_analysis_agentic_index",
}

type Row struct {
    Header        []string
    Values        []string
    Authoritative map[string]bool
}
type Provenance struct {
    RawSHA256  string // required token, 64 lowercase hex
    Normalizer string // optional token, verbatim
    Aggregator string // optional token, verbatim
}

func Read(path string) (rows []Row, provenance *Provenance, err error)
// single leading "# which-model-scores-provenance" comment line; raw_sha256 required
// (64 lowercase hex, else ErrMalformedCSV); normalizer/aggregator optional; unknown keys
// skipped; no comment line -> provenance == nil (never stale)
func WriteAtomic(path string, rows []Row, provenance *Provenance) error
func WriteAtomicBytes(path string, content []byte) error // opaque; temp+fsync+CAS+rename; NO backup inside
func Backup(path string, keep int) (backupPath string, err error)
func StaleCheck(scoresPath, rawPath string) (bool, error) // compares raw_sha256 only
func StaleWarning(scoresPath, rawPath string) string
func ValidateRawRows(rows []Row) error
```

Consumed additionally: `security.ReadBoundedFile` (F05), `httpkit.NewClient` +
`httpkit.WithTimeout` (F04), `output.RenderJSON/RenderTable/WriteWarning/WriteFailure`
(F03), `decimal.Parse` (F02), `config.Load/LoadOptions/UnmarshalKey/ResolvePaths/Paths`
(F01), `whichmodel.Global/GlobalFlags/UsageError/CodedError/ExitCodeFor/CodeFor/register/
NewCatalogCmd pattern` (F22).

## Config keys owned by F23

`[catalog] raw_csv_path | scores_csv_path | provider_config_path | benchmark_config_path |
cache_ttl | warn_on_stale_scores` (schema + defaults per DECISION B; env overrides
`WHICH_MODEL_CATALOG_*` handled by F01's `UnmarshalKey`). Read-only:
`[scoring] normalizer | aggregator` (F09's schema), `ARTIFICIAL_ANALYSIS_API` env +
`<repo root>/.env` (via `aa.LoadAAAPIKey`), `--config` (via `config.Load`).

## Flags owned by F23

| Flag | Where | Meaning |
|---|---|---|
| `--provider <id>` (repeatable) | refresh, benchmarks, providers | provider subset; validated against providers.toml ids |
| `--provider-config <path>` | refresh, benchmarks, providers | providers.toml override |
| `--benchmarks <path>` | refresh, scores | benchmarks.toml override |
| `--in <path>` | scores | raw CSV input override |
| `--out <path>` | refresh, scores, workflow | scores CSV / output override |
| `--add <value>` (repeatable) | refresh, benchmarks | `aa_page` only; unknown → exit 2 |
| `--reasoning <value>` (repeatable) | list | exact-match filter |
| `--min-score <int>` | list | 0..100 filter on intelligence_index |
| `--write <file>` / `--check <file>` | workflow | mutually exclusive; both → exit 2 |

Global flags consumed: `--refresh`, `--refresh-benchmarks`, `--refresh-scores`,
`--offline`, `--timeout`, `--json`, `--quiet`, `--config`.

## Error codes added by F23

| Code | Exit | Where |
|---|---|---|
| `arguments` | 2 | offline+Collect, missing AA key, unknown `--add`/`--provider`, unknown normalizer/aggregator, `--write`+`--check` |
| `config` | 2 | `loadCatalogConfig` / `loadProviderConfig` / `loadConfig` / `parseCacheTTL` failures |
| `error` | 1 | missing raw/scores/benchmarks/provider-config/catalogue files, Collect runtime failures |
| `workflow_unavailable` | 1 | workflow stub (F30 boundary) |

## JSON shapes emitted by F23

| Command | Shape |
|---|---|
| `refresh --json` | `{"collect": {"providers": N, "models": M, "raw_csv_path": "..."}, "derive": {"rows": N, "scores_csv_path": "..."}}` + envelope; absent stage omitted |
| `benchmarks --json` | `{"providers": N, "models": M, "raw_csv_path": "..."}` + envelope |
| `scores --json` | `{"rows": N, "scores_csv_path": "..."}` + envelope |
| `list --json` | bare array `[{"model": "...", "reasoning": "...", "intelligence_index": "63.1", "cost_per_intelligence_index_task_usd": "2.34"}, ...]`; missing columns omitted |
| `providers --json` | `{"<provider_id>": [{"id": "...", "name": "...", "reasoning": [...]}], ...}` + envelope |

Text shapes: collect line `collected N providers, M models -> <path>`; derive line
`derived N rows -> <path>`; providers rows `%-16s %-9s %s` e.g.
`anthropic       12 models   3 excluded` / `1 excluded (grok-4.5)`; staleness warning exact
text per `csvstore.StaleWarning`; list table via F03 `RenderTable` headers
`model, reasoning, intelligence_index, cost_per_intelligence_index_task_usd`.

## Build contract

- All F23 files compile under `-tags nousage` (F21 stubs cover usage paths; F23 touches
  usage only via the `--refresh-usage` no-op).
- F23 never edits files owned by other features; consumption is via the pinned APIs above.
- Derive never re-serializes F09's bytes (D12); the provenance line's `normalizer`/`aggregator`
  fields are preserved verbatim by `WriteAtomicBytes`.
- The models.dev catalogue cache is F23-owned JSON (`[]ProviderModel`); Collect enforces
  `cache_ttl` via mtime; the providers view reads it at any age (D4, D20).
