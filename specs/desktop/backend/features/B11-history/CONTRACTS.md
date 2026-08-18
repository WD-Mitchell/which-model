---
kind: feature-contracts
version: "1.0"
feature: B11-history
project: which-model-desktop
---

# B11-history — Contracts

## 1. Package and files

| File | Contents |
|---|---|
| `internal/service/history.go` | `PickHistoryEntry`, `AggregatePicks`, `AppendPick` |
| `internal/service/history_test.go` | table tests per §5 |

Package `internal/service`, but stdlib-only within this file (`bufio`, `encoding/json`, `errors`, `fmt`, `os`, `path/filepath`, `strings`, `time`). No `Services`, no config, no events, no engine imports. `ProfileStats` is the D00 canon DTO.

## 2. Exported API — `internal/service/history.go`

```go
package service

// PickHistoryEntry is one line of <StateDir>/pick/history.jsonl.
// KEEP IN SYNC (by convention, field-for-field, identical JSON tags) with
// HistoryEntry in pkg/whichmodel/pick.go — the CLI owns the on-disk shape;
// B00 SPEC §2.3 forbids importing it. Evidence is opaque here (SPEC §2.1).
type PickHistoryEntry struct {
    ULID          string          `json:"ulid"` // 26-char ULID
    TS            string          `json:"ts"`   // RFC3339
    Profile       string          `json:"profile"`
    Strategy      string          `json:"strategy"`
    CandidateID   string          `json:"candidate_id"` // "" when no pick
    FinalScore    float64         `json:"final_score"`  // 0 when no pick
    ExcludedCount int             `json:"excluded_count"`
    Evidence      json.RawMessage `json:"evidence"` // round-tripped verbatim
}

// AggregatePicks streams the JSONL file at path and returns per-profile
// stats (SPEC §2.2–2.5): Picks counts lines with a non-empty candidate_id;
// LastUsed is the original ts string of the max-time counting line.
// skipped counts corrupt lines (bad JSON, empty profile, bad ts). Missing
// file -> empty non-nil map, 0, nil. Only real I/O errors are returned.
func AggregatePicks(path string) (stats map[string]ProfileStats, skipped int, err error)

// AppendPick validates entry (profile non-empty, ts RFC3339; messages §4),
// creates parent dirs (0700), and appends one compact JSON line + "\n" via
// O_APPEND|O_CREATE|O_WRONLY, 0600. Nil Evidence is written as {}. No
// event; the caller emits pick:recorded (SPEC §2.6).
func AppendPick(path string, entry PickHistoryEntry) error
```

## 3. Paths and cross-feature use

- Canonical location: `filepath.Join(paths.StateDir, "pick", "history.jsonl")` — composed by callers (B03 for stats, B04 `RecordPick`, B07 `Launch`) from the injected `config.Paths`; B11 hardcodes nothing.
- Callers wrap §4 validation errors as `errValidation` (`validation_failed`) and OS errors as `io_error`; B11 returns plain errors.

## 4. Error strings (exact)

| Condition | String |
|---|---|
| `AppendPick` empty profile | `history: entry profile must not be empty` |
| `AppendPick` bad timestamp | `history: entry ts %q is not RFC3339` |

OS failures wrap the underlying error: `history: %w`.

## 5. Test fixtures (`history_test.go`)

All cases use `t.TempDir()` files; no `newTestServices`. A shared fixture JSONL (string constant in the test file) with lines covering: two profiles with multiple picks each, out-of-order timestamps, differing UTC offsets for the same instant ordering, a no-pick line (`candidate_id: ""`), a blank line, a truncated-JSON line, a line with empty `profile`, and a line with `ts: "yesterday"`. Required cases:

1. **Golden aggregation**: fixture → exact expected `map[string]ProfileStats` (Picks counts exclude the no-pick line; LastUsed is the max-time line's original ts string) and `skipped == 3` (truncated JSON, empty profile, bad ts — blank line NOT counted).
2. **Missing file**: nonexistent path → empty non-nil map, 0 skipped, nil error.
3. **Empty file**: zero-byte file → empty map, 0, nil.
4. **Append round-trip**: AppendPick twice (one entry with nil Evidence, one with a populated raw object) → file has exactly 2 lines; re-reading via AggregatePicks yields Picks 2 for that profile; nil Evidence line contains `"evidence":{}` and the populated one round-trips byte-identically.
5. **Append creates parents**: path with two missing directory levels → file created, dirs 0700, file 0600 (permission assertions unix-only via build tag or runtime.GOOS guard).
6. **Append validation**: empty profile and bad ts → exact §4 messages; file not created.
7. **CLI interleaving**: a fixture line copied verbatim from the CLI writer's output shape (full evidence object per `pkg/whichmodel/pick.go`) decodes and counts — guards the comment-sync clause.
