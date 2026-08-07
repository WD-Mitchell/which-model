---
kind: feature-spec
feature: F07-identity
version: "1.0"
project: which-model
---

# F07 — identity

## 1. Purpose

`internal/catalog/identity` is the catalog's identity layer: it turns a provider's display name, reasoning annotation, and benchmark name into the canonical keys the rest of the pipeline keys on. It owns model-name cleaning (`clean_model_name`), reasoning-effort parsing and the `default→high` collapse, and benchmark alias keys (`_benchmark_key`).

The port targets are the Python helpers in `available-model-data-export/.github/workflows/update_available_model_data/`: `clean_model_name` (`model_types.py:27-59`), `_effort` (`get_benchmarks.py:42-59`), and `_benchmark_key` (`generate_scores.py:117-133`), as specified by `docs/plan/research/model-data-pipeline-spec.md §4.2, §4.5` and `docs/plan/annex-b-catalog-port.md §3.1`.

The package is pure and total: no I/O, no config, no error returns. Consumers are F08 (collectors clean at ingestion), F09 (scores derivation), and F10 (explain output).

## 2. Behaviour

### 2.1 Model-name cleaning (`CleanModelName`)

`CleanModelName(value string) string` ports `clean_model_name` (`model_types.py:27-59`) exactly:

- Iterate the string **by rune** (Python iterates code points).
- `(`, `[`, `{` push onto a stack and are dropped from the output.
- `)`, `]`, `}` close: if the stack is non-empty **and** the top matches the closer's opening delimiter, pop; otherwise the closer is discarded (a malformed/mismatched closer never opens output). Annotation punctuation is discarded in every case.
- Any other rune is kept only while the stack is empty — an unmatched opener suppresses the remainder of that malformed annotation (its closer never arrives, or a mismatched closer does not pop it).
- Finally the kept runes are whitespace-normalized: `strings.Fields` + `strings.Join(..., " ")` (leading/trailing whitespace trimmed, interior runs collapsed to single spaces — Python `" ".join("".join(kept).split())`).

The function is **total**: it never errors and returns a (possibly empty) string. It removes only balanced annotation groups; the actual model display name is never reordered or truncated. Real pinned inputs: `"Claude Opus 4.5 [claude-opus-4-5-20251101]"` → `"Claude Opus 4.5"`, `"Claude Opus 4.5 (latest)"` → `"Claude Opus 4.5"` (`tests/test_update_raw_values.py:622,627`, annex-b §3.1).

### 2.2 Reasoning collapse (`CollapseReasoning`)

`CollapseReasoning(level string) string` ports `_normalise_reasoning_level` (`get_aa_api_values.py:231-233`): `"default"` → `"high"`, every other value returned verbatim (pipeline spec §4.2 — a provider/API default configuration is the high-effort row). It is **total** — unknown values pass through unchanged; validation of reasoning values happens at parse boundaries (F08 collectors, F09 scores), never here.

### 2.3 Identity key (`IdentityKey`, `Identity`)

```go
type Identity struct { Model, Reasoning string }
```

