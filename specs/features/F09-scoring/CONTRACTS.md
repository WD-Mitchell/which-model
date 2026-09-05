---
kind: feature-contracts
feature: F09
version: "1.0"
project: which-model
---

# F09 — Scoring: Contracts (`internal/catalog/score`)

Package `internal/catalog/score` (directory `internal/catalog/score/`). Imports allowed per `specs/global/CONTRACTS.md §8`: `internal/catalog` types, `internal/decimal` (F02), `internal/catalog/identity` (F07), `internal/catalog/csvstore` (F06), stdlib, BurntSushi/toml. **MUST NOT import `internal/catalog/fetch/**`** (any package) — enforced by an import-graph test. No network, no filesystem writes.

## 1. Types and constants

### 1.1 Normalizer/Aggregator — `internal/catalog/score/config.go`

Global interfaces from `specs/global/CONTRACTS.md §2.2`, used verbatim:

```go
package score

type Normalizer interface {
    Normalize(raw decimal.Decimal, min, max decimal.Decimal) decimal.Decimal
}
type Aggregator interface {
    Aggregate(values []decimal.Decimal, weights []decimal.Decimal) (decimal.Decimal, bool)
}
```

F09-owned implementations in `internal/catalog/score/normalize.go` and `internal/catalog/score/aggregate.go`:

```go
package score

// MinMaxLinear: score = round(((raw - min) / (max - min)) * 100, 0); caller
// guarantees min != max (degenerate ranges are handled at column level) and
// min <= raw <= max. The global Normalizer interface carries no direction
// parameter, so lower-is-better metrics (time, cost, median) are
// direction-adjusted by the derive layer BEFORE Normalize via the exact
// reflection v' = min + max - v, which yields (max - v)/(max - min)
// (generate_scores.py normalized_score).
type MinMaxLinear struct{}

// WeightedArithmeticMean: round(Σ(v*w)/Σw, 0); weights may have any scale
// (renormalized by construction); returns false when len(values) == 0.
type WeightedArithmeticMean struct{}

// AGGREGATOR_* and NORMALIZER_* name constants:
const (
    NormalizerNameMinMaxLinear string = "minmax-linear"
    AggregatorNameWeightedArithmeticMean string = "weighted-arithmetic-mean"
)

// DefaultNormalizer / DefaultAggregator return the canonical instances.
func DefaultNormalizer() Normalizer
func DefaultAggregator() Aggregator

// ResolveNormalizer / ResolveAggregator map a config name to an instance;
// unknown names return *Error{Code: ErrUnknownNormalizer|ErrUnknownAggregator}
// with message `unknown normalizer: <name>` / `unknown aggregator: <name>`.
func ResolveNormalizer(name string) (Normalizer, error)
func ResolveAggregator(name string) (Aggregator, error)
```

### 1.2 Benchmark config — `internal/catalog/score/benchmark_config.go`

```go
type BenchmarkConfig struct {
    CanonicalBenchmarks map[string][]string // canonical name -> alias list (incl. itself)
    EvidenceGroups      []EvidenceGroup     // category evidence, file order
}
type EvidenceGroup struct {
    Category   string   // one of the 11 grouped category names (not planning_capability)
    Benchmarks []string // canonical benchmark names, group-list order (D3 layer b)
}

// CategoryMinimumEvidence is the per-column gate of generate_scores.py:66-81
// (security_score needs 1 populated evidence; every other category needs 2).
var CategoryMinimumEvidence = map[string]int{
    "reasoning": 2, "knowledge": 2, "research": 2, "instruction_following": 2,
    "software_engineering": 2, "ui_visual": 2, "agentic_tools": 2,
    "finance": 2, "evidence_capture": 2, "security": 1, "data_ml": 2,
}

// ParseBenchmarkConfig parses the benchmarks catalog TOML (strict: unknown
// top-level and group keys are errors; evidence names canonicalized via
// identity.BenchmarkAliases; unknown evidence names -> ErrInvalidBenchmarkConfig).
func ParseBenchmarkConfig(data []byte) (*BenchmarkConfig, error)
```

### 1.3 Errors — `internal/catalog/score/errors.go`

```go
type ErrorCode int

const (
    ErrInvalidRaw             ErrorCode = iota // raw CSV invalid (exit 1)
    ErrInvalidBenchmarkConfig                  // benchmarks TOML invalid (exit 1)
    ErrInvalidScoresCSV                        // scores CSV invalid (exit 1)
    ErrUnknownNormalizer                       // exit 2
    ErrUnknownAggregator                       // exit 2
)

type Error struct {
    Code    ErrorCode
    Message string // Python-verbatim, see table below
}

func (e *Error) Error() string
func (e *Error) Unwrap() error
```

