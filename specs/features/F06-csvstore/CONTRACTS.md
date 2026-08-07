---
kind: feature-contracts
feature: F06-csvstore
version: "1.0"
project: which-model
---

# F06 — csvstore: Contracts

Package: `internal/catalog/csvstore` (module `github.com/WD-Mitchell/which-model`).

Imports (allowed by `specs/global/CONTRACTS.md §8`): `internal/decimal` (F02), `internal/security` (F05), stdlib (`encoding/csv`, `crypto/sha256`, `os`, `path/filepath`, `time`, `strings`, `unicode/utf8`, `errors`, `fmt`, `sort`, `io/fs`). MUST NOT import anything under `internal/usage`, `internal/routing`, `internal/pick`, or `internal/catalog/identity`.

Cross-feature symbols used (cited verbatim from their owning features):

```go
// F02 — internal/decimal, specs/features/F02-decimal/CONTRACTS.md
func decimal.Parse(s string) (decimal.Decimal, error) // wraps shopspring NewFromString

// F05 — internal/security, specs/features/F05-security/CONTRACTS.md
func security.ReadBoundedFile(path string, maxBytes int64) ([]byte, fs.FileMode, error)
// distinct errors for missing vs oversized files (missing → ErrMissingFile,
// oversized → ErrFileTooLarge in csvstore)
```

## 1. Constants and package-level values

File: `internal/catalog/csvstore/read.go` (constants and column lists).

```go
const (
    BenchmarkColumnPrefix = "benchmark:" // pipeline spec §3.1; Python BENCHMARK_COLUMN_PREFIX
    ProvenancePrefix      = "# which-model-scores-provenance" // annex-b §6.2a comment-line keyword
    MaxCsvBytes           = 16 << 20 // 16 MiB; csvstore's own read bound (SPEC §4)
    DefaultBackupKeep     = 5        // SPEC §4 backup rotation default
)

// RawCoreColumns is the fixed core-column order of available_model_raw_values.csv
// (pipeline spec §3.1, model_types.py:10-19). First 8 columns of every raw CSV.
var RawCoreColumns = []string{
    "model",
    "reasoning",
    "intelligence_index",
    "time_per_intelligence_index_task_seconds",
    "cost_per_intelligence_index_task_usd",
    "median_end_to_end_response_time_seconds",
    "artificial_analysis_coding_index",
    "artificial_analysis_agentic_index",
}

// CategoryScoreColumns is the fixed 12-category order of the scores CSV
// (annex-b §4.8; pipeline spec §3.2, generate_scores.py:67-80).
var CategoryScoreColumns = []string{
    "reasoning_score",
    "knowledge_score",
    "research_score",
    "planning_capability_score",
    "instruction_following_score",
    "software_engineering_score",
    "ui_visual_score",
    "agentic_tools_score",
    "finance_score",
    "evidence_capture_score",
    "security_score",
    "data_ml_score",
}

// NonNegativeRawColumns names the raw-CSV metric columns whose cells must be
// >= 0 (csv_store.py:34-38 NONNEGATIVE_RAW_COLUMNS).
var NonNegativeRawColumns = map[string]bool{
    "time_per_intelligence_index_task_seconds": true,
    "cost_per_intelligence_index_task_usd":     true,
    "median_end_to_end_response_time_seconds":  true,
}
```

## 2. Types

File: `internal/catalog/csvstore/read.go`.

