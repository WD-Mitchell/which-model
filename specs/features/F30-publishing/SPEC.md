---
kind: feature-spec
feature: F30-publishing
version: "1.0"
project: which-model
---

# F30 — Catalog Publishing (GitHub Action Generator): Spec

## Purpose

The deterministic generator `which-model catalog workflow --write|--check` renders `.github/workflows/refresh-model-data.yml` from `[catalog.publish]`. The generated Action invokes the standalone `.daily-update/refresh-model-data.py`, which discovers every models.dev provider and the union of every models.dev benchmark plus every supported Artificial Analysis benchmark, without checked-in provider or benchmark configuration, and updates the configured raw CSV and its generated score CSV as one verified pair. Provider aliases that bridge previously separate identities are merged deterministically when their Artificial Analysis family does not conflict; when display name and provider ID match different AA families, display name wins. It runs the Python score generator and Python tests against that same configured pair before staging either artifact; it does not build or invoke the Go application.

## Behaviour

1. **Command surface** (extends F23's `which-model catalog`; DECISION A wiring — the `workflow` subcommand is added inside F23's `pkg/whichmodel/catalog_cmd.go`, which F30 depends on):
   - `which-model catalog workflow --write [--out PATH]`
   - `which-model catalog workflow --check [--out PATH]`
   - `--write` renders `[catalog.publish]` into the workflow file and (over)writes it; `--check` renders in memory and byte-compares against the committed file, exiting non-zero on drift (`docs/plan/annex-b-catalog-port.md` §8.2; `docs/plan/annex-d-cli-reference.md` §2.3/§2.3a).
   - `--write` and `--check` are mutually exclusive → both given is an invocation error, exit `2` (annex-d §2.3a).
   - `--out PATH` overrides the output path; default is `<repoRoot>/.github/workflows/refresh-model-data.yml`, where `repoRoot` = nearest ancestor of cwd containing `.git/` (self-contained upward walk — no F28 dependency; `docs/plan/annex-d-cli-reference.md` §2.3 `--out` row).

2. **`[catalog.publish]` config ownership** (annex-b §8.1 verbatim; DECISION B): F30 owns `internal/catalog/publish/config.go` — `PublishConfig` (alias of `catalog.PublishConfig`) + `Load`, decoding the shared root with `cfg.UnmarshalKey("catalog", &cc)` after seeding `cc.Publish` defaults. Missing section → all defaults apply; an explicitly present `branches = []` is a validation error. Defaults:

   | key | default |
   |---|---|
   | `enabled` | `true` |
   | `schedule` | `"0 6 * * *"` |
   | `timezone` | `"Europe/London"` |
   | `environment` | `""` (no GitHub Actions environment) |
   | `branches` | `["main"]` |
   | `mode` | `"pull-request"` |
   | `auto_merge` | `true` |
   | `merge_method` | `"squash"` |
   | `commit_message` | `"chore(data): refresh available model scores"` |
   | `pr_title` | `"chore(data): refresh available model scores"` |
   | `pr_labels` | `["data", "automated"]` |

   F30 also reads `[catalog].raw_csv_path` (blank → `data/available_model_raw_values.csv`) and `[catalog].scores_csv_path` (blank → `data/available_model_scores.csv`) as the paired `git add` paths in the generated workflow.

3. **Validation** (all errors are typed, mapped to exit `2`):
   - `schedule`: exactly 5 whitespace-separated fields (minute, hour, day-of-month, month, day-of-week). Per-field tokens: `*`, single number, `A-B` range, `*/N` step, `A-B/N`, or comma-lists of those. Bounds: minute 0–59, hour 0–23, day-of-month 1–31, month 1–12, day-of-week 0–6; month/day-of-week also accept the 3-letter English names (case-insensitive) as single tokens or list elements (`JAN`..`DEC`, `SUN`..`SAT`), never inside ranges/steps. Reject: 6-field (seconds) crons, `@`-keywords (`@daily`, `@hourly`, …), empty fields, out-of-bounds numbers, names in ranges/steps. Decision recorded in `## Decisions` (grammar is the GitHub Actions documented subset).
   - `mode`: exactly `"pull-request"` or `"direct-push"`.
   - `merge_method`: exactly `"squash"`, `"merge"`, or `"rebase"` (validated always; used only in `pull-request` mode).
   - `environment`: optional string; when non-empty, rendered as the refresh job's GitHub Actions environment so environment-scoped secrets are available only to that job.
   - `auto_merge`: bool (used only in `pull-request` mode).
   - `branches`: non-empty after defaults; explicit `[]` → error `"catalog.publish.branches must not be empty"`.
   - `enabled`: bool.
   - `pr_labels`: array of non-empty strings (deduplicated, order preserved).

