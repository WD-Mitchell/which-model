---
kind: feature-contracts
version: "1.0"
feature: F26-cmd-pick
project: which-model
module: github.com/WD-Mitchell/which-model
---

# F26 — cmd-pick: Contracts

## 1. Owned files

- `pkg/whichmodel/pick_cmd.go` — `pick` command, NO build tag (registered in all builds; degraded behavior per SPEC §2.4). `func init() { register(NewPickCmd) }` + `func NewPickCmd() *cobra.Command`; also registers F26's exit codes (`RegisterExitCode`, SPEC §2.5).
- `pkg/whichmodel/pick.go` — pipeline logic: profile resolution, filtering, ranking, usage/bands join, strategy, result assembly, text renderer, history append.
- `pkg/whichmodel/explain_cmd.go` — `explain` command, NO build tag (same wiring; `init() { register(NewExplainCmd) }`).
- `pkg/whichmodel/explain.go` — history read + Evidence reconstruction.
- Tests: `pkg/whichmodel/pick_cmd_test.go`, `pick_profile_test.go`, `pick_pipeline_test.go`, `pick_usage_test.go`, `pick_degraded_test.go`, `pick_strategy_test.go`, `pick_exit_test.go`, `pick_json_test.go`, `pick_history_test.go`, `explain_test.go`.

No config keys, no flags beyond the tables below are owned; F26 writes ONE state file (owned by F26): `<state_dir>/pick/history.jsonl`.

## 2. Exported API (`pkg/whichmodel`)

