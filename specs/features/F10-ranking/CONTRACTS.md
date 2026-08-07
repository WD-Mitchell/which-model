---
kind: feature-contracts
feature: F10
version: "1.0"
project: which-model
---

# F10 — Ranking: Contracts (`internal/pick`)

Package `internal/pick` (directory `internal/pick/`). No CLI. Importable by `internal/routing` and `pkg/whichmodel` per `specs/global/CONTRACTS.md §8`. Uses only `internal/decimal` (F02) and `internal/catalog` types (global `ScoreRow`), plus stdlib.

## 1. Types

### 1.1 Axes and columns — `internal/pick/axes.go`

```go
package pick

type Tier1Axis string

const (
    AxisIntelligence Tier1Axis = "intelligence"
    AxisCost         Tier1Axis = "cost"
    AxisSpeed        Tier1Axis = "speed"
)

// Tier1ScoreColumn maps the 3 fixed ranking axes to their scores-CSV columns.
var Tier1ScoreColumn = map[Tier1Axis]string{
    AxisIntelligence: "intelligence_index_score",
    AxisCost:         "cost_per_intelligence_index_task_usd_score",
    AxisSpeed:        "median_end_to_end_response_time_seconds_score",
}

// Tier1AxisOrder is the fixed iteration order for missing_tier1 reasons.
var Tier1AxisOrder = []Tier1Axis{AxisIntelligence, AxisCost, AxisSpeed}

// CategoryNames is the canonical 12-category order (annex-b §5.1); also the
// deterministic iteration order for profile.Tier2Weights (F10 SPEC D2).
var CategoryNames = []string{
    "reasoning", "knowledge", "research", "planning_capability",
    "instruction_following", "software_engineering", "ui_visual",
    "agentic_tools", "finance", "evidence_capture", "security", "data_ml",
}
```

### 1.2 Identity and availability — `internal/pick/availability.go`

```go
// Identity is an exact (model, reasoning) tuple as used by the availability
// filter. Matching is exact membership; no cleaning or fuzzy matching.
type Identity struct {
    Model     string
    Reasoning string
}
```

### 1.3 Ranking errors — `internal/pick/errors.go`

```go
// RankingError: profile/availability validation or parse failure.
// F22 maps to exit 2 (arguments/config error, global SPEC §5).
type RankingError struct{ Message string }

func (e *RankingError) Error() string

// NoCandidatesError: zero candidates survive the tier-1 and availability
// filters. F22 maps to exit 3 (no viable candidate, global SPEC §5).
type NoCandidatesError struct{ Message string }

func (e *NoCandidatesError) Error() string
```

Verbatim messages (rank_models.py:80-103, 359-483):

| Condition | Message |
|---|---|
| tier1_share <= 0 or tier2_share < 0 | `tier 1 share must be positive and tier 2 share cannot be negative` |
| shares sum != 100 | `tier 1 and tier 2 shares must sum to 100` |
| tier1 keys != {intelligence, cost, speed} | `tier 1 weights must include intelligence, cost, and speed (missing <sorted>; unknown <sorted>)` — details joined with `; `, each detail `missing <names>` / `unknown <names>` |
| tier1 weight out of (0,5] | `tier 1 weight <name> must be greater than 0 and at most 5` |
| tier2 key not in CategoryNames | `unknown tier 2 categories: <sorted names>` |
| tier2 weight out of (0,5] | `tier 2 weight <name> must be greater than 0 and at most 5` |
| zero candidates, filter supplied | `no candidates remain after live model-and-effort availability and Tier 1 filtering` |
| zero candidates, no filter | `no candidates contain all mandatory Tier 1 scores` |
| profile JSON unparseable | `weights JSON is invalid: <err>` |
| profile JSON not an object | `weights JSON must be an object` |
| tier1/tier2 weights not objects | `weights JSON tier1/tier2 weights must be objects` |
| weight map key blank | `tier 1 weight names must be non-blank strings` / `tier 2 weight names must be non-blank strings` |
| weight value non-numeric | `tier 1 weight <name> must be numeric` / `tier 2 weight <name> must be numeric` |
| weight value non-finite | `tier 1 weight <name> must be finite` / `tier 2 weight <name> must be finite` |
| weight value outside [0,5] | `tier 1 weight <name> must be between 0 and 5` / `tier 2 weight <name> must be between 0 and 5` |
| share non-numeric | `tier1 share must be numeric` / `tier2 share must be numeric` |
| availability identity malformed | `availability identity <q> must use model\|reasoning, model::reasoning, model,reasoning, or model/reasoning` |
| availability JSON not a list | `availability JSON must be a list` |
| availability JSON entry invalid | `invalid availability entry: <q>` |
| availability list empty | `availability list contains no identities` |
| filter supplied but empty | `availability filter was supplied but contains no identities` |