```go
// Row is one CSV data row. Header and Values are positionally aligned:
// Values[i] is the cell for column Header[i]. A blank cell is "".
// Authoritative records benchmark names this row's producer explicitly
// re-scoped/cleared this refresh; MergeRows uses it for the clear-vs-fallback
// rule (SPEC §2.6). May be nil.
type Row struct {
    Header        []string
    Values        []string
    Authoritative map[string]bool
}

// Provenance is the parsed `# which-model-scores-provenance …` comment line of
// a scores CSV (SPEC §2.7, annex-b §6.2a). nil means provenance-unknown,
// never stale. Normalizer and Aggregator are "" when the tokens were absent.
type Provenance struct {
    RawSHA256  string // lowercase hex sha256, 64 chars; required token
    Normalizer string // optional `normalizer=` token, verbatim
    Aggregator string // optional `aggregator=` token, verbatim
}
```

## 3. Sentinel errors

File: `internal/catalog/csvstore/errors.go`. All are `var` errors; every function wraps them with `fmt.Errorf("%w: …", sentinel, …)`.

```go
var (
    ErrMissingFile        = errors.New("csv file missing")
    ErrFileTooLarge       = errors.New("csv file too large")
    ErrMalformedCSV       = errors.New("malformed csv")
    ErrDuplicateIdentity  = errors.New("duplicate model/reasoning identity")
    ErrChangedDuringWrite = errors.New("csv file changed while data was being collected")
)
```

## 4. Functions

### 4.1 Read

File: `internal/catalog/csvstore/read.go`.

```go
// Read reads the CSV at path (bounded by MaxCsvBytes via
// security.ReadBoundedFile). A single leading `#` comment line, if present,
// must start with ProvenancePrefix and parse as whitespace-separated
// key=value tokens: raw_sha256 required (64 lowercase hex), normalizer and
// aggregator optional verbatim strings, unknown keys skipped; any other shape
// or a second leading `#` line is ErrMalformedCSV. No comment line →
// provenance == nil. All data rows share header; each row's Header is that
// header. Cell text is verbatim.
// Errors: ErrMissingFile, ErrFileTooLarge, ErrMalformedCSV.
func Read(path string) (rows []Row, provenance *Provenance, err error)
```

### 4.2 WriteAtomic

File: `internal/catalog/csvstore/write.go`.

```go
// WriteAtomic atomically replaces path with the rendered rows. If provenance
// is non-nil its comment line is written as the first line before the header:
// "# which-model-scores-provenance raw_sha256=<hex>" plus " normalizer=<n>" /
// " aggregator=<a>" when those fields are non-empty; RawSHA256 must be 64
// lowercase hex (ErrMalformedCSV). Algorithm (SPEC §2.3): read current bytes
// (ErrMissingFile if absent); render content; write temp file in the same
// directory (prefix ".<name>."), flush + fsync; re-read path and verify it
// still equals the step-1 bytes (ErrChangedDuringWrite); os.Rename over path.
// On any error the temp file is removed and path is untouched. Structural
// validation only: non-empty rows, uniform headers across rows, Values length
// == Header length (ErrMalformedCSV).
func WriteAtomic(path string, rows []Row, provenance *Provenance) error

// WriteAtomicBytes is WriteAtomic with opaque content: no provenance parsing,
// no header rendering, no validation of content. Callers (F09 scores Derive
// via F23) supply the complete bytes including the §6.2a provenance line. Same
// step-1..step-5 replace primitive and same errors (ErrMissingFile,
// ErrChangedDuringWrite).
func WriteAtomicBytes(path string, content []byte) error
```

### 4.3 Backup

File: `internal/catalog/csvstore/backup.go`.

```go
// Backup copies path to "<name>.<UTCstamp>.bak" (stamp format
// 20060102T150405.000000Z) with exclusive create and collision suffix ".1",
// ".2", …, fsyncs the copy, then rotates siblings matching "<name>.*.bak" to
// keep the `keep` most recent (newest-first by name; fixed-width UTC stamps
// sort lexicographically). keep < 1 → error. Returns the backup path.
// Errors: ErrMissingFile.
func Backup(path string, keep int) (backupPath string, err error)
```

### 4.4 CollapseRows

File: `internal/catalog/csvstore/merge.go`.

```go
// CollapseRows ports _collapse_default_reasoning minus name cleaning
// (SPEC §2.5): groups rows by (model, collapseReasoning(reasoning)) in
// first-seen order; the output row is the first non-"default" member (else the
// first member) with, per non-identity non-benchmark column, its own value if
// non-blank else the first non-blank member value; per benchmark column the
// max of the members' non-blank values (decimal comparison via decimal.Parse);
// Authoritative is the union. Blank model cell → ErrMalformedCSV.
func CollapseRows(rows []Row) ([]Row, error)
```

### 4.5 MergeRows

File: `internal/catalog/csvstore/merge.go`.

```go
// MergeRows ports merge_rows (SPEC §2.6): CollapseRows on both inputs; index
// existing by identity (model, collapseReasoning(reasoning)); for each fresh
// row, non-identity non-benchmark columns take fresh if non-blank else
// existing; benchmark cells take fresh if non-blank, else existing when the
// fresh cell is blank and the name is not in fresh's Authoritative set, else
// blank. Fresh-only identities appended as-is; existing-only identities
// dropped. Output rows carry the fresh dataset's header.
func MergeRows(existing, fresh []Row) ([]Row, error)
```

### 4.6 MergePartialRefresh

File: `internal/catalog/csvstore/merge.go`.

```go
// MergePartialRefresh ports merge_partial_refresh: MergeRows, then, only when
// preserveUnselected is true, appends every collapsed existing row whose model
// is not in refreshedModels, re-mapped onto the fresh header by column name
// (names absent from the fresh header become blank).
func MergePartialRefresh(existing, fresh []Row, refreshedModels []string, preserveUnselected bool) ([]Row, error)
```

### 4.7 Validation

File: `internal/catalog/csvstore/validate.go`.

```go
// ValidateRows: non-empty; uniform headers; Values length == Header length;
// model and reasoning cells non-blank; no duplicate (model, reasoning)
// identity (ErrDuplicateIdentity). Errors: ErrMalformedCSV, ErrDuplicateIdentity.
func ValidateRows(rows []Row) error