```go
// PickArgs is the fully-validated pick command input.
type PickArgs struct {
    Profile      string   // resolved profile id (after --task-category mapping)
    TaskCategory string   // raw --task-category (resolved in T2)
    Complexity   string   // raw --complexity
    Strategy     string   // "score" default
    Seed         *uint64  // required iff strategy == "weighted_random"
    Allowlists   []string // --available paths (raw, pre-read)
    NoUsage      bool     // Global.NoUsage
    JSON         bool     // Global.JSON; forced true when stdout is not a TTY
    ConfigPath   string   // Global.ConfigPath
}

// RouteRef is the JSON form of a route inside Candidate/ExcludedCandidate —
// annex-c §4.2 "$defs/Route" VERBATIM: required [provider, model_id, model,
// reasoning, window_ids], additionalProperties false.
type RouteRef struct {
    Provider  string   `json:"provider"`
    ModelID   string   `json:"model_id"`
    Model     string   `json:"model"`
    Reasoning string   `json:"reasoning"` // minimal|low|medium|high|xhigh|max|default
    WindowIDs []string `json:"window_ids"`
}

// Candidate mirrors annex-c §4.2 candidates[] VERBATIM.
type Candidate struct {
    CandidateID    string   `json:"candidate_id"`  // "<provider>:<model_id>" pointer
    Route          RouteRef `json:"route"`         // full Route object, not a bare id
    ModelScore     float64  `json:"model_score"`   // decimal.Round(0) → float64
    Band           string   `json:"band,omitempty"`        // omitted when usage disabled
    BandWeight     float64  `json:"band_weight,omitempty"` // omitted when usage disabled
    ProviderWeight float64  `json:"provider_weight"`
    FinalScore     float64  `json:"final_score"`
    Warnings       []string `json:"warnings"`
}

// ExcludedCandidate mirrors annex-c §4.2 excluded_candidates[] VERBATIM:
// required [route, reason_code, reason], additionalProperties false.
type ExcludedCandidate struct {
    Route      RouteRef `json:"route"`
    ReasonCode string   `json:"reason_code"` // band_gated|no_score_row|auth_required|provider_error|not_in_availability_list
    Reason     string   `json:"reason"`      // human-readable detail
}

// PickResult is the pick --json document root: annex-c §4.2 fields
// (schema_version, profile, strategy, seed, candidates, excluded_candidates)
// + §4.6 additions (usage_enabled, usage_disabled_reason) + normalizer/
// aggregator (annex-c §4.6 example) + SPEC Decisions D-17/D-18.
type PickResult struct {
    SchemaVersion       string              `json:"schema_version"`          // "2.0"
    UsageEnabled        bool                `json:"usage_enabled"`
    UsageDisabledReason *string             `json:"usage_disabled_reason"`   // null when enabled
    Profile             string              `json:"profile"`
    Strategy            string              `json:"strategy"`
    Seed                *uint64             `json:"seed"`                    // null unless weighted_random
    Normalizer          string              `json:"normalizer"`              // Global.Normalizer
    Aggregator          string              `json:"aggregator"`              // Global.Aggregator
    Candidates          []Candidate         `json:"candidates"`
    ExcludedCandidates  []ExcludedCandidate `json:"excluded_candidates"`
}

// RunPick executes the full pipeline (SPEC §2.2). Returns nil on a pick,
// UsageError on argument errors, CodedError for exit classes 1/3/4/5/2.
func RunPick(args PickArgs, stdout, stderr io.Writer) error

// HistoryEntry is one append-only line of <state_dir>/pick/history.jsonl.
type HistoryEntry struct {
    ULID          string  `json:"ulid"`            // github.com/oklog/ulid/v2, 26 chars
    TS            string  `json:"ts"`              // RFC3339
    Profile       string  `json:"profile"`
    Strategy      string  `json:"strategy"`
    Seed          *uint64 `json:"seed"`
    CandidateID   string  `json:"candidate_id"`    // "" when no pick
    FinalScore    float64 `json:"final_score"`     // 0 when no pick
    ExcludedCount int     `json:"excluded_count"`
    Evidence      Evidence `json:"evidence"`       // full annex-c §4.3 record (SPEC D-13)
}

// Evidence mirrors annex-c §4.3 "$defs/Evidence" VERBATIM: required
// [profile, score_inputs, band, snapshot_age_seconds, confidence,
// route_provenance, excluded_candidates, last_verified],
// additionalProperties false. Degraded mode omits band/snapshot_age_seconds/
// confidence/last_verified (annex-c §5.1).
type Evidence struct {
    Profile            string             `json:"profile"`
    ScoreInputs        map[string]float64 `json:"score_inputs"`   // tier1 + category composite values (numbers)
    Band               *BandEvidence      `json:"band,omitempty"` // {name, used_percent, weight}
    SnapshotAgeSeconds *int64             `json:"snapshot_age_seconds,omitempty"`
    Confidence         string             `json:"confidence,omitempty"` // live|cached; omitted in degraded mode
    RouteProvenance    string             `json:"route_provenance"`     // provider_live|models_dev|user_declared
    ExcludedCandidates []ExcludedCandidate `json:"excluded_candidates"` // full §4.2 objects
    LastVerified       string             `json:"last_verified,omitempty"` // single RFC3339 date-time
}

type BandEvidence struct {
    Name        string  `json:"name"`
    UsedPercent float64 `json:"used_percent"`
    Weight      float64 `json:"weight"`
}

// ExplainResult is the explain --json document root — annex-c §4.3 VERBATIM:
// required [schema_version, candidate, evidence].
type ExplainResult struct {
    SchemaVersion string    `json:"schema_version"` // "2.0"
    Candidate     string    `json:"candidate"`      // candidate_id echoed back
    Evidence      Evidence  `json:"evidence"`
}

// ExplainArgs selects the history record.
type ExplainArgs struct {
    Last     bool
    PickID   string // "" unless --pick-id
    JSON     bool   // Global.JSON
    ConfigPath string
}

// RunExplain emits ExplainResult for the selected record (SPEC §2.11).
func RunExplain(args ExplainArgs, stdout, stderr io.Writer) error

// FormatPickText renders the text result (SPEC §2.3.9).
func FormatPickText(res *PickResult) string
```

## 3. Flags owned

`pick`:

| Flag | Type | Default | Meaning |
|---|---|---|---|
| `--profile` | string | `""` | Profile id; REQUIRED unless `--task-category` given; validated against the 11 annex-c §2.1 names |
| `--task-category` | string | `""` | Alternative selector, must pair with `--complexity`; mutually exclusive with `--profile` |
| `--complexity` | string | `""` | `simple\|medium\|complex`; rejected for 1:1-mapped categories |
| `--strategy` | string | `"score"` | F20 registry name; `--seed` required when `weighted_random` |
| `--seed` | uint64 | 0 | Determinism seed |
| `--available` | stringSlice | `[]` | Repeatable allowlist file path; missing file → exit 2 |

`explain`:

| Flag | Type | Default | Meaning |
|---|---|---|---|
| `--last` | bool | `false` | Last history record; exactly one of `--last`/`--pick-id` |
| `--pick-id` | string | `""` | ULID to explain; exactly one of the two |