## 2. Functions

### 2.1 Profile validation and parsing — `internal/pick/profile.go`

```go
// ValidateProfile enforces the 6 rules of rank_models.py:80-103 verbatim
// (annex-b §5.2). Returns *RankingError on the first violation.
func ValidateProfile(p Profile) error

// ProfileFromJSON parses a custom profile from flat or nested JSON
// (rank_models.py profile_from_json): flat keys tier1_share/tier2_share/
// tier1_weights/tier2_weights, or nested tier1:{share,weights|axis...}/
// tier2:{share,weights|category...}; share defaults 100/0. Validates via
// ValidateProfile before returning. Profile.Name is "custom".
func ProfileFromJSON(data []byte) (Profile, error)
```

`Profile` is the global type from `specs/global/CONTRACTS.md §4.3` (Go file `internal/catalog/types.go`):

```go
package catalog

type Profile struct {
    Name         string
    Tier1Share   decimal.Decimal
    Tier2Share   decimal.Decimal
    Tier1Weights map[string]decimal.Decimal // keys "intelligence"|"cost"|"speed"
    Tier2Weights map[string]decimal.Decimal // keys are CategoryNames members
}
```

### 2.2 Built-in profiles — `internal/pick/profiles.go`

```go
// Profiles holds the 11 built-ins, constructed via mustProfile which panics
// on ValidateProfile failure (annex-b §5.1, mirrors Python import-time crash).
var Profiles map[string]Profile
```

Keys and literal weights (annex-b §5.1, rank_models.py:124-171):

| Key | Tier1Share/Tier2Share | Tier1Weights | Tier2Weights |
|---|---|---|---|
| simple_implementation | 80/20 | int 1, cost 5, speed 5 | instruction_following 5 |
| simple_action_execution | 65/35 | int 1, cost 5, speed 5 | instruction_following 5, evidence_capture 5, agentic_tools 3, software_engineering 2 |
| balanced_implementation | 70/30 | int 3, cost 3, speed 3 | software_engineering 5, instruction_following 3, agentic_tools 2 |
| complex_implementation | 60/40 | int 5, cost 1, speed 1 | software_engineering 5, planning_capability 4, instruction_following 2 |
| ui_ux | 60/40 | int 3, cost 2, speed 3 | ui_visual 5, software_engineering 4, instruction_following 3, evidence_capture 2 |
| complex_action_execution | 60/40 | int 4, cost 2, speed 2 | agentic_tools 5, instruction_following 4, evidence_capture 2 |
| financial_work | 60/40 | int 5, cost 1, speed 2 | finance 5, knowledge 4, reasoning 4, research 3, instruction_following 3 |
| research | 60/40 | int 4, cost 2, speed 2 | research 5, knowledge 4, reasoning 3, instruction_following 2, agentic_tools 2 |
| planning | 60/40 | int 5, cost 1, speed 1 | planning_capability 5 |
| orchestration | 60/40 | int 5, cost 5, speed 4 | planning_capability 5, instruction_following 5 |
| review | 65/35 | int 4, cost 3, speed 2 | instruction_following 5, software_engineering 4, reasoning 4, security 3, evidence_capture 2 |

### 2.3 Scoring and ranking — `internal/pick/rank.go`

```go
// ScoreModel computes tier1/tier2/total for ONE row (rank_models.py:371-427,
// annex-b §5.3 verbatim). Missing tier-1 axis scores produce
// ExcludedReasons=["missing_tier1:<joined missing axes in Tier1AxisOrder>"]
// with all score fields zeroed; otherwise ExcludedReasons is empty.
func ScoreModel(row catalog.ScoreRow, profile Profile) ModelScore

// Rank ranks all rows: tier-1 exclusion, scoring, availability filter
// (applied last), 7-key tie-break sort, excluded-row sort (SPEC D7).
// available == nil means no filter. Returns *NoCandidatesError when zero
// candidates remain (two distinct messages, SPEC §2.11). Precondition:
// rows have unique (model, reasoning) identities (enforced by
// score.ParseScoresCSV, F09).
func Rank(rows []catalog.ScoreRow, profile Profile, available []Identity) (Result, error)
```

