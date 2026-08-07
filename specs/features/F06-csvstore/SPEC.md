---
kind: feature-spec
feature: F06-csvstore
version: "1.0"
project: which-model
---

# F06 — csvstore

## 1. Purpose

`internal/catalog/csvstore` is the catalog's storage layer: atomic CSV persistence (temp file + fsync + rename), timestamped backups with bounded rotation, incremental identity-keyed merging of fresh and existing rows, and raw-CSV content hashing for staleness detection between the raw and scores artifacts. It is deliberately identity-agnostic — it never imports `internal/catalog/identity` — so it can sit at the bottom of the catalog stack per the dependency graph (`specs/DEPENDENCY-GRAPH.md` row F06: depends_on F02, F05).

The port target is `csv_store.py` (`available-model-data-export/.github/workflows/update_available_model_data/csv_store.py`, 274 lines) as specified by `docs/plan/annex-b-catalog-port.md §6` and `docs/plan/research/model-data-pipeline-spec.md §3–§4`. The CSV column layouts are those of `available_model_raw_values.csv` (pipeline spec §3.1) and `available_model_scores.csv` (pipeline spec §3.2).

## 2. Behaviour

### 2.1 CSV file format

Every file this package reads or writes is UTF-8 text with `\n` line terminators, a header row first, then data rows. A blank cell is an empty string `""` — never the text `"null"` or `"None"`. An optional provenance comment line may precede the header (scores CSV only, §2.7); it is the literal first line of the file and has the exact shape from annex-b §6.2a:

```
# which-model-scores-provenance raw_sha256=<64-hex> normalizer=<name> aggregator=<name>
```

`normalizer=` and `aggregator=` are optional (absent when unknown/not recorded); `raw_sha256=` is required on any provenance line. (Mechanism: annex-b §6.2a — "a header comment line, not a sidecar file or a metadata column".)

`Row` is positionally structured: `Values[i]` is the cell for `Header[i]`. The header is carried per row so a `Row` is self-describing. Cells are kept verbatim — `Read` never trims or re-renders cell text (rendering `Decimal` values to cells is the scoring/collector layer's job via F02, not csvstore's).

### 2.2 Bounded I/O

Every file read in this package goes through `security.ReadBoundedFile(path, MaxCsvBytes)` from F05 (`internal/security`, pinned API: `func ReadBoundedFile(path string, maxBytes int64) ([]byte, fs.FileMode, error)`, distinct errors for missing vs oversized). `MaxCsvBytes = 16 << 20` (16 MiB): CSVs are data, not credentials, and will outgrow the 1 MiB credential bound. This is a global decision (Main, pinned for F06).

### 2.3 Atomic write (`WriteAtomic`)

`WriteAtomic(path, rows, provenance)` replaces `path` atomically, porting `csv_store.py:240-273` (`replace_with_backup`) minus the backup step (which is the separate `Backup`, §2.4):

1. Read the current file at `path` (bounded, §2.2). If it does not exist → `ErrMissingFile` — this is a *replace*, not a create, mirroring Python's `"existing raw CSV not found"`.
2. Render the full content (optional provenance comment line + header + data rows) to bytes.
3. Write to a temp file in the same directory (prefix `.<name>.`), flush and `fsync` the temp file.
4. Re-read `path`; if its bytes no longer equal the step-1 read → `ErrChangedDuringWrite` ("changed while data was being collected").
5. `os.Rename` the temp file over `path`.

Any failure before step 5 leaves `path` byte-identical to what it was before the call started, and removes the temp file. This is the guarantee tests assert directly (annex-b §6.4, "atomic backup/replace" group, `test_update_raw_values.py` `test_failed_update_leaves_original_and_no_backup`-style invariants).

The step-4 check is an internal compare-and-swap. The pre-collection `expected_original` CAS from Python is deliberately absent from csvstore: the refresh pipeline is single-writer (one process runs Collect→Derive), so a change between collection start and write implies external interference, which the step-4 check still catches at rename time. Recorded in §4.