`IdentityKey(model, reasoning string) Identity` = `{Model: CleanModelName(model), Reasoning: CollapseReasoning(reasoning)}`. `Identity` is a comparable struct and therefore a legal Go map key; the whole catalog keys rows by identity on it (F06 merge key `(model, reasoning)` is this pair after collapse, csvstore's `identityOf`). Two provider snapshots that differ only by annotation (`"Claude Opus 4.5 [claude-opus-4-5-20251101]"` vs `"Claude Opus 4.5 (latest)"`) collapse to one identity; `("Example","default")` and `("Example","high")` collapse to the same identity (pipeline spec §4.2).

### 2.4 Reasoning-effort parsing (`ParseEffort`)

`ParseEffort(variant string) (level string, ok bool)` ports `_effort` (`get_benchmarks.py:42-59`). `ok == false` means "no effort annotation" (Python returns `None` — this is a normal outcome, not an error). Algorithm:

1. `strings.ToLower(variant)` (Python `casefold`; equivalent for the ASCII ladder, see §4), replace `_` and `-` with spaces, `strings.TrimSpace`.
2. Match with `regexp.MatchString` against, in order:
   - `^(minimal|low|medium|high|xhigh|max)(?: effort| reasoning)?(?:, (?:context compaction|with tools))?$`
   - `^reasoning effort (none|minimal|low|medium|high|xhigh|max)(?:, (?:context compaction|with tools))?$`
3. No match → `("", false)`. Match → `("default", true)` when the captured effort is `"none"`, else `(captured, true)`.

Pinned real inputs: `"reasoning effort xhigh"` → `("xhigh", true)` (`tests/test_model_source_boundaries.py:74`), `"reasoning effort none"` → `("default", true)`, `"high, with tools"` → `("high", true)`, `""` → `("", false)`, `"HIGH"` → `("high", true)`.

### 2.5 Benchmark alias keys (`BenchmarkKey`, `BenchmarkAliases`)

`BenchmarkKey(name string) string` ports `_benchmark_key` (`generate_scores.py:117-133`):

1. `strings.ToLower(name)` (Python `casefold`), then replace U+2019 (`’`) and backtick with `'` (the apostrophe normalization).
2. Keep only alphanumeric runes (`unicode.IsLetter || unicode.IsDigit` — Python `str.isalnum`).
3. Look up the compact string in `BenchmarkAliases`; unknown compact strings map to themselves.

`BenchmarkAliases` is the verbatim Python dict (`generate_scores.py:122-129`):

```go
var BenchmarkAliases = map[string]string{
    "financeagent":                        "financeagent",
    "gdpval":                              "gdpval",
    "gdpvalaa":                            "gdpval",
    "humanityslastexam":                   "humanityslastexam",
    "artificialanalysiscodingindex":       "artificialanalysiscodingindex",
    "artificialanalysiscodingagentindex":  "artificialanalysiscodingagentindex",
}
```

Only `gdpvalaa → gdpval` is an effective collapse; the remaining entries are identity mappings kept verbatim for parity with the Python source. Pinned real inputs: `"Finance Agent"` / `"FinanceAgent"` → `financeagent`, `"GDPval"` / `"GDPval-AA"` → `gdpval` (`tests/test_model_ranking.py:101-109`), `"Humanity’s Last Exam"` (U+2019) → `humanityslastexam`.

### 2.6 Exported ladder constants

`EffortOrder` (`minimal 0 … max 5`) and `ReasoningLevels` (the seven valid levels incl. `default`) are exported for F09 (ordering, validation) and F10 (explain). They are derived from the two Python regexes' ladder and `_normalise_reasoning_level`'s special case.

### 2.7 Purity and boundaries

The package imports stdlib only (`strings`, `regexp`, `unicode`) and MUST NOT import `internal/catalog/csvstore`, `internal/usage`, `internal/routing`, `internal/pick`, or any other internal package (global CONTRACTS §8 — it is a leaf). It compiles and passes tests under `go build -tags nousage` (annex-b §0). Every exported function is total; there are no sentinel errors, no exit codes, no `Failure.Code` values, and no JSON output.

## 3. Error behaviour

None: the package returns no errors. The only "not found" signal is `ParseEffort`'s `ok == false` for "no effort annotation". `CleanModelName` and `BenchmarkKey` are best-effort normalizers that always return a (possibly empty) string. This mirrors the Python: `clean_model_name` and `_benchmark_key` never raise; `_effort` returns `None` for non-matching variants.

## 4. Decisions

| Decision | Value | Rationale |
|---|---|---|
| Unmatched opener | suppresses the remainder of the malformed annotation | Verbatim `clean_model_name` (`model_types.py:27-59`): the stack never empties, so nothing after is kept. |
| Mismatched closer | discarded, never pops a non-matching opener, never opens output | Verbatim Python: `stack and stack[-1] == closing[character]`; annotation punctuation is discarded even when malformed. |
| Whitespace normalization | `strings.Fields` + join with single spaces | Python `" ".join(...split())`; display names may carry stray runs of whitespace from provider data. |
| Rune iteration | range over the string, not bytes | Python iterates code points; a multi-byte UTF-8 name must not be corrupted. |
| `CollapseReasoning` total | `"default"` → `"high"`, all other values verbatim | Pipeline spec §4.2 / `_normalise_reasoning_level`; validation lives at parse boundaries (F08/F09), not in the collapse. |
| `ParseEffort` returns `(string, bool)` | `ok == false` = no effort annotation | Python returns `None`; this is a normal outcome, not an error — the package has no error type. |
| `casefold` → `strings.ToLower` | ASCII-only equivalence | The ladder `minimal…max` and `none` are ASCII; adopt `x/text/cases` if a non-ASCII variant appears. |
| NFKC skipped in `BenchmarkKey` | `ToLower` + explicit `’`/`` ` `` → `'` + alnum filter | All 51 benchmark names are ASCII; NFKC only matters when it maps a non-alnum rune to an alnum one (e.g. `①`→`1`), which none of them do. The one real non-ASCII input, U+2019 in "Humanity’s Last Exam", is handled by the explicit replacement. Adopt `x/text/unicode/norm` if a non-ASCII name appears. |
| Alias dict | verbatim 6-entry map; only `gdpvalaa → gdpval` collapses | Parity with `generate_scores.py:122-129`; identity entries cost nothing and keep the port trivially diffable against Python. |
| `Identity` is a struct of two strings | comparable → legal map key | F06's merge key and F09's grouping need exact identity equality; a struct key is the natural Go shape. |
| No error type | package is total, no I/O, no config | Leaf package per global CONTRACTS §8; consumers validate inputs at their own boundaries. |

## 5. Out of scope

- CSV storage, merging, provenance — `internal/catalog/csvstore` (F06).
- AA v2 API `configuration` string parsing (`_reasoning` / `_name_parts`, `get_aa_api_values.py:167-229`) — F08 collectors: `_reasoning` raises `UpdateError` on unrecognized configurations, which is a collector-level error, not an identity-level signal; F07 exposes only `ParseEffort` for variant strings.
- Reasoning validation on the raw-CSV read path and legacy hand-annotated name handling — F09.
- Scoring, category composites, benchmark normalization — F09.
- `ProviderModel`/`_provider_keys` tokenization — F08.
- Any I/O, config, flags, or CLI surface — F07 is a pure leaf.