Verbatim messages (generate_scores.py:95-193, rank_models.py:359-401; `{n}` = 1-based row):

| Condition | Message |
|---|---|
| blank core cell | `row {n}: {column} must not be blank` |
| non-numeric cell | `row {n}: {column} must be numeric, got '{value}'` |
| non-finite cell | `row {n}: {column} must be finite, got '{value}'` |
| negative time/cost/median | `row {n}: {column} must not be negative, got '{value}'` |
| too many cells | `row {n}: too many values` |
| too few cells | `row {n}: too few values` |
| unexpected core headers | `unexpected core columns: expected {joined core columns}, got {joined actual}` |
| bad dynamic columns | `invalid or duplicate dynamic benchmark columns` |
| model blank after cleaning | `model name is blank after removing annotations` |
| zero data rows | `input contains no data rows` |
| zero measured rows | `input contains no published metric values` |
| unknown normalizer/aggregator | `unknown normalizer: {name}` / `unknown aggregator: {name}` |
| scores CSV missing columns | `score CSV is missing required columns: {sorted names}` |
| scores CSV extra cells | `score CSV row {n} has extra cells` |
| scores CSV blank identity | `score CSV row {n} has a blank model/reasoning identity` |
| scores CSV duplicate identity | `score CSV has duplicate identity: {model} / {reasoning}` |
| scores CSV out-of-range cell | `score CSV row {n} {column} must be between 0 and 100` |
| scores CSV empty | `score CSV contains no model rows` |

Row numbers `{n}` in raw-CSV messages start at **2** (row 1 is the header; `generate_scores.py:169`).

### 1.4 Owned config keys

| Key | Type | Default | Owner of reading |
|---|---|---|---|
| `scoring.normalizer` | string enum (`minmax-linear`) | `minmax-linear` | F23 via F01 `UnmarshalKey("scoring", ...)` → `ResolveNormalizer` |
| `scoring.aggregator` | string enum (`weighted-arithmetic-mean`) | `weighted-arithmetic-mean` | F23 via F01 `UnmarshalKey("scoring", ...)` → `ResolveAggregator` |

Registered in F01's envKeys/vocabulary (`WHICH_MODEL_SCORING_NORMALIZER`, `WHICH_MODEL_SCORING_AGGREGATOR`; F01 validates keys, F09 validates values). Defaults are applied by F09's `DefaultScoringConfig`-equivalent: F23 pre-populates the struct with `DefaultNormalizerName`/`DefaultAggregatorName` before `UnmarshalKey`, so an absent `[scoring]` section yields the defaults.

## 2. Functions

### 2.1 Derivation — `internal/catalog/score/derive.go`

```go
// Derive is the single public entry point: raw CSV bytes + benchmarks TOML
// bytes -> scores CSV bytes (dual-column schema, annex-b §4.0a), including
// the provenance header line. Pure, offline, deterministic (SPEC D6).
// rawCSV may carry a leading "# raw_sha256=..." line (F06 ProvenancePrefix),
// which is stripped before parsing; the header line emitted uses
// csvstore.ProvenanceHash on the raw bytes AS GIVEN (including the stripped
// line, matching F06 semantics).
//
// Processing order (generate_scores.py read_rows + _merge_input_rows +
// generate): parse + validate (row numbers start at 2) -> merge duplicate
// identities (CleanModelName + default->high collapse; first-wins fill-in for
// core cells, max for benchmark cells, SPEC D8) -> measured-row filter (any published metric; SPEC D9) -> per-column
// min/max ranges over eligible rows -> relative scores (direction-aware
// MinMaxLinear, ROUND_HALF_UP) -> category composites -> provenance header.
func Derive(rawCSV []byte, benchmarksTOML []byte, normalizer Normalizer, aggregator Aggregator) ([]byte, error)
```

Output layout (per row, after the header line): `model,reasoning`, then 6 interleaved metric pairs in RawCoreColumns metric order — `intelligence_index`, `time_per_intelligence_index_task_seconds`, `cost_per_intelligence_index_task_usd`, `median_end_to_end_response_time_seconds`, `artificial_analysis_coding_index`, `artificial_analysis_agentic_index` (absolute `<metric>`, then `<metric>_score`) — then 12 category `_score` columns in CATEGORY_SCORE_COLUMNS order (reasoning, knowledge, research, planning_capability, instruction_following, software_engineering, ui_visual, agentic_tools, finance, evidence_capture, security, data_ml), then per-benchmark pairs `benchmark:<name>`, `benchmark:<name>_score` in raw dynamic-column order. Absolute cells are the raw values verbatim (blank preserved); relative cells are integer 0-100 or blank. The metric column names and direction flags (lower-is-better for the three latency/cost metrics) come from RawCoreColumns (F06) and the CORE_METRICS flags below.