The provenance parameter is part of the same atomic content (a single render+rename), so a scores CSV and its `raw_sha256` comment can never drift (§2.7).

`WriteAtomicBytes(path, content)` is the same atomic replace with opaque content — no provenance parsing, no header rendering, no structural validation of `content`. It exists for the scores-Derive seam (F09/F23): Derive assembles the complete scores bytes including the §6.2a provenance line, and the CLI writes them verbatim without a parse-render round-trip (which would risk byte-format drift). Both entry points share one internal replace primitive (bounded read of the original → temp in the same directory → fsync → step-4 re-read CAS → rename).

### 2.4 Backup and rotation (`Backup`)

`Backup(path, keep)` copies `path` to a timestamped backup before a refresh replaces it:

- Stamp format `20060102T150405.000000Z` (UTC, microsecond — Python `%Y%m%dT%H%M%S%fZ`), backup name `<name>.<stamp>.bak`, exclusive create (`O_CREATE|O_EXCL`) with collision suffix `.1`, `.2`, … (verbatim `_backup_path`, `csv_store.py:236-247`).
- The backup file is written, flushed and `fsync`ed before rotation.
- Rotation: after the new backup exists, list sibling files matching `<name>.*.bak`, sort newest-first (lexicographic order works because the stamp is fixed-width UTC), keep the `keep` most recent, delete the rest. `keep < 1` is an error. `DefaultBackupKeep = 5`.

Python keeps every `.bak` forever; the port bounds the pile. `keep` is a parameter so callers may choose; the default constant is 5 (§4).

### 2.5 Identity collapse on read (`CollapseRows`)

`CollapseRows(rows)` ports `_collapse_default_reasoning` (`csv_store.py:124-178`) minus name cleaning:

- Group rows by identity `(model, collapseReasoning(reasoning))` in first-seen order, where `collapseReasoning("default") == "high"` and every other reasoning value is unchanged (pipeline spec §4.2).
- Within a group: the output row is the first non-`default` row, else the first row. For every non-identity, non-benchmark column the base's value wins if non-blank, else the first non-blank value among the group's rows in group order. For every benchmark column the value is the maximum over the group's non-blank values (decimal comparison via F02 `decimal.Parse`).
- `Authoritative` is the union of the group members' sets.
- A blank `model` cell is an error.

This reproduces the duplicate-evidence merge Python applies when reading a raw CSV, without needing `internal/catalog/identity` (see §2.10).

### 2.6 Incremental merge (`MergeRows`, `MergePartialRefresh`)

`MergeRows(existing, fresh)` ports `merge_rows` (`csv_store.py:181-209`), annex-b §3.4:

1. Both inputs pass through `CollapseRows` (§2.5) first.
2. Index existing rows by identity.
3. For each fresh row: if an existing row shares its identity, every non-identity, non-benchmark column takes **fresh if non-blank, else existing** — a "refresh" merge, not a max-merge; zero is a valid win and is never treated as blank (pipeline spec §4.4: "zero is a valid win, never falsy-skipped"). Benchmark cells: fresh non-blank wins; a fresh blank cell falls back to the existing value **unless** the benchmark name is in the fresh row's `Authoritative` set (an explicit clear).
4. Fresh-only identities are appended as-is. **Existing-only identities are dropped** — they survive only through `MergePartialRefresh` with `preserveUnselected=true` (this is exactly Python's behaviour; `--provider`-narrowed partial runs keep unrefreshed providers' rows).

`MergePartialRefresh(existing, fresh, refreshedModels, preserveUnselected)` ports `merge_partial_refresh` (`csv_store.py:204-209`): runs `MergeRows`, then — only when `preserveUnselected` is true — appends every existing row whose model is not among `refreshedModels`, unchanged except re-mapped onto the fresh dataset's header (values by column name; names absent from the fresh header become blank). Emitting every row under the fresh header keeps the writer's uniform-header invariant; a benchmark name present only in the old file is dropped, which is correct because `benchmarks.toml` is authoritative for the dynamic column set (annex-b §6.3, §4).

