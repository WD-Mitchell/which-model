---
kind: feature-contracts
feature: F30-publishing
version: "1.0"
project: which-model
module: github.com/WD-Mitchell/which-model
---

# F30 — Catalog Publishing: Contracts

Source: `docs/plan/annex-b-catalog-port.md` §8.1–§8.7, `docs/plan/annex-d-cli-reference.md` §2.3/§2.3a/§4.2/§5, `specs/features/F30-publishing/SPEC.md`.

## 1. `internal/catalog/publish` (new package)

`internal/catalog/publish/config.go`:

```go
package publish

// PublishConfig mirrors [catalog.publish] plus the raw CSV path staged by the
// generated workflow.
type PublishConfig struct {
    Enabled        bool
    Schedule       string
    Timezone       string
    Environment    string // optional GitHub Actions environment name
    Branches       []string
    Mode           string // "pull-request" | "direct-push"
    AutoMerge      bool
    MergeMethod    string // "squash" | "merge" | "rebase"
    CommitMessage  string
    PRTitle        string
    PRLabels       []string
    RawCSVPath     string // from [catalog].raw_csv_path; blank -> default
    ScoresCSVPath  string // from [catalog].scores_csv_path; blank -> publication default
}

// Defaults (annex-b §8.1; SPEC behaviour 2).
const (
    DefaultSchedule      = "0 6 * * *"
    DefaultTimezone      = "Europe/London"
    DefaultMode          = "pull-request"
    DefaultMergeMethod   = "squash"
    DefaultCommitMessage = "chore(data): refresh available model scores"
    DefaultPRTitle       = "chore(data): refresh available model scores"
)

var DefaultBranches = []string{"main"}
var DefaultPRLabels = []string{"data", "automated"}

// UnmarshalKeyer is satisfied by *config.Config (F01 pin:
// func (c *Config) UnmarshalKey(key string, out any) error).
type UnmarshalKeyer interface{ UnmarshalKey(key string, out any) error }

// Load decodes the complete [catalog] table, applies publishing defaults for
// absent keys, and runs Validate. Missing section = all defaults.
func Load(cfg UnmarshalKeyer) (*PublishConfig, error)

// Validate checks mode/merge_method/branches/schedule/labels per SPEC
// behaviour 3. Typed errors name the offending key.
func Validate(pc *PublishConfig) error

// ValidateCron implements the SPEC behaviour-3 grammar (GitHub Actions
// documented subset). Returns a typed error with the offending field.
func ValidateCron(schedule string) error
```

`internal/catalog/publish/workflow.go`:

```go
package publish

// DefaultWorkflowName is the committed file name (annex-b §8.2).
const DefaultWorkflowName = "refresh-model-data.yml"

// RepoRoot walks upward from cwd to the nearest ancestor containing ".git".
func RepoRoot() (string, error)

// WorkflowPath returns repoRoot/.github/workflows/refresh-model-data.yml.
func WorkflowPath(repoRoot string) string

// Render returns the workflow YAML bytes (deterministic; exactly one trailing
// "\n"), or nil when !pc.Enabled. Pure function of pc.
func Render(pc *PublishConfig) ([]byte, error)

// Write renders pc and writes it to path (0644, MkdirAll parents), or
// removes path when !pc.Enabled. Returns a human summary line (SPEC
// behaviour 9 shapes).
func Write(pc *PublishConfig, path string) (string, error)

// Check renders in memory and byte-compares with path (SPEC behaviour 7).
// Returns nil when in sync (incl. !Enabled and path absent). Returns a
// *DriftError otherwise (missing file, stale file, or byte difference).
func Check(pc *PublishConfig, path string) error

// DriftError carries the stderr lines (headers + minimal line diff).
type DriftError struct {
    Lines []string
}
func (e *DriftError) Error() string
```

`internal/catalog/publish/pins.go` owns the pinned checkout action SHA:

```go
package publish

const CheckoutPin = "de0fac2e4500dabe0009e67214ff5f5447ce83dd" // actions/checkout v6.0.2
```

## 2. CLI changes (`pkg/whichmodel/catalog_cmd.go` — F23-owned file, extended by F30)

The `workflow` subcommand is added inside F23's `NewCatalogCmd` (F30 depends on F23; DECISION A: F30 adds zero lines to F22 files).

```
which-model catalog workflow --write [--out PATH]
which-model catalog workflow --check [--out PATH]
```

- `--write`, `--check`: bools, mutually exclusive (both → error, exit 2).
- `--out PATH`: output path override (default `WorkflowPath(repoRoot)`).
- Exit codes: `0` success incl. `--check` in sync; `1` drift / I/O failure; `2` invocation or config validation errors.
- stdout/stderr shapes per SPEC behaviour 9 and the SPEC error table.

## 3. Config keys owned