// ValidateRawHeader: first 8 columns are exactly RawCoreColumns in order;
// extras all start with BenchmarkColumnPrefix and have non-empty names; no
// duplicate benchmark names. Errors: ErrMalformedCSV.
func ValidateRawHeader(header []string) error

// ValidateRawRows: ValidateRows + ValidateRawHeader on rows[0].Header; every
// non-blank cell in a numeric column (raw metric columns 2..7 and every
// benchmark: column) parses via decimal.Parse; the NonNegativeRawColumns cells
// must not be negative. Errors: ErrMalformedCSV.
func ValidateRawRows(rows []Row) error
```

### 4.8 ResolveBenchmarkColumns

File: `internal/catalog/csvstore/validate.go`.

```go
// ResolveBenchmarkColumns expands the dynamic benchmark:<name> column list:
// each group's names in argument order, then direct names, de-duplicated
// keeping first occurrence (pipeline spec §3.3 / model_config.py:70-78).
func ResolveBenchmarkColumns(groupBenchmarks [][]string, direct []string) []string
```

### 4.9 ProvenanceHash

File: `internal/catalog/csvstore/provenance.go`.

```go
// ProvenanceHash returns the lowercase hex sha256 of path's exact on-disk
// bytes (bounded read). Order-sensitive by design (SPEC §2.7).
// Errors: ErrMissingFile, ErrFileTooLarge.
func ProvenanceHash(path string) (string, error)
```

### 4.10 StaleCheck / StaleWarning

File: `internal/catalog/csvstore/provenance.go`.

```go
// StaleCheck reports whether the scores CSV at scoresPath was derived from a
// raw CSV different from the current bytes at rawPath. Provenance-unknown
// scores (no comment line) → (false, nil). Missing file on either side → error
// (ErrMissingFile). Never a hard error for callers beyond that: a stale result
// is a warning, not a failure (SPEC §2.7).
func StaleCheck(scoresPath, rawPath string) (stale bool, err error)

// StaleWarning returns the exact single-line warning callers emit when
// StaleCheck reports stale; it names both artifact paths and instructs the
// operator to run --refresh-scores. Exact string (tests assert equality):
//
//	stale scores CSV <scoresPath>: its recorded raw CSV hash does not match the
//	current <rawPath>; run --refresh-scores to regenerate
func StaleWarning(scoresPath, rawPath string) string
```

## 5. Config keys, flags, exit codes, JSON

- Config keys owned: none.
- Flags owned: none.
- Exit codes added: none (errors map to exit 1 at the CLI layer, `specs/global/SPEC.md §5`).
- `Failure.Code` values added: none.
- JSON shapes emitted: none (csvstore writes CSV text only).
- Build variants: package MUST compile and pass tests under `go build -tags nousage` (annex-b §0).