The merge key is fixed: `(model, reasoning)` after `default→high` collapse (pipeline spec §4.2). There is no arbitrary-key-column API — the pipeline never merges on any other key.

### 2.7 Provenance hash and staleness (`ProvenanceHash`, `StaleCheck`, `StaleWarning`)

The scores CSV records the sha256 of the raw CSV it was derived from, plus the `Normalizer`/`Aggregator` names already required by annex-b §4.0, as a single comment line written as part of the same atomic content (§2.3) — annex-b §6.2a:

```
# which-model-scores-provenance raw_sha256=<64-hex> normalizer=<name> aggregator=<name>
```

- `ProvenanceHash(path)` returns the lowercase hex sha256 of the file's **exact on-disk bytes** (read bounded, §2.2). It is computed once at the end of Derive and never recomputed from a re-serialization of parsed rows — a re-serialization could drift in byte format and defeat the hash (annex-b §6.2a). Consequently the hash is **order-sensitive**: two files with the same rows in different row orders hash differently.
- `Read` strips and parses exactly one leading `#`-prefixed line. If the line is present it MUST start with `ProvenancePrefix = "# which-model-scores-provenance"` and parse as whitespace-separated `key=value` tokens: `raw_sha256` is required and must be 64 hex chars; `normalizer=`/`aggregator=` are optional string tokens preserved verbatim into `Provenance{Normalizer, Aggregator}` (empty when absent); tokens with an unknown key are skipped (forward-compatible reads); a malformed shape (not `key=value`, empty key or value, non-hex or short `raw_sha256`) is a hard error (`ErrMalformedCSV`); a second leading `#` line is a hard error. A file with no comment line at all is **provenance-unknown** (`provenance == nil`), not stale.
- `StaleCheck(scoresPath, rawPath)` reads the scores CSV's provenance and compares `RawSHA256` against `ProvenanceHash(rawPath)`: mismatch → `stale = true`; unknown provenance → `stale = false`; a missing file on either side is a hard error (the comparison is impossible).
- Staleness is **never a hard error** for the caller: a stale scores CSV is still usable and `which-model pick` degrades exactly as it does for a stale route table (annex-b §6.2a / §7.2). `StaleWarning(scoresPath, rawPath)` returns the single warning string naming both artifact paths and instructing the operator to run `--refresh-scores`; callers that read a scores CSV emit exactly this one warning when `StaleCheck` reports stale.
- `WriteAtomic` renders the line from `Provenance{RawSHA256, Normalizer, Aggregator}`: `# which-model-scores-provenance raw_sha256=<hex>` plus ` normalizer=<name>` / ` aggregator=<name>` when non-empty. The scores Derive seam (F09 → F23) bypasses the renderer via `WriteAtomicBytes` (§2.3) and emits the line verbatim — both paths must produce the identical §6.2a shape, which `Read` and the F09 golden tests pin.

### 2.8 Dynamic benchmark columns (`ResolveBenchmarkColumns`)

The `benchmark:` column set is fully dynamic and MUST NOT be hardcoded (annex-b §6.3, pipeline spec §3.3). `ResolveBenchmarkColumns(groupBenchmarks, direct)` is the single place that computes the list: concatenate each group's names in argument order, then the direct names, de-duplicating keeping first occurrence (`dict.fromkeys` semantics, `model_config.py:70-78`). It takes plain string slices rather than an `internal/config` type because F06 does not depend on F01; the config layer (F08/F23) supplies the slices.

### 2.9 Validation (`ValidateRows`, `ValidateRawHeader`, `ValidateRawRows`)