### 2.2 Scores CSV parsing — `internal/catalog/score/parse_scores.go`

```go
// ParseScoresCSV parses a Derive-produced scores CSV into rows for F10.
// Rejects duplicate identities HERE (rank_models.py:359-401). A leading
// provenance line is validated per F06 rules (ProvenancePrefix shape,
// raw_sha256 64-hex; second '#' line or malformed token -> ErrInvalidScoresCSV)
// but not exposed. Empty input -> ErrInvalidScoresCSV.
func ParseScoresCSV(data []byte) ([]catalog.ScoreRow, error)
```

`catalog.ScoreRow` (global, `specs/global/CONTRACTS.md §2.1`): `Tier1 map[string]decimal.Decimal` keyed by the 6 `_score` column names; `Categories map[string]decimal.Decimal` keyed by plain category names; `Benchmarks map[string]decimal.Decimal` keyed by benchmark name; absent key = blank. Identities are cleaned via `identity.CleanModelName` and `identity.CollapseReasoning` (F07) at parse time.

### 2.3 Composites — `internal/catalog/score/composites.go`

```go
// SourceScores resolves a row's evidence map (annex-b §4.5): the two AA index
// score columns first seed canonical keys ("Artificial Analysis Coding Index",
// "Artificial Analysis Coding Agent Index" via identity.BenchmarkKey), then
// each benchmark _score column setdefaults its key in raw CSV header order
// (D3 layer a; AA-index-preferred over models.dev).
func SourceScores(row catalog.ScoreRow) map[string]decimal.Decimal

// CategoryScores computes the 11 grouped category composites for one row
// (generate_scores.py _category_score): per-group unweighted mean
// (sum/len, ROUND_HALF_UP) of populated evidence in benchmarks.toml group
// list order, deduped by BenchmarkKey (D3 layer b); blank when the number of
// populated evidences is below CategoryMinimumEvidence[group].
// Returns map[category]decimal.Decimal with absent keys for blanks.
func CategoryScores(row catalog.ScoreRow, cfg *BenchmarkConfig) map[string]decimal.Decimal

// PlanningCapabilityScore = 0.4*reasoning + 0.3*knowledge + 0.2*agentic_tools
// + 0.1*research, ROUND_HALF_UP; zero (blank) when ANY input category score
// is absent. Inputs are the row's computed category scores.
func PlanningCapabilityScore(categoryScores map[string]decimal.Decimal) decimal.Decimal
```

### 2.4 ScoringConfig — `internal/catalog/score/config.go`

```go
// ScoringConfig is the [scoring] TOML section owned by F09 (read by F23 via
// cfg.UnmarshalKey("scoring", &c) with c pre-set to DefaultScoringConfig()).
type ScoringConfig struct {
    Normalizer string `toml:"normalizer"`
    Aggregator string `toml:"aggregator"`
}

func DefaultScoringConfig() ScoringConfig // {Normalizer: "minmax-linear", Aggregator: "weighted-arithmetic-mean"}
```

## 3. Flags, error codes, JSON shapes

- **Flags owned:** none (F23 owns `--scores-csv`, `--benchmarks`, etc.).
- **Exit-code mapping:** `ErrInvalidRaw`, `ErrInvalidBenchmarkConfig`, `ErrInvalidScoresCSV` → exit 1 (data error); `ErrUnknownNormalizer`, `ErrUnknownAggregator` → exit 2 (config error) — per `specs/global/SPEC.md §5`, applied by F23.
- **JSON shapes emitted:** none (Derive emits CSV bytes).
- **Provenance header emitted** (annex-b §6.2a): `# which-model-scores-provenance raw_sha256=<64-hex> normalizer=<name> aggregator=<name>` — token-separated, `raw_sha256` required, `normalizer=`/`aggregator=` optional, names are `NormalizerNameMinMaxLinear` / `AggregatorNameWeightedArithmeticMean`. Parsed by F06 `Provenance` (`specs/features/F06-*/CONTRACTS.md`).

## Partial-coverage regression rows

| Input | Required output |
|---|---|
| Astra intelligence 60/cost 3 and no speed; baseline intelligence 20/cost 1 | Astra retained, relative intelligence 100/cost 0; speed absolute and relative blank |
| Benchmark-only row at 60, other published values 30 and 90 | Row retained and benchmark relative score 50 |
| Two intelligence values equal to 60, all other metrics blank | Both absolute values retained; relative columns blank; no error |
| Identity-only rows | `input contains no published metric values` |

These rows supersede the prior mandatory-core filtering and degenerate-core error cases, per the user's 2026-09-05 correction. Output schema and numeric validation remain unchanged.
