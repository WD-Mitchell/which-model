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

// PublishConfig mirrors [catalog.publish] (annex-b §8.1) plus the two
// [catalog] artifact paths needed by the generated workflow's git add step.
type PublishConfig struct {
    Enabled        bool
    Schedule       string
    Timezone       string
    Branches       []string
    Mode           string // "pull-request" | "direct-push"
    AutoMerge      bool
    MergeMethod    string // "squash" | "merge" | "rebase"
    CommitMessage  string
    PRTitle        string
    PRLabels       []string
    RunTests       bool
    RawCSVPath     string // from [catalog].raw_csv_path; blank -> default
    ScoresCSVPath  string // from [catalog].scores_csv_path; blank -> default
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

// Load reads [catalog.publish] (and [catalog].raw_csv_path /
// [catalog].scores_csv_path), applies defaults for absent keys, and runs
// Validate. Missing section = all defaults. Returns typed errors (→ exit 2).
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

`internal/catalog/publish/pins.go` — the pinned action SHAs with their version comments (annex-b §8 "Toolchain setup" pinning convention, verified against the legacy workflow and the `actions/setup-go` v6.3.0 tag):

```go
package publish

const (
    // CheckoutPin is actions/checkout v6.0.2 (same pin as the legacy workflow).
    CheckoutPin = "de0fac2e4500dabe0009e67214ff5f5447ce83dd"
    // SetupGoPin is actions/setup-go v6.3.0 (lightweight tag commit).
    SetupGoPin = "4b73464bb391d4059bd26b0524d20df3927bd417"
)
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
| `catalog.publish.branches` | array[string] | `["main"]` | listed order; explicit `[]` = error |
| `catalog.publish.mode` | string | `"pull-request"` | `"pull-request"`\|`"direct-push"` |
| `catalog.publish.auto_merge` | bool | `true` | pull-request mode only |
| `catalog.publish.merge_method` | string | `"squash"` | `"squash"`\|`"merge"`\|`"rebase"` |
| `catalog.publish.commit_message` | string | `"chore(data): refresh available model scores"` | commit `-m` |
| `catalog.publish.pr_title` | string | `"chore(data): refresh available model scores"` | `gh pr create --title` |
| `catalog.publish.pr_labels` | array[string] | `["data", "automated"]` | one `--label` each |
| `catalog.publish.run_tests` | bool | `true` | test gate step presence |
| `catalog.raw_csv_path` | string | `"available_model_raw_values.csv"` | `git add` path 1 (read-only) |
| `catalog.scores_csv_path` | string | `"available_model_scores.csv"` | `git add` path 2 (read-only) |

## 4. Generated workflow shape (golden)

Full golden documents live in `specs/features/F30-publishing/TASKS.md` task F30-T4 (`internal/catalog/publish/testdata/refresh-model-data.golden.yml`). Invariants every generated file satisfies (checked by tests):

1. First line: `# GENERATED by \`which-model catalog workflow --write\` from [catalog.publish] — do not hand-edit.`
2. `on.schedule` carries the literal cron + `# <timezone>, per [catalog.publish].schedule`; `workflow_dispatch: {}` present.
3. `permissions:` block per SPEC Decisions (mode-dependent).
4. `concurrency.group: refresh-model-data`; `cancel-in-progress: false`.
5. `jobs.refresh.strategy.matrix.branch` = `branches` in listed order; `fail-fast: false`.
6. Steps: checkout (pinned `# v6.0.2`, `ref: ${{ matrix.branch }}`), setup-go (pinned `# v6.3.0`, `go-version-file: go.mod`, `cache: true`), build, `go test ./internal/catalog/... ./internal/pick/... ./internal/routing/...` (only when `run_tests`), `./which-model catalog refresh` with `ARTIFICIAL_ANALYSIS_API: ${{ secrets.ARTIFICIAL_ANALYSIS_API }}`, changes, commit (bot identity), mode steps, outcome report.
7. `gh pr create --base "${{ matrix.branch }}" --title "<pr_title>"` + one `--label <l>` per `pr_labels`; `gh pr merge --auto --<merge_method>` when `auto_merge`; both with `GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}`; `git push origin HEAD:${{ matrix.branch }}` for direct-push.
8. No `secrets.` reference other than `ARTIFICIAL_ANALYSIS_API` and `GITHUB_TOKEN`; no usage command anywhere.
9. Exactly one trailing `\n`; LF line endings.

## 5. Cross-feature references (pinned)

- F01 `internal/config`: `func (c *Config) UnmarshalKey(key string, out any) error` (DECISION B) — the `UnmarshalKeyer` interface is satisfied structurally; tests use a fake.
- F23 `pkg/whichmodel/catalog_cmd.go` — `workflow` subcommand added inside `NewCatalogCmd` after F23 lands.
- `internal/security` (F05) usage: none required (no network, no credentials, no file reads beyond the workflow file).
- Compiles under `-tags nousage`: `internal/catalog/publish` never imports `internal/usage` (annex-b §0 usage independence).

## 6. Error codes added

None — uses the fixed `0`/`1`/`2` set (`specs/global/SPEC.md §5`); `--check` drift is exit `1` (annex-d §2.3a), config validation errors are exit `2` (global SPEC §5: bad config). No new `Failure.Code` values (`specs/global/CONTRACTS.md §1.6`).

## 7. Flags owned

`--write`, `--check`, `--out <path>` on `catalog workflow` (all others belong to F23's other subcommands).