- `ValidateRows(rows)` — structural + identity: rows non-empty; every row shares the first row's header; every `Values` slice matches the header length; `model` and `reasoning` cells non-blank; no duplicate `(model, reasoning)` identity (raw CSV row identity is unique — a duplicate is a hard validation error, pipeline spec §3.1 / `validate_complete_rows`).
- `ValidateRawHeader(header)` — raw-CSV schema: the first 8 columns are exactly `RawCoreColumns` in order; every extra column starts with `benchmark:` and has a non-empty name; no duplicate benchmark columns (ports the parse-time checks in `parse_existing_csv`, `csv_store.py:88-103`).
- `ValidateRawRows(rows)` — `ValidateRows` + `ValidateRawHeader` + numeric checks: every non-blank cell in a numeric column parses via F02 `decimal.Parse`; the three non-negative columns (`time_per_intelligence_index_task_seconds`, `cost_per_intelligence_index_task_usd`, `median_end_to_end_response_time_seconds`) must not be negative (ports `validate_complete_rows` / `_decimal`).

### 2.10 Identity-agnostic boundary

`internal/catalog/csvstore` MUST NOT import `internal/catalog/identity` (or anything under `internal/usage`, `internal/routing`, `internal/pick` — global CONTRACTS §8). Consequences, all deliberate:

- `CollapseRows`/`MergeRows` collapse only `default→high` reasoning; they do not clean model names. Callers guarantee names are already clean: the collectors (F08) clean at ingestion via `identity.CleanModelName`, and the raw CSV writer validates uniqueness of the stored identity (§2.9). Legacy hand-annotated names are F09's read-path concern (F09 imports both csvstore and identity).
- The package compiles and passes its full test suite under `go build -tags nousage` (annex-b §0: catalog packages MUST compile under `nousage`).

## 3. Error behaviour

All errors are returned, never logged or panicked. Sentinel errors (defined in `internal/catalog/csvstore/errors.go`), always wrapped with context via `%w`:

| Sentinel | Meaning | Wrapped message pattern |
|---|---|---|
| `ErrMissingFile` | target CSV does not exist (`Read`/`WriteAtomic`/`Backup`/`ProvenanceHash`/`StaleCheck`) | `csv file missing: <path>` |
| `ErrFileTooLarge` | file exceeds `MaxCsvBytes` (from F05's oversized-file error) | `csv file too large: <path>` |
| `ErrMalformedCSV` | not UTF-8; bad provenance line; multiple comment lines; wrong cell count; no header; no data rows; inconsistent row headers; non-numeric cell; negative non-negative column; bad benchmark header shape | `malformed csv: <detail>` |
| `ErrDuplicateIdentity` | duplicate `(model, reasoning)` in `ValidateRows` | `duplicate model/reasoning identity: <model> / <reasoning>` |
| `ErrChangedDuringWrite` | file changed between the step-1 read and the step-4 re-read in `WriteAtomic` | `csv file changed while data was being collected: <path>` |

Write failures are atomic: `WriteAtomic` never leaves a partial file at `path` and never leaves a temp file behind. Merge and validation errors carry the offending row's model/reasoning in the message where Python does (`"duplicate model/reasoning row: {model} / {reasoning}"`).

This package defines no exit codes: it returns errors; the CLI layer maps them (runtime failures → exit 1, per `specs/global/SPEC.md §5`). It adds no new `Failure.Code` values and emits no JSON.

## 4. Decisions

| Decision | Value | Rationale |
|---|---|---|
| CSV size bound | `MaxCsvBytes = 16 << 20` (16 MiB) | CSVs are data, not credentials, and outgrow 1 MiB; global decision pinned for F06. |
| Backup rotation count | `DefaultBackupKeep = 5`, `keep` parameterized, `keep < 1` errors | Python keeps every `.bak` forever; bounded rotation is the port's policy. |
| Provenance hash | sha256 over exact on-disk bytes; **order-sensitive** | Annex-b §6.2a: computed once, never recomputed from a re-serialization; byte content defines the artifact. |
| Merge conflict policy | fresh wins when non-blank (refresh merge, not max); zero is a valid win; benchmark fallback only when fresh blank and name not `Authoritative` | Annex-b §3.4 / pipeline spec §4.4. |
| Merge key columns | fixed `(model, reasoning)` after `default→high` collapse | Pipeline spec §4.2 defines identity; no other merge key exists. |
| Existing-only rows | dropped by `MergeRows`; kept only via `MergePartialRefresh(…, preserveUnselected=true)` | Verbatim Python `merge_rows` / `merge_partial_refresh` semantics; `--provider`-narrowed runs opt in. |
| Merge output header | every output row carries the fresh dataset's header; names absent from fresh become blank | Keeps the writer's uniform-header invariant; `benchmarks.toml` is authoritative for the column set. |
| Identity-agnostic | csvstore never imports `internal/catalog/identity`; collapses only `default→high` | Dependency graph F06 → F02, F05 only; cleaning is F07, applied upstream by F08. |
| `Read` signature | `Read(path) ([]Row, *Provenance, error)`; header carried per row in `Row.Header` | Scores CSV needs the provenance comment; one read path serves both artifacts. |
| Provenance in `WriteAtomic` | rendered as the first line of the same atomic content | Annex-b §6.2a mechanism: comment line, not sidecar file. |
| Provenance line format | `# which-model-scores-provenance raw_sha256=<64-hex> [normalizer=<name>] [aggregator=<name>]`, space-separated `key=value` tokens | Annex-b §6.2a verbatim line; `normalizer`/`aggregator` are the §4.0 metadata names, required to be recorded in the artifact. |
| `raw_sha256` required on any provenance line | provenance line present but hash missing/not 64-hex → `ErrMalformedCSV` | The line exists to carry the hash; a line without it indicates a broken writer and must not be silently treated as provenance-unknown. |
| Unknown provenance tokens | skipped, not an error, not preserved | Forward-compatible reads: a future field addition must not break `pick` on an existing artifact; data rows remain fully usable. |
| `Provenance` struct | `{RawSHA256 string; Normalizer, Aggregator string}`; empty string = absent | F10/`explain` consumers want the recorded normalizer/aggregator names (annex-b §6.2a). |
| Opaque write entry point | `WriteAtomicBytes(path, content)` — same CAS primitive, no render/parse | F09's Derive assembles complete scores bytes incl. the §6.2a line; F23 writes them verbatim, avoiding a parse-render byte-drift risk. |
| `WriteAtomic` requires existing file | missing target → `ErrMissingFile` | Ports Python `"existing raw CSV not found"`; atomic replace, not create. |
| CAS depth | internal step-4 verify only; no `expected_original` parameter | Single-writer pipeline; interruption-safety preserved; pre-collection CAS adds no protection here. |
| `ResolveBenchmarkColumns` input | plain `[][]string`/`[]string`, not an F01 config type | F06 does not depend on F01; config wiring lives in F08/F23. |
| Cell text | kept verbatim; no trimming, no re-rendering | Rendering `Decimal`→cell is F02/F09's job; csvstore is format-neutral. |

## 5. Out of scope

- Model-name cleaning, reasoning-ladder validation, effort parsing, benchmark alias keys — `internal/catalog/identity` (F07).
- Duplicate-evidence handling for legacy hand-annotated names on the scores read path — F09 scoring (imports both csvstore and identity).
- `Decimal`→cell rendering (`ScoreString`, 1dp/0dp/2dp formats) — F02/F09.
- Collectors, `benchmarks.toml`/`providers.toml` parsing, config wiring — F08/F01.
- Scores CSV generation, category composites, `_merge_input_rows` max-merge — F09.
- Backup invocation policy (when to call `Backup` before `WriteAtomic`) — F08/F23 orchestration.
- Staleness *policy* (warnings in `pick`/`routes`) — F10/F18/F23 consumers of `StaleCheck`.
