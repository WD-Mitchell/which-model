---
kind: feature-spec
version: "1.0"
feature: B11-history
project: which-model-desktop
---

# B11-history — Pick History Aggregation

## 1. Purpose

Pure functions over the CLI's append-only pick log `<StateDir>/pick/history.jsonl` (path composed by callers from the injected `config.Paths.StateDir`; B11 itself takes explicit paths). `AggregatePicks` produces the per-profile `Picks`/`LastUsed` stats B03 attaches to profile lists; `AppendPick` is the writer B04's `RecordPick` and B07's `Launch` use to record desktop-initiated picks in the same file, same shape, so CLI and GUI history interleave cleanly.

Depends on: nothing in `internal/service` (no `Services` receiver, no lock, no events, no config). D00's `ProfileStats` DTO is used as-is.

## 2. Behaviour

1. **Entry shape.** `PickHistoryEntry` re-declares, field-for-field with identical JSON tags, the CLI's `HistoryEntry` in `pkg/whichmodel/pick.go` (B00 SPEC §2.3 forbids importing it): `ulid`, `ts` (RFC3339), `profile`, `strategy`, `candidate_id` ("" when no pick), `final_score` (0 when no pick), `excluded_count`, `evidence`. `Evidence` is carried as `json.RawMessage` — B11 never inspects it and round-trips it verbatim (Decisions).

2. **AggregatePicks streams.** The file is read line-by-line (`bufio.Scanner` with a raised buffer limit); the whole file is never loaded at once. Empty and whitespace-only lines are ignored (not counted as skipped).

3. **Counting.** A line counts toward its profile iff it decodes, `profile` is non-empty, `ts` parses as RFC3339, and `candidate_id` is non-empty (a run that produced no pick is a valid line but not a pick — ignored, not skipped). Per profile: `Picks` = count of counting lines; `LastUsed` = the original `ts` string of the counting line with the maximum parsed time (ties keep the later line in file order).

4. **Corrupt lines.** A non-empty line that fails JSON decoding, or decodes with empty `profile`, or has an unparseable `ts`, is skipped and tallied in the returned `skipped` count. Corruption never aborts the scan and never surfaces as an error.

5. **Missing file.** `os.IsNotExist` on open → empty non-nil map, `skipped` 0, nil error — a fresh install has no history and that is not an error. Any other open/read error is returned.

6. **AppendPick.** Validates the entry minimally (`profile` non-empty, `ts` valid RFC3339 — the fields aggregation depends on); creates parent directories (0700) as needed; opens with `O_APPEND|O_CREATE|O_WRONLY` mode 0600; writes exactly one line: compact JSON + `"\n"`. A nil `Evidence` is normalised to `{}` before marshalling so every written line satisfies the CLI schema. No fsync (matches the CLI writer); no event — the caller (B04/B07) emits `pick:recorded`.

## 3. Error behaviour

- `AggregatePicks`: only I/O errors other than not-exist (open failure, scanner error) return a non-nil error; the map/skipped values are then meaningless. Data-level problems are always the `skipped` count, never errors.
- `AppendPick`: validation failures return an error with the exact messages in CONTRACTS §4 (callers wrap as `errValidation`); mkdir/open/write failures return the wrapped OS error (callers map to `io_error`). A failed write may leave a partial line; the aggregator's corrupt-line skipping (§2.4) makes that tolerable by design.

## 4. Decisions

| Decision | Value | Rationale |
|---|---|---|
| Function shape | Package-level pure funcs, explicit `path` params, no receiver/lock/events | B03/B04/B07 call under their own locking; trivially testable with `t.TempDir()` files |
| Skipped-line reporting | Second return value `skipped int` (not an error, not a callback) | Callers can log/surface it; corruption must never hide the healthy majority of the log |
| Evidence field type | `json.RawMessage`, opaque | Aggregation needs only ulid-adjacent scalars; re-declaring the CLI's nested Evidence/BandEvidence/ExcludedCandidate tree would triple the sync surface |
| "Pick" definition | `candidate_id != ""` counts; no-pick runs ignored silently | Mockup "picks" column counts successful picks; a gated/empty run is neither a pick nor corruption |
| LastUsed value | Original `ts` string of the max-time entry | Avoids re-serialisation drift (offset normalisation); D00 `ProfileStats.LastUsed` is a string |
| Shape sync | Comment-sync clause naming `pkg/whichmodel/pick.go` | B00 SPEC §2.3: re-declare, never import CLI wrappers |