| Key | Type | Default | Notes |
|---|---|---|---|
| `catalog.publish.enabled` | bool | `true` | master switch (§8.6) |
| `catalog.publish.schedule` | string | `"0 6 * * *"` | literal cron into `on.schedule` |
| `catalog.publish.timezone` | string | `"Europe/London"` | comment on the cron line |
| `catalog.publish.environment` | string | `""` | optional Actions environment attached to the refresh job |
| `catalog.publish.branches` | array[string] | `["main"]` | listed order; explicit `[]` = error |
| `catalog.publish.mode` | string | `"pull-request"` | `"pull-request"`\|`"direct-push"` |
| `catalog.publish.auto_merge` | bool | `true` | pull-request mode only |
| `catalog.publish.merge_method` | string | `"squash"` | `"squash"`\|`"merge"`\|`"rebase"` |
| `catalog.publish.commit_message` | string | `"chore(data): refresh available model scores"` | commit `-m` |
| `catalog.publish.pr_title` | string | `"chore(data): refresh available model scores"` | `gh pr create --title` |
| `catalog.publish.pr_labels` | array[string] | `["data", "automated"]` | one `--label` each |
| `catalog.raw_csv_path` | string | `"data/available_model_raw_values.csv"` | raw artifact path; staged together with scores |
| `catalog.scores_csv_path` | string | `"data/available_model_scores.csv"` | generated score artifact; staged together with raw values |

## 4. Generated workflow shape (golden)

Full golden documents live in `specs/features/F30-publishing/TASKS.md` task F30-T4 (`internal/catalog/publish/testdata/refresh-model-data.golden.yml`). Invariants every generated file satisfies (checked by tests):

1. First line: `# GENERATED by \`which-model catalog workflow --write\` from [catalog.publish] — do not hand-edit.`
2. `on.schedule` carries the literal cron + `# <timezone>, per [catalog.publish].schedule`; `workflow_dispatch: {}` present.
3. `permissions:` block per SPEC Decisions (mode-dependent).
4. `concurrency.group: refresh-model-data`; `cancel-in-progress: false`.
5. `jobs.refresh.strategy.matrix.branch` = `branches` in listed order; `fail-fast: false`.
6. Optional non-empty `environment` is emitted at `jobs.refresh.environment`. Checkout and PR creation authenticate with `${{ secrets.CSV_UPDATE_TOKEN || github.token }}`; the standalone refresh uses `ARTIFICIAL_ANALYSIS_API`; score generation and the Python suite validate the configured raw/scores pair before staging both artifacts; no Go setup, build, or application invocation.
7. Pull-request mode assigns a unique head branch, creates a PAT-authored PR when `CSV_UPDATE_TOKEN` is configured, conditionally approves it with `github.token`, and enables auto-merge as step `id: merge`. Direct-push mode uses step `id: publish`.
8. Outcome reporting keys off the relevant publish-step outcome: `auto-merge-enabled` for an accepted deferred merge request, `published` only for completed direct pushes, `skipped-no-changes`, or `failed`. No usage command or usage credential appears.
9. Exactly one trailing `\n`; LF line endings.

## 5. Cross-feature references (pinned)
- F01 `internal/config`: `func (c *Config) UnmarshalKey(key string, out any) error` (DECISION B) — the `UnmarshalKeyer` interface is satisfied structurally; tests use real strict TOML decoding (a fake is reserved for sentinel-error propagation).
- F23 `pkg/whichmodel/catalog_cmd.go` — `workflow` subcommand added inside `NewCatalogCmd` after F23 lands.
- `internal/security` (F05) usage: none required (no network, no credentials, no file reads beyond the workflow file).
- Compiles under `-tags nousage`: `internal/catalog/publish` never imports `internal/usage` (annex-b §0 usage independence).

## 6. Error codes added

None — uses the fixed `0`/`1`/`2` set (`specs/global/SPEC.md §5`); `--check` drift is exit `1` (annex-d §2.3a), config validation errors are exit `2` (global SPEC §5: bad config). No new `Failure.Code` values (`specs/global/CONTRACTS.md §1.6`).

## 7. Flags owned

`--write`, `--check`, `--out <path>` on `catalog workflow` (all others belong to F23's other subcommands).

### Shared catalog schema correction (#166)

All catalog consumers decode the complete `catalog` table through F01's strict,
table-only `UnmarshalKey`. `internal/catalog.Config` owns all existing catalog
fields plus `Publish PublishConfig` (`toml:"publish"`); `catalog.PublishConfig`
owns the existing nested publishing fields. `whichmodel.CatalogConfig` and
`publish.PublishConfig` remain public aliases. Unknown catalog and publish keys
are errors; valid nested publishing settings coexist with configured raw/scores
paths, including environment-only overrides. Publishing seeds nested defaults,
then reads raw artifact paths from the decoded root. Pick validates catalog
configuration before loading scores and propagates config errors as exit 2.
Empty consumer paths retain their previous defaults. This corrects scalar
accessor guidance that contradicted F01's table-only contract.

### Paired artifact publication correction (#165)

This supersedes the former raw-only publication decision. Each refresh produces
and publishes a coherent raw/scores pair. The standalone Python refresh receives
`--output <RawCSVPath>`; `generate_scores.py` receives `--input <RawCSVPath>` and
`--output <ScoresCSVPath>`. Before staging either artifact, run
`python3 -m unittest discover -s .daily-update/tests -v` with
`WHICH_MODEL_TEST_RAW_CSV` and `WHICH_MODEL_TEST_SCORES_CSV` set to the configured
artifact pair. Generation or test failure
aborts publication. Both artifact arguments are shell-quoted consistently across
refresh, generation, and staging. Equal normalized destinations are invalid.
The score publication default is `data/available_model_scores.csv`, distinct from
the CLI's cache-path default. PR CI runs the Python suite, preserving deterministic
regeneration checks. The generator retains Python Decimal/schema behavior and
requires no Go runtime. Checkout pins in the renderer, golden, and committed
workflow remain synchronized. Regression cases cover custom paths, stage ordering,
generator failure before staging, and repeat generation matching committed bytes.