`catalog.ScoreRow` is the global type from `specs/global/CONTRACTS.md §2.1` (Go file `internal/catalog/types.go`):

```go
package catalog

type ScoreRow struct {
    Model       string
    Reasoning   string
    Tier1       map[string]decimal.Decimal // keys are the 6 metric _score column names
    Categories  map[string]decimal.Decimal // keys are the 12 category names (no _score suffix)
    Benchmarks  map[string]decimal.Decimal // keys are benchmark names (no benchmark: prefix)
}
```

Tier-1 axis scores are read as `row.Tier1[Tier1ScoreColumn[axis]]` (absent key = missing). Category scores are read as `row.Categories[name]` (absent key = blank). Both weighted means use `decimal.WeightedMean` from `internal/decimal` (F02 pin, `specs/global/CONTRACTS.md §2.3`).

### 2.4 Results — `internal/pick/rank.go` (JSON per annex-b §5.8)

```go
type ModelScore struct {
    Model             string                     `json:"model"`
    Reasoning         string                     `json:"reasoning"`
    Total             decimal.Decimal            `json:"total_score"`
    Tier1             decimal.Decimal            `json:"tier1_score"`
    Tier2             *decimal.Decimal           `json:"tier2_score"`       // null when no tier-2 evidence
    Tier1Contribution decimal.Decimal            `json:"tier1_contribution"`
    Tier2Contribution decimal.Decimal            `json:"tier2_contribution"`
    Categories        map[string]decimal.Decimal `json:"category_scores"`   // only populated categories
    Warnings          []string                   `json:"warnings"`
    ExcludedReasons   []string                   `json:"-"`                 // never serialized
}

type ExcludedRow struct {
    Model     string   `json:"model"`
    Reasoning string   `json:"reasoning"`
    Reasons   []string `json:"reasons"`
}

type Result struct {
    Profile                  string       `json:"profile"`
    Recommendation           ModelScore   `json:"recommendation"`
    Alternatives             []ModelScore `json:"alternatives"`
    Excluded                 []ExcludedRow `json:"excluded"`
    CandidateCount           int          `json:"candidate_count"`
    AvailabilityFilterApplied bool        `json:"availability_filter_applied"`
}
```

Decimal fields serialize via `decimal.Decimal.MarshalJSON` (text-based, precision-preserving; `_tie_*` sort keys are internal to Rank and never serialized).

### 2.5 Availability parsing — `internal/pick/availability.go`

```go
// ParseAvailability parses a JSON array or plain-text availability list
// (rank_models.py _availability_values + _identity + parse_availability,
// annex-b §5.7). JSON elements: plain string (separator rule), object
// {"model","reasoning"}, or [model, reasoning] pair. Plain text: one identity
// per non-blank non-# line, optional case/space-insensitive header line
// "model,reasoning"|"model|reasoning" skipped. Separator priority
// "|", "::", ",", "/" with last-occurrence split. Empty input returns
// (nil, nil) — the caller treats nil as "no filter supplied".
func ParseAvailability(data []byte) ([]Identity, error)
```

## 3. Config keys, flags, error codes, JSON shapes

- **Config keys owned:** none (profile and availability inputs arrive as function arguments; F22 owns CLI flags and any config mapping).
- **Flags owned:** none.
- **Error codes added:** none — errors are Go types (`RankingError`, `NoCandidatesError`); F22 maps them to exit codes 2 and 3 per `specs/global/SPEC.md §5`.
- **JSON shapes emitted:** the `Result` struct above (annex-b §5.8) — used by F22 as the payload of `which-model pick --json` and `which-model explain --json`.
- **Warning strings emitted** (verbatim, annex-b §5.6): `missing optional category scores: <names in CategoryNames order>` and `no optional task-category scores available; Tier 1 score used` (only when `Tier2Weights` is non-empty).