Consumed globals: `--json`, `--no-usage`, `--config` (→ `Global.ConfigPath`), `--normalizer`, `--aggregator` (→ `Global.Normalizer`/`Global.Aggregator`).

## 4. Exit codes (F26-registered via `RegisterExitCode` in `init()`)

`RegisterExitCode("no_pick", 3)`, `RegisterExitCode("usage_gated", 4)`, `RegisterExitCode("auth_required", 5)` — called in `pick_cmd.go`'s `init()` before `register(NewPickCmd)`.

| Exit | Code | Condition |
|---|---|---|
| 0 | — | ≥1 candidate picked |
| 1 | `runtime` (unknown → 1) | runtime errors outside the classes below |
| 2 | `arguments` (`UsageError`) | any argument error, table SPEC §3 |
| 2 | `usage_config` | strict no_providers misconfig (SPEC §2.14) |
| 2 | `usage_disabled` | `least_used` in degraded mode (SPEC §2.15) |
| 3 | `no_pick` | zero survivors, only `not_in_availability_list`/`no_score_row` exclusions |
| 4 | `usage_gated` | zero survivors, any `band_gated`/`provider_error`, no `auth_required` |
| 5 | `auth_required` | zero survivors, any `auth_required` exclusion |

## 5. JSON shape — `PickResult` (annex-c §4.2 + §4.6 VERBATIM)

```json
{
  "schema_version": "2.0",
  "usage_enabled": true,
  "usage_disabled_reason": null,
  "profile": "complex_implementation",
  "strategy": "score",
  "seed": null,
  "normalizer": "minmax-linear",
  "aggregator": "weighted-arithmetic-mean",
  "candidates": [
    {
      "candidate_id": "claude:claude-sonnet-4-5",
      "route": {
        "provider": "claude",
        "model_id": "claude-sonnet-4-5",
        "model": "claude-sonnet-4-5",
        "reasoning": "default",
        "window_ids": ["5h", "7d"]
      },
      "model_score": 92,
      "band": "five hour",
      "band_weight": 0.8,
      "provider_weight": 1.0,
      "final_score": 73.6,
      "warnings": []
    }
  ],
  "excluded_candidates": [
    {
      "route": {"provider": "codex", "model_id": "gpt-5-codex", "model": "gpt-5-codex", "reasoning": "default", "window_ids": []},
      "reason_code": "band_gated",
      "reason": "band usage 95% > gate 90%"
    }
  ]
}
```

