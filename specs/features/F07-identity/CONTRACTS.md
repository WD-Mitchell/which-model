---
kind: feature-contracts
feature: F07-identity
version: "1.0"
project: which-model
---

# F07 — identity: Contracts

Package: `internal/catalog/identity` (module `github.com/WD-Mitchell/which-model`).

Imports: stdlib only (`strings`, `regexp`, `unicode`). MUST NOT import any other `internal/...` package (global CONTRACTS §8 — leaf). Compiles and passes tests under `-tags nousage`.

## 1. Constants and package-level values

File: `internal/catalog/identity/identity.go`.

```go
// EffortOrder ranks the reasoning-effort ladder (SPEC §2.6); derived from the
// ParseEffort regexes' ladder (get_benchmarks.py:42-59).
var EffortOrder = map[string]int{
    "minimal": 0,
    "low":     1,
    "medium":  2,
    "high":    3,
    "xhigh":   4,
    "max":     5,
}

// ReasoningLevels is the set of valid reasoning levels including "default"
// (SPEC §2.6). Consumers use it for validation (F09) and explain output (F10).
var ReasoningLevels = map[string]struct{}{
    "default": {},
    "minimal": {},
    "low":     {},
    "medium":  {},
    "high":    {},
    "xhigh":   {},
    "max":     {},
}
```

File: `internal/catalog/identity/benchmark.go`.

```go
// BenchmarkAliases is the verbatim Python alias dict (SPEC §2.5,
// generate_scores.py:122-129). Only "gdpvalaa" → "gdpval" is an effective
// collapse; the identity entries are kept verbatim for parity with the source.
var BenchmarkAliases = map[string]string{
    "financeagent":                       "financeagent",
    "gdpval":                             "gdpval",
    "gdpvalaa":                           "gdpval",
    "humanityslastexam":                  "humanityslastexam",
    "artificialanalysiscodingindex":      "artificialanalysiscodingindex",
    "artificialanalysiscodingagentindex": "artificialanalysiscodingagentindex",
}
```

## 2. Types

File: `internal/catalog/identity/identity.go`.

```go
// Identity is the canonical catalog row identity: a cleaned model name plus a
// collapsed reasoning level. It is a comparable struct and a legal Go map key
// (SPEC §2.3). csvstore's merge key is this pair after collapse.
type Identity struct {
    Model     string
    Reasoning string
}
```

## 3. Functions

### 3.1 CleanModelName

File: `internal/catalog/identity/clean.go`.

```go
// CleanModelName removes balanced (), [], {} annotation groups from a model
// display name and normalizes whitespace (SPEC §2.1, verbatim port of
// clean_model_name, model_types.py:27-59). Total: never errors; may return "".
//   - openers ( [ { push a stack and are dropped
//   - closers ) ] } pop only when the top matches; mismatched closers are
//     discarded (annotation punctuation is always dropped)
//   - any other rune is kept only while the stack is empty (an unmatched
//     opener suppresses the rest of the malformed annotation)
//   - the kept runes are joined with strings.Fields + strings.Join(" ")
func CleanModelName(value string) string
```

### 3.2 CollapseReasoning / IdentityKey

File: `internal/catalog/identity/identity.go`.

```go
// CollapseReasoning treats a provider/API default configuration as the
// high-effort row (SPEC §2.2, pipeline spec §4.2): "default" → "high", every
// other value verbatim. Total.
func CollapseReasoning(level string) string

// IdentityKey builds the canonical identity: CleanModelName(model) with
// CollapseReasoning(reasoning) (SPEC §2.3).
func IdentityKey(model, reasoning string) Identity
```

### 3.3 ParseEffort

File: `internal/catalog/identity/effort.go`.

```go
// ParseEffort extracts a reasoning-effort level from a variant string (SPEC
// §2.4, verbatim port of _effort, get_benchmarks.py:42-59). ok == false means
// no effort annotation (Python None) — a normal outcome, not an error.
//
// Normalization: strings.ToLower, "_" → " ", "-" → " ", strings.TrimSpace.
// Then, in order:
//
//	^(minimal|low|medium|high|xhigh|max)(?: effort| reasoning)?(?:, (?:context compaction|with tools))?$
//	^reasoning effort (none|minimal|low|medium|high|xhigh|max)(?:, (?:context compaction|with tools))?$
//
// A match returns ("default", true) when the captured effort is "none", else
// (captured, true).
func ParseEffort(variant string) (level string, ok bool)
```

### 3.4 BenchmarkKey

File: `internal/catalog/identity/benchmark.go`.

```go
// BenchmarkKey returns the stable deduplication key for a benchmark name
// (SPEC §2.5, verbatim port of _benchmark_key, generate_scores.py:117-133):
// ToLower; U+2019 and backtick → "'"; keep alphanumeric runes only
// (unicode.IsLetter || unicode.IsDigit); then BenchmarkAliases lookup
// (unknown keys map to themselves). Total.
func BenchmarkKey(name string) string
```

## 4. Config keys, flags, exit codes, JSON

- Config keys owned: none.
- Flags owned: none.
- Exit codes added: none.
- `Failure.Code` values added: none.
- JSON shapes emitted: none.
- Build variants: package MUST compile and pass tests under `go build -tags nousage` (annex-b §0).