4. **Rendering** (`internal/catalog/publish/workflow.go`, `Render(pc *PublishConfig) ([]byte, error)`): deterministic template — same config in, byte-identical YAML out; exactly one trailing `\n`; no other trailing whitespace; the full template is in `specs/features/F30-publishing/TASKS.md` task F30-T4 (golden files) and implements annex-b §8.7 with the Decisions below. `Render` returns `nil` when `!pc.Enabled` (no file content exists to render; `Write`/`Check` handle the enabled=false lifecycle, behaviour 6). Template elements:
   - Header comment `# GENERATED by \`which-model catalog workflow --write\` from [catalog.publish] — do not hand-edit.`
   - `name: refresh-model-data`; `on.schedule` with literal cron + `# <timezone>, per [catalog.publish].schedule` comment; `workflow_dispatch: {}` unconditionally (annex-b §8.1 "workflow_dispatch kept unconditionally").
   - `permissions:` — mode-dependent least privilege (Decision): `pull-request` → `contents: write` + `pull-requests: write` (needed for `gh pr create`); `direct-push` → `contents: write` only.
   - `concurrency:` — group `refresh-model-data` constant (Decision: the §8.7 excerpt's `refresh-model-data-main` hardcodes the default branch; the constant name is branch-agnostic), `cancel-in-progress: false`.
   - One job `refresh`: `runs-on: ubuntu-latest`, `timeout-minutes: 30`; optional `environment: "<environment>"` when configured; `strategy.fail-fast: false` with `matrix.branch: [<branches in listed order>]` and comment `# from [catalog.publish].branches, listed order` (annex-b §8.3).
   - Steps in order: pinned `actions/checkout`, using optional `CSV_UPDATE_TOKEN` with `github.token` fallback; `python3 .daily-update/refresh-model-data.py --output <quoted raw>` with `env: ARTIFICIAL_ANALYSIS_API: ${{ secrets.ARTIFICIAL_ANALYSIS_API }}`; Python score generation with the configured raw/scores paths followed by the Python test suite; the `changes` step (`id: changes`, `git add -- <quoted raw> <quoted scores>` + the unchanged diff check); commit, publish, and outcome steps. The workflow contains no Go setup, build, test, `which-model` invocation, provider config, or benchmark config; score generation and Python tests precede staging both CSVs.

5. **Publish modes** (annex-b §8.4; `mode` selects per invocation, not per branch):
   - `pull-request`: require `CSV_UPDATE_TOKEN` for publication, create a unique head branch, and create a Task issue and PR assigned to the authenticated human uploader. The PR follows the repository template including Mermaid, verification and a closing issue reference; read back both assignments and the Development link. When `auto_merge` is true, wait up to ten minutes for CI's `test` and the repository's `CodeQL` checks to register, watch all PR checks, require every resulting bucket to pass, recheck the PR head, then merge using `CSV_UPDATE_TOKEN` with `--match-head-commit` and the configured method. Missing, failed, cancelled, skipped or unavailable checks, a changed head, an uncompleted merge, or normal merge-control rejection fail closed. No bot approval or `--admin` bypass is requested.
   - `direct-push`: `git push origin HEAD:${{ matrix.branch }}` as step `id: publish` (no PR or auto-merge steps).
   - Every publish step is gated on `if: steps.changes.outputs.changed == 'true'` (commit-only-if-changed, annex-b §8 "Staged-commit-only-if-changed" row).
   - Per-branch isolation (annex-b §8.3): `fail-fast: false`; a failure on one branch never aborts the others. The final `if: always()` report emits `skipped-no-changes`; `merged` only after the PR is confirmed merged; `pr-created` when auto-merge is disabled and creation succeeded; `published` only when the direct-push step succeeded; otherwise `failed`.

6. **`enabled = false` lifecycle** (annex-b §8.6): `--write` emits no workflow file and REMOVES `.github/workflows/refresh-model-data.yml` if it exists (from a prior `--write`); `--check` passes (exit 0) iff the file is absent, and reports drift (exit 1) if a stale generated file is still present.

7. **`--check` drift semantics** (annex-b §8.2, annex-d §2.3a): render in memory (the SAME `Render` used by `--write` — never a divergent code path), read the committed file as-is, byte-compare. Any difference — CRLF vs LF, changed indentation, extra/removed blank lines, reordered keys — is drift. On drift: exit `1`, stderr carries the diff with headers `--- <path> (committed)` / `+++ <path> (generated from [catalog.publish])` followed by a minimal line diff (first-difference hunk with 3 lines of context, `-`/`+` pairs, per Decision). Missing committed file = drift: exit `1`, stderr names the file and the fix (`run which-model catalog workflow --write`).

8. **No app or usage in CI**: the generated workflow contains no Go toolchain, build, test, `which-model`, usage command, usage credential, provider config, or benchmark config. The only data-source secret is `ARTIFICIAL_ANALYSIS_API` on the standalone refresh step.

9. **Stdout shapes** (annex-d §2.3a):
   - `--write` success: `wrote .github/workflows/refresh-model-data.yml (schedule="0 6 * * *", branches=[main], mode=pull-request)` — schedule quoted, branches bracketed, mode named; path replaced by `--out` value when given.
   - `--write` with `enabled = false`: `removed .github/workflows/refresh-model-data.yml (catalog.publish.enabled = false)` (or `no workflow file present (catalog.publish.enabled = false)` when absent).
   - `--check` in sync: no stdout, exit 0.
   - `--check` drift: no stdout, diff on stderr, exit 1.

10. **Exit codes** (global `specs/global/SPEC.md` §5; annex-d §2.3a): `0` success (incl. `--check` in sync); `1` drift, or I/O failure writing/removing/reading the workflow file; `2` invocation errors (`--write`+`--check` together, unknown flag) and config validation errors (bad cron, bad mode, bad `merge_method`, empty branches). No new `Failure.Code` values.

11. **Migration** (annex-d §5 migration-table row; `docs/plan/README.md` M6): the legacy hand-maintained workflow `available-model-data-export/.github/workflows/update-available-model-data.yml` is deleted (git rm) in the same change that commits the first generated workflow; `which-model catalog workflow --check` exits 0 afterwards. The legacy Python scripts themselves are F02/F05/F23-owned scope; F30 deletes only the workflow file.

12. **Determinism contract**: `Render` is a pure function of `PublishConfig` (plus the two `[catalog]` artifact paths) — no timestamps, no map-order iteration for labels or branches (slices only), no environment dependence; `--write` twice without a config change is a byte-identical no-op diff (annex-b §8.2). `Write` writes `0644` with parent dirs created; when `!Enabled` and the file exists, removal is exact (the path from `--out` or the default).

13. **Compiles under `-tags nousage`**: `internal/catalog/publish` imports only stdlib + `internal/config` (F01) + shopspring/decimal-free code; the catalog layer is usage-independent by design (annex-b §0).

## Error behaviour

| Condition | Exit | stdout | stderr |
|---|---|---|---|
| `workflow --write` success | 0 | `wrote <path> (schedule=…, branches=[…], mode=…)` | — |
| `workflow --write` with `enabled=false` | 0 | `removed <path> (catalog.publish.enabled = false)` / `no workflow file present (…)` | — |
| `workflow --check` in sync | 0 | — | — |
| `workflow --check` drift (file present) | 1 | — | `--- <path> (committed)` / `+++ <path> (generated from [catalog.publish])` + minimal line diff |
| `workflow --check` drift (file missing) | 1 | — | `<path> is missing; run which-model catalog workflow --write` |
| `workflow --check` with stale file while `enabled=false` | 1 | — | drift diff (generated side is empty) |
| `--write --check` together | 2 | — | `catalog workflow: --write and --check are mutually exclusive` |
| bad cron / bad mode / bad `merge_method` / empty `branches` in config | 2 | — | typed validation error naming the key |
| I/O failure (write/remove/read) | 1 | — | error detail |

## Decisions

| Decision | Value | Rationale |
|---|---|---|
| Generated file name | `.github/workflows/refresh-model-data.yml` | annex-b §8.2/§8.6 and annex-d §2.3a name it verbatim; `--out` overrides |
| Secret name | `ARTIFICIAL_ANALYSIS_API` | annex-b §8 "Secret" row: same name as the legacy workflow, unchanged |
| Concurrency group | `refresh-model-data` (constant) | The §8.7 excerpt's `refresh-model-data-main` hardcodes the default branch; the constant is branch-agnostic for multi-branch configs |
| `permissions` block | Mode-dependent least privilege: `pull-request` → `contents: write` + `pull-requests: write`; `direct-push` → `contents: write` only | `gh pr create`/`gh pr merge` need pull-request scope; direct-push needs only contents |
| Byte-compare normalization | None — exact byte compare of `Render` output vs file read as-is; renderer emits exactly one trailing `\n` | Annex-b §8.2 "byte-identical"; CRLF/whitespace/indent edits ARE drift |
| Drift diff format | Headers + minimal line diff (first-difference hunk, 3 lines context, `-`/`+` pairs) | Matches the annex-d §2.3a example shape; implementable deterministically in stdlib |
| Missing file under `--check` | Drift, exit 1, stderr names file + `--write` fix | A missing committed workflow is exactly the drift `--check` exists to catch |
| Cron grammar | 5 fields; `*`, numbers, `A-B`, `*/N`, `A-B/N`, comma-lists; bounds per GitHub (min 0–59, hour 0–23, dom 1–31, month 1–12, dow 0–6); 3-letter month/day names as single tokens or list elements only | GitHub Actions' documented subset; reject 6-field crons and `@`-keywords |
| `auto_merge` / `merge_method` in `direct-push` mode | Validated for type/enum, then ignored (no error) | annex-b §8.1 says auto_merge is "pull-request mode only"; ignoring keeps configs toggle-friendly |
| `mode` selection | Per invocation, never per branch | annex-b §8.4 verbatim |
| Empty `branches = []` | Validation error, exit 2 | An explicit empty list is ambiguous; default only applies when the key is absent |
| Artifact paths in `git add` | From `[catalog].raw_csv_path` and `.scores_csv_path`, defaulting to `data/available_model_raw_values.csv` and `data/available_model_scores.csv` | The master refresh publishes raw source values and their deterministically generated scores together |
| Repo-root resolution | `--out` wins; else nearest `.git` ancestor of cwd | Annex-d §2.3 `--out` default; self-contained |
| Check-gated merge | Emitted as `id: merge` when `auto_merge`; expected-head guard and configured method | Publishing token performs a normal merge after every check passes; no synthetic approval or rule changes |
| Outcome vocabulary | PR mode: `merged` or `pr-created`; direct-push: `published`; both: `skipped-no-changes` / `failed` | Never claim a deferred PR was already published; key the report to the actual mode step outcome |
| Migration scope | Delete only the legacy workflow file; legacy Python scripts are other features' scope | annex-d §5 migration row; M6 clean cutover |

## Out of scope

- The legacy Python collector/scorer scripts and their removal (F02/F05/F23-owned).
- `gh` CLI behavior, GitHub branch protection, and Actions runtime semantics (the generator emits steps; the runner executes them).
- Usage refresh in CI (annex-b §8.5 — deliberately absent).
- `workflow_dispatch` input parameters, schedules other than the literal cron, and Actions version upgrades beyond the two pinned actions.
- The `--out` flag for other `catalog` subcommands (F23-owned).

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
`python3 -m unittest discover -s .daily-update/tests -v`. Generation or test failure
aborts publication. Both artifact arguments are shell-quoted consistently across
refresh, generation, and staging. Equal normalized destinations are invalid.
The score publication default is `data/available_model_scores.csv`, distinct from
the CLI's cache-path default. PR CI runs the Python suite, preserving deterministic
regeneration checks. The generator retains Python Decimal/schema behavior and
requires no Go runtime. Checkout pins in the renderer, golden, and committed
workflow remain synchronized. Regression cases cover custom paths, stage ordering,
generator failure before staging, and repeat generation matching committed bytes.


## Legacy run_tests compatibility — #167 review

Accept the documented `catalog.publish.run_tests` boolean, including its
environment override, as a legacy compatibility option. The option has no effect on the generated workflow; F30 governs its verification
steps. Paired-artifact verification is introduced by #165. This supersedes the former
conditional-test option without rejecting existing configurations.

### Half-hourly repository refresh and check-gated merge (September 2026)

The owner requested this repository refresh every 30 minutes and merge automatically after checks pass. The committed `which-model.toml` overrides the library schedule default with `*/30 * * * *`; the default for other configurations remains daily. This supersedes the former bot-approval/deferred-auto-merge flow, which could not satisfy this repository's review rules. The existing publishing token performs the normal merge only after explicit checks and an expected-head guard; branch rules remain unchanged. PR-mode publication requires that token so PR checks can run unattended.