- `normalizer`/`aggregator` echo `Global.Normalizer`/`Global.Aggregator` (F22 defaults `minmax-linear` / `weighted-arithmetic-mean`).
- `route` is the full Route object (annex-c §4.2 `$defs/Route`; `window_ids` from the F18 route's windows; `reasoning` passes through with the F18 default `default`).
- Numbers are JSON numbers (decimal→float64 after `Round(0)` for `model_score`; `band_weight`/`provider_weight`/`final_score` keep decimal precision as float64).
- `band`/`band_weight` are OMITTED (not null) when usage disabled (annex-c §4.6 normative text).
- `reason_code` enum is closed at the five verbatim values; adding values later is a MINOR schema change (annex-c §4.5).
- Empty arrays serialize as `[]`, never `null` (always initialized).
- The §4.6 degraded example's `recommendation`/`alternatives`/`excluded` field names are superseded by §4.2's `candidates`/`excluded_candidates` + §4.6's normative additions (SPEC Decision D-18).

## 6. JSON shape — explain (annex-c §4.3 VERBATIM)

```json
{
  "schema_version": "2.0",
  "candidate": "claude:claude-sonnet-4-5",
  "evidence": {
    "profile": "complex_implementation",
    "score_inputs": {"tier1": 84, "category": 8},
    "band": {"name": "five hour", "used_percent": 25, "weight": 0.8},
    "snapshot_age_seconds": 300,
    "confidence": "live",
    "route_provenance": "provider_live",
    "excluded_candidates": [
      {"route": {"provider": "codex", "model_id": "gpt-5-codex", "model": "gpt-5-codex", "reasoning": "default", "window_ids": []}, "reason_code": "band_gated", "reason": "band usage 95% > gate 90%"}
    ],
    "last_verified": "2026-08-07T17:03:11Z"
  }
}
```

- `candidate` = the recorded `candidate_id` (annex-c §4.3 "candidate_id echoed back"); `""` when the record had no pick.
- `score_inputs` values are NUMBERS (tier1 + category composite values from F10's score row; keys per F10 CONTRACTS — F26 copies the row's inputs map verbatim).
- `confidence` ∈ `live|cached` only in the 2.0 contract (the `estimated` enum value predates §5.1: degraded mode OMITS the field rather than emitting `estimated` — §5.1 row "snapshot_age, confidence | omitted").
- `route_provenance` = global Provenance enum `provider_live|models_dev|user_declared` (annex-c §5.1 uses the same names); degraded mode never emits `provider_live` (SPEC §2.15).
- `excluded_candidates` entries are full `ExcludedCandidate` objects (annex-c §4.3 refs the pick-result `$defs/ExcludedCandidate`).
- `last_verified` is ONE RFC3339 date-time (the picked provider's last verification), not a map.
- Degraded mode (SPEC §2.15): `band`, `snapshot_age_seconds`, `confidence`, `last_verified` omitted; `profile`, `score_inputs`, `route_provenance`, `excluded_candidates` retained.

## 7. Text layout spec

`pick` text (non-JSON, TTY):
```
picked <model_id> via <provider> (score <final_score>)
  profile: <profile>
  strategy: <strategy>
  band: <name> (<used_percent>% used, weight <band_weight>)
  warnings: <n>
```
(band line only when usage enabled; warnings line only when n>0; numbers via `strconv.FormatFloat(v, 'f', -1, 64)`.)

No pick → nothing on stdout (F22 renders the failure line); exclusion warnings per SPEC §3.

`explain` text:
```
explain <profile> (<ulid>): picked <candidate_id> (score <final_score>)
  confidence: live
  band: five hour (25% used, weight 0.8)
  route_provenance: provider_live
  excluded: <candidate_id> (<reason_code>)
  last_verified: 2026-08-07T17:03:11Z
```
(lines omitted when the field is absent in degraded mode; `candidate_id` is `-` when the record had no pick.)

## 8. Imported contracts (consumed upstream)

### 8.1 F22 `pkg/whichmodel` (pinned; canonical owner: `specs/features/F22-cli-skeleton/CONTRACTS.md`)

`GlobalFlags` (all fields incl. `Normalizer`, `Aggregator`, `NoUsage`, `JSON`, `ConfigPath`), `Global`, `UsageError`, `CodedError`, `ReportedError`, `ExitCodeFor`, `CodeFor`, `RegisterExitCode` (F26 registers `no_pick`→3, `usage_gated`→4, `auth_required`→5 in its own init()), `ExecuteArgs` (single render point: failure line via F03 `output.WriteFailure` on stderr; JSON error document on stdout in `--json` mode — never for `ReportedError`), unexported `register`/`registeredCommands`, `RegisterSchema` (F26 registers `"pick"` and `"explain"` docs). F26 never calls `AddCommand` or `os.Exit`.

### 8.2 F21 `internal/usage/toggle` (canonical owner: `specs/features/F21-usage-toggle/CONTRACTS.md`; pinned)

```go
package toggle

// ResolveUsageEnabled returns (enabled, reason).
// reason ∈ "flag" | "config" | "compiled_out" | "no_providers_enabled".
func ResolveUsageEnabled(flagNoUsage bool, cfg *config.Config) (bool, string)

// Compiled is true in normal builds, false under -tags nousage.
const Compiled bool
```

- F26 uses `ResolveUsageEnabled(Global.NoUsage, cfg)` in `RunPick`.
- Strict rule: `reason == "no_providers_enabled"` AND the raw config `usage.enabled` value parses to `"true"` (config.UsageTrue) → exit 2 `usage_config` with the SPEC §3 message (SPEC §2.14).
- `ErrUsageCompiledOut` (F21, `internal/usage/errors.go`): when `toggle.Compiled == false` the toggle resolves disabled with reason `compiled_out`; F26 runs degraded (§2.15).
- F26 compiles under `-tags nousage`: `toggle.ResolveUsageEnabled` is the stub, so `RunPick`/`RunExplain` must not reference real usage types outside the toggle call in that build.

### 8.3 F14 `internal/usage/fetch` (canonical owner: `specs/features/F14-usage-fetch/CONTRACTS.md`)

```go
func FetchAll(ctx context.Context, opts FetchAllOptions) (*FetchResult, error)
// FetchAllOptions{Providers []string; All bool; Source usage.Source;
//   ForceRefresh, MaxAge, Timeout, Offline, IncludeIdentity ...}
// FetchResult{Snapshots []usage.Snapshot; LastVerified map[string]time.Time}
```

F26 calls it with the survivor providers (usage stage, SPEC §2.2e); F26's seam `fetchAllFunc` (defined in `pick.go`, default `fetch.FetchAll`) is injectable in tests. `LastVerified[provider]` feeds Evidence `last_verified` (single timestamp of the picked provider) and `confidence: "live"`; absence → `"cached"`.

### 8.4 F19 `internal/usage/band` (canonical owner: `specs/features/F19-usage-bands/CONTRACTS.md`)

```go
package band

type Result struct {
    Name        string
    UsedPercent float64
    Weight      float64
    Gated       bool
}

func Evaluate(snapshot *usage.Snapshot, route string, cfg *config.Config) (Result, error)
```

F26 calls `Evaluate` per survivor (SPEC §2.2f); `Gated` → excluded with `reason_code: "band_gated"`, `reason: "band usage <UsedPercent>% > gate"`; surviving → `band`/`band_weight` fields.

### 8.5 F20 `internal/usage/strategy` (canonical owner: `specs/features/F20-usage-strategies/CONTRACTS.md`)

```go
package strategy

type Options struct {
    Seed *uint64 // consumed by weighted_random
}

// Apply returns the ordered surviving candidates after the strategy pass
// (single pick = first element), or an error for unavailable strategies.
func Apply(name string, candidates []pick.Candidate, opts Options) ([]pick.Candidate, error)

// Names returns the registered strategy names (F26 validates --strategy
// against this list at runtime).
func Names() []string
```

F26 resolves the strategy name against `strategy.Names()` (unknown → exit 2). `least_used` + usage disabled is refused by F26 BEFORE calling Apply (SPEC §2.15; master plan §6.4) — F26 does not rely on Apply to refuse.

### 8.6 F18 `internal/usage/routing` (canonical owner: `specs/features/F18-usage-routing/CONTRACTS.md`)

```go
package routing

type Route struct {
    Provider, ModelID, Model, Reasoning string
    Windows                              []string
    Provenance                           string // provider_live|models_dev|user_declared
}

func LoadRoutes(path string) ([]Route, error)
func SaveRoutes(path string, routes []Route) error
func ProduceRoutes(cfg *config.Config) ([]Route, error)
func RoutesPath(cfg *config.Config) (string, error)
```

F26 joins `RoutesPath(cfg)` routes for the survivor models (SPEC §2.2d); each becomes a `RouteRef` (`WindowIDs = Windows`; `Reasoning` passes through, default `"default"`); `Provenance` feeds Evidence `route_provenance`.

### 8.7 F10 `internal/scoring` (canonical owner: F10 CONTRACTS; F26's seam)

```go
package scoring

// Score returns the rounded model score for (profile, model, reasoning) and
// whether a score row exists, plus the row's tier1+category composite inputs.
func Score(profile, model, reasoning string) (decimal.Decimal, bool, map[string]float64)
```

F26's seam `scoreFunc` (default `scoring.Score`) is injectable in tests; the inputs map feeds Evidence `score_inputs` verbatim (keys per F10 CONTRACTS). Missing row → `ok=false` (exclusion `no_score_row`). Ranking is F26's (descending by score, ties by provider order then model_id lexical).

### 8.8 F01 `internal/config`

- `func Load(path string) (*config.Config, error)`, `func (c *Config) UnmarshalKey(key string, out any) error`, `func (c *Config) StateDir() (string, error)` — state dir for `pick/history.jsonl`; `UnmarshalKey("usage.enabled", &v)` feeds the strict-rule check (§8.2).

## 9. Security invariants (this feature)

- No credential material in any output; usage failure messages come from F14 sanitized (global SPEC §6.5); canary test covers the canonical `Failure` redaction boundary.
- `explain` reveals only the recorded evidence — which never contains credentials (history record fields are fixed, §2).
- History file is append-only; F26 never rewrites it (write failure → stderr warning, exit unaffected, SPEC D-12).
