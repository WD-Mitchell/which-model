# Annex D — `which-model` CLI Reference

This annex specifies the complete command surface of the `which-model` binary: global conventions, every command's flags/output/exit codes, the determinism and concurrency contract for `which-model pick`, the config file schema and resolution order, the migration mapping from the two legacy prototypes' entry points, and shell-completion/man-page delivery. It is the normative contract that Annexes A (providers), B (scoring), and C (skills/hooks) invoke against; it does not re-derive provider internals, scoring maths, or the Go-vs-other-languages decision — see [master plan](./README.md) for those.

## 1. Global conventions

### 1.1 Binary and command tree

Binary name: **`which-model`**. Built with `cobra` (module `github.com/wdmitchell-uk/which-model`, entrypoint `cmd/which-model/main.go`). Three aliases ship alongside it — **`wm`**, **`wmodel`**, **`whichm`** — see §1.1a. Command tree:

```
which-model
├── usage [provider...]
├── auth
│   ├── status [provider...]
│   ├── login <provider>
│   └── logout <provider>
├── catalog
│   ├── refresh
│   ├── benchmarks
│   ├── scores
│   ├── list
│   ├── providers
│   └── workflow
├── pick
├── explain
├── routes
│   ├── list
│   ├── add
│   ├── remove
│   ├── refresh
│   └── verify
├── config
│   ├── show
│   ├── set
│   ├── path
│   └── validate
├── serve
├── schema [command]
└── version
```

### 1.1a Aliases

| Name | Role |
|---|---|
| `which-model` | Canonical. The name in docs, error messages, man pages, and `--help` output |
| `wm` | Shortest form for interactive use |
| `wmodel` | Mnemonic middle ground |
| `whichm` | For anyone who starts typing the canonical name and wants to stop early |

**Aliases are pure synonyms.** All four resolve to the identical binary and behave identically. `which-model` MUST NOT inspect `argv[0]` to alter behaviour — no busybox-style multi-call dispatch, no alias-specific defaults. An operator reading `wm pick …` in a script MUST be able to substitute `which-model pick …` and get byte-identical output.

Implementation is install-time symlinks, not Go code:

```sh
install -m 0755 which-model "$PREFIX/bin/which-model"
for a in wm wmodel whichm; do ln -sf which-model "$PREFIX/bin/$a"; done
```

Homebrew formula uses `bin.install "which-model"` then `bin.install_symlink bin/"which-model" => "wm"` (and the other two). `.deb`/`.rpm` packaging creates the same three symlinks in `postinst`/`%post`.

Consequences worth stating:

- **`--help` and errors always self-identify as `which-model`**, regardless of invocation name. Consistent, greppable diagnostics beat cosmetic echoing of whichever alias the user typed, and it means a pasted error message is unambiguous.
- **Completions and man pages are generated for `which-model` only.** Each alias gets a completion stub that delegates (`compdef wm=which-model` in zsh, `complete -o default -F __start_which-model wm` in bash), rather than three near-duplicate generated files that could drift apart.
- **`wm` is the collision risk to watch.** It is short and unqualified; a user may already have `wm` bound to a window-manager helper or a personal alias. Packaging MUST NOT clobber an existing `$PREFIX/bin/wm` silently — detect and skip with a warning, leaving the canonical name and the other two aliases installed. The other three names are distinctive enough not to need this guard.
- The env-var prefix is **`WHICH_MODEL_`**, not `WM_`, for the same reason: `$WM` is a long-standing window-manager convention and `WM_`-prefixed variables are used by X11 tooling. Aliasing the binary is cheap; colliding in the environment namespace is not.

### 1.2 Global flags

Available on every command (cobra persistent flags on the root command); per-command tables below list only flags additional to this set.

| flag | type | default | meaning |
|---|---|---|---|
| `--json` | bool | `false` | Emit the command's JSON schema-stable payload on stdout instead of the human text form. |
| `--schema` | bool | `false` | Instead of running the command, print its JSON Schema (equivalent to `which-model schema <command>`) and exit 0. |
| `--config <path>` | string | `""` (resolved, §4.5) | Explicit config file path; bypasses project/user discovery. |
| `--offline` | bool | `false` | Forbid all network I/O for this invocation; only cache/local sources are consulted. A stale-but-present cache is returned successfully with `Stale: true` (Annex A §6). A missing cache exits `1` (`fallback_unavailable`). A contradictory combination like `--refresh-benchmarks --offline` exits `2` (§1.6 rule 4). `--refresh-scores` is explicitly permitted under `--offline` since Derive needs no network (§1.6 rule 3). |
| `--refresh` | bool | `false` | Superset: runs Collect then Derive (§1.6) and bypasses the usage cache — equivalent to `--refresh-usage --refresh-benchmarks --refresh-scores` combined, "make it all current" (§1.6 rule 5). |
| `--refresh-usage` | bool | `false` | Ignore usage cache TTLs and force a live fetch where the command supports caching (`usage`, `pick`). Replaces the single global `--refresh` this annex previously specified for the usage domain; triggers no catalog stage. |
| `--refresh-benchmarks` | bool | `false` | Run catalog stage **Collect** (`catalog benchmarks`, §2.3): re-fetch AA v2 + models.dev and overwrite the raw CSV. REQUIRES `ARTIFICIAL_ANALYSIS_API`; exits `2` with a clear diagnostic if absent. Does NOT run Derive — leaves the scores CSV stale relative to the new raw CSV, which `warn_on_stale_scores` (§4.2) surfaces as a warning on next read, never a hard error (§1.6). Incompatible with `--offline` (exit `2`, §1.6 rule 4). |
| `--refresh-scores` | bool | `false` | Run catalog stage **Derive** (`catalog scores`, §2.3): regenerate the scores CSV from the on-disk raw CSV + benchmark config. Pure function (§1.6, §3): no `ARTIFICIAL_ANALYSIS_API`, no network. The flag to use after editing benchmark config. Permitted, and unaffected, under `--offline`. |
| `--max-age <duration>` | duration | provider/catalog `CacheTTL` | Treat cached data older than this as stale (forces refetch); overrides the descriptor/config TTL for this invocation only. |
| `--timeout <duration>` | duration | `10s` | Per-outbound-request timeout; passed through to `internal/usage/provider/*` fetchers and `internal/catalog/fetch/*` collectors. |
| `--quiet` | bool | `false` | Suppress non-error stderr diagnostics (progress, warnings still shown at `--verbose`≥1 only). |
| `--verbose` | count | `0` | Repeatable (`-v`, `-vv`); increases stderr diagnostic detail. Never affects stdout. |
| `--no-color` | bool | `false` (auto-detected from TTY/`NO_COLOR`) | Disable ANSI styling in text-mode stdout. |
| `--show-identity` | bool | `false` | Opt-in: allow provider account/login identity strings into text and JSON output (`Snapshot.Account`, Copilot `login`, etc.). Carried forward verbatim from the prototype's identity-opt-in posture (`research/usage-allowance-checks-spec.md` §2.3, `copilot-usage.mjs` `--show-identity`, §9 checklist "device-flow secrets stay in-memory only ... even the login/username is opt-in-only"). Without it, `Snapshot.Account` is omitted from output entirely, not merely masked. |
| `--no-usage` | bool | `false` | Level L0 of the usage toggle (§3.4, [master plan §6](./README.md)). Suppresses all usage acquisition for this invocation: no provider network calls, no credential reads, no usage-cache reads or writes. Existing cache is left intact, not invalidated. `which-model pick` degrades to pure score ranking; `which-model usage`/`which-model auth` exit `2`. Beats `[usage] enabled = true` in config. |
| `--normalizer <name>` | string | `minmax-linear` | Override the active score `Normalizer` (Annex B §4.0). Recorded in `which-model explain` evidence. |
| `--aggregator <name>` | string | `weighted-arithmetic-mean` | Override the active score `Aggregator` (Annex B §4.0). Recorded in `which-model explain` evidence. |

### 1.3 stdout/stderr discipline

MUST hold for every command, unconditionally:

- **stdout carries ONLY the command's machine-consumable or human-readable primary output** — the formatted report (text mode) or the single JSON document (`--json` mode). Nothing else is ever interleaved onto stdout.
- **stderr carries ALL diagnostics**: progress, warnings (e.g. broad credential-file-permission warnings — ported verbatim from `core.mjs:249-263`'s `runSafely` split, which the prototype already enforces), non-fatal per-provider errors, and the final failure line on error.
- This means `which-model usage --all --json | jq .` and `which-model pick --json | jq .` are always safe: a non-zero exit still leaves stdout either empty or containing a well-formed JSON error document (never partial/malformed JSON), per command below.
- Device-flow login prompts (`which-model auth login`) go to stdout when they represent the primary "here is what to do" output (matching `copilot-usage.mjs`'s `writeLogin` wiring to stdout, `research/usage-allowance-checks-spec.md` §2.3 item 3 and §6 line 303 — "login prompt goes to STDOUT, not stderr"); the interactive polling status ("waiting for confirmation…") goes to stderr as progress.
- **Failure-line format** is fixed: `which-model <command>: [<code>] <message>` on a single stderr line, where `<code>` is the stable machine code (`Failure.Code` for provider failures, `arguments` for exit-2 argument errors) and `<message>` is sanitized. The prototype's `Usage check failed [<code>]: <message>` prefix (`core.mjs:249-263`) is NOT carried forward — it is meaningless for non-usage commands such as `which-model pick`. The `[<code>]` bracket convention and the never-leak-credential-material guarantee ARE carried forward verbatim.

### 1.4 Unknown-flag rejection

Every command and subcommand MUST reject unrecognized flags and unexpected positional arguments as a hard error, exit code `2` — cobra's `SilenceUsage: false` default plus `Command.SetFlagErrorFunc` wired to return a non-nil error on unknown flags (cobra by default already errors on unknown flags with `UnknownFlags: false`; this MUST NOT be set to `true`/permissive anywhere in the tree). This inherits the prototype invariant `research/usage-allowance-checks-spec.md` §9 checklist: *"CLI argument surfaces are minimal, explicit allow-lists — unknown/extra flags are hard errors (`arguments` UsageError), not silently ignored"* (see also `claude-usage.mjs` line 294: any argument at all is a hard error; `codex-usage.mjs` line 298: exactly `--trust-configured-origin <origin>` or nothing). No `which-model` command MUST ever silently ignore or warn-and-continue on an unrecognized flag.

### 1.5 Exit codes (fixed, every command)

| code | meaning |
|---|---|
| `0` | Success. |
| `1` | Runtime error (network failure, malformed provider response, filesystem error, etc.) not covered by a more specific code below. |
| `2` | Argument/usage error — unknown flag, missing required value, mutually-exclusive flags both set, malformed flag value. |
| `3` | No viable candidate after filtering (`which-model pick` only — every routed model was excluded by `--exclude-provider`/`--max-used-percent`/availability filtering). |
| `4` | All eligible providers gated or exhausted (`which-model pick`, `which-model usage --fail-on-gated` — every candidate provider hit `gate_above_used_percent` or returned `Failure`). |
| `5` | Authentication required (`which-model usage`, `which-model pick`, `which-model auth status` — one or more required providers have no usable credential; matches the prototype's `login_required` code, `research/usage-allowance-checks-spec.md` core.mjs:16). |

Codes 3, 4, 5 are returned only by commands that evaluate providers/candidates (`usage`, `pick`, `auth status`); all other commands use only `{0,1,2}`.

### 1.6 Refresh flag semantics

The catalog pipeline is exactly two stages; every `--refresh*` flag and every `catalog` refresh-family subcommand maps onto one or both, never a third thing:

| Stage | Command form | Global flag | Input | Output | Needs `ARTIFICIAL_ANALYSIS_API`? | Needs network? |
|---|---|---|---|---|---|---|
| **Collect** | `catalog benchmarks` | `--refresh-benchmarks` | AA v2 API + models.dev | raw CSV (`catalog.raw_csv_path`) | **Yes** | Yes |
| **Derive** | `catalog scores` | `--refresh-scores` | raw CSV + benchmark config | scores CSV (`catalog.scores_csv_path`) | No | **No** |

Flag x stage x requirement matrix (includes the usage-only flag for completeness):

| Flag | Stages triggered | Needs key | Needs network |
|---|---|---|---|
| `--refresh-usage` | neither (usage cache bypass only) | no | yes |
| `--refresh-benchmarks` | Collect only | **yes** | yes |
| `--refresh-scores` | Derive only | no | no |
| `--refresh` | usage + Collect + Derive | **yes** | yes |

Normative rules (binding on both the flag surface and the `catalog` subcommand surface, §2.3):

1. **Ordering.** When an invocation triggers both stages (`--refresh`, or `catalog refresh`), Collect MUST run before Derive, never the reverse — Derive consumes Collect's output.
2. **Staleness warning, not a hard error.** The scores CSV records the content hash of the raw CSV it was derived from. `--refresh-benchmarks` alone (or `catalog benchmarks` alone) leaves that hash stale; the next read of the scores CSV by any command detects the mismatch and emits a staleness warning to stderr. This mirrors the route-table staleness mechanism already specified in Annex B §7.2.
3. **Derive is a pure function** of (raw CSV, benchmark config, active `Normalizer`, active `Aggregator`). It is runnable fully offline and MUST NOT be blocked by `--offline` (§3).
4. **`--refresh-benchmarks` (equivalently `catalog benchmarks`) under `--offline` is an argument error, exit `2`** — Collect requires network by definition and cannot possibly succeed offline.
5. **`--refresh` is deliberately the full superset**, including usage, because that is the least surprising reading of a bare `--refresh`. `--refresh-usage` gives precise, usage-only control.
6. **No new flag silently implies another.** `--refresh-benchmarks` does NOT auto-run Derive; it warns instead (rule 2) — an operator may legitimately want to inspect newly-collected raw data before rebuilding scores.
---

## 2. Per-command reference

### 2.1 `which-model usage [provider...]`

**Synopsis:** `which-model usage [provider...] [--all] [--source auto|oauth|api|cli|web|local] [--include-cost] [--window <id>] [--fail-on-gated]`

**Description:** Fetches and reports `Snapshot`s for the named providers (positional args, provider `Descriptor.ID`s) or, with `--all`, every enabled provider in the resolved config's `[providers.<id>]` tables (`internal/config`). Equivalent in spirit to running all three of `claude-usage.mjs`/`codex-usage.mjs`/`copilot-usage.mjs`, generalized to N providers via `internal/usage/registry.go`.

| flag | type | default | meaning |
|---|---|---|---|
| `--all` | bool | `false` | Report every enabled provider instead of requiring positional names. Mutually exclusive with positional args (exit 2 if both given). |
| `--source` | enum | `auto` | Force a specific `Source` (`oauth\|api\|cli\|web\|local`) instead of the descriptor's ordered `AuthSource` fallback chain; exit 2 if the provider has no matching source. |
| `--include-cost` | bool | `false` | Include `UnitUSD`/`UnitCredits` windows in output (some providers report these only on request to avoid noisy default output, e.g. Codex `credits.balance`, `research/usage-allowance-checks-spec.md` codex.mjs:172). |
| `--window <id>` | string, repeatable | all windows | Restrict output to the named `Window.ID`(s). |
| `--fail-on-gated` | bool | `false` | Exit `4` if any reported provider's usage crosses `gate_above_used_percent` (§4, `[bands]`), instead of exit `0` with a warning annotation. |

**stdout (text mode)** — one block per provider, format ported verbatim from `formatUsageReport` (`research/usage-allowance-checks-spec.md` core.mjs:230-247: header line `"<Provider> usage allowance"`, then `"- <label>: <detail1>; <detail2>; ..."` per window, `unlimited` exclusive of the percent/count fields, `resetAt` always last):

```
$ which-model usage claude codex
Claude usage allowance
- five hour: 25% used; 75% available; resets 2026-08-07T18:00:00Z
- seven day: 41% used; 59% available

Codex usage allowance
- primary window: 12% used; 88% available; resets 2026-08-08T00:00:00Z
- credits: 340 remaining
```

**stdout (`--json`)** — `[]Snapshot` (§ contract types), one entry per requested provider, in request order:

```console
$ which-model usage claude --json
[
  {
    "provider": "claude",
    "windows": [
      {"id": "5h", "label": "five hour", "unit": "percent", "used_percent": 25, "resets_at": "2026-08-07T18:00:00Z", "usage_known": true},
      {"id": "7d", "label": "seven day", "unit": "percent", "used_percent": 41, "usage_known": true}
    ],
    "fetched_at": "2026-08-07T17:03:11Z",
    "source": "oauth",
    "confidence": "live"
  }
]
```

**Exit codes:** `0` all requested providers reported (even if individual providers carry a non-fatal `Failure` inline — see below); `1` unexpected runtime error; `2` bad flags/provider name/mutual exclusion; `4` with `--fail-on-gated` and a gate was crossed; `5` if EVERY requested provider failed with `unauthorized`/`login_required`-class `Failure.Code` (any single provider auth failure alone is reported inline as `Snapshot.Failure`, not a nonzero exit — mirrors the prototype's per-script isolation, since multi-provider batching is new in `which-model`).

**Examples:**
```
$ which-model usage --all --include-cost
Claude usage allowance
- five hour: 25% used; 75% available
Codex usage allowance
- primary window: 12% used; 88% available
- credits: 340 remaining
GitHub Copilot usage allowance
- chat: 1200 remaining; 4800 total
GitHub identity verified.

$ which-model usage copilot --json --show-identity | jq '.[0].account'
"octocat"

$ which-model usage claude --window 5h --fail-on-gated; echo $?
Claude usage allowance
- five hour: 99% used; 1% available
4
```

### 2.2 `which-model auth status|login|logout <provider>`

**Synopsis:** `which-model auth status [provider...]` / `which-model auth login <provider> [--trust-configured-origin <https-origin>]` / `which-model auth logout <provider>`

**Description:** Credential lifecycle. `status` performs a lightweight liveness/identity check without full usage fetch. `login` runs a provider's interactive auth flow (device-code, browser, etc.) — see below for the mandatory unattended-refusal rule. `logout` clears cached/local credential material `which-model` itself wrote (never touches provider-native credential stores such as `~/.claude/credentials.json`, `~/.codex/auth.json`, or `git config` — those remain owned by their respective CLI/IDE tools, per the prototype's read-only posture).

| flag | type | default | meaning | applies to |
|---|---|---|---|---|
| `--login` | bool | `false` | `status` only: attempt `login` inline if no usable credential is found, instead of returning exit `5`. | `status` |
| `--trust-configured-origin <https-origin>` | string | none | Exact-origin opt-in for providers with a config-file-declared fallback base URL (ported verbatim from Codex's `--trust-configured-origin`, `research/usage-allowance-checks-spec.md` §2.2/§3/§9). MUST be validated with the exact rules of `validateTrustedBaseUrl` (core.mjs:93-116): HTTPS only, no userinfo/query/fragment on either side, trust argument MUST be a bare origin (`pathname == "/"`), and MUST equal the configured base URL's origin exactly — protocol+host+port, not prefix/substring. Passing it when the provider declares no fallback URL is a no-op, not an error. Never persisted to config by this flag; it is a per-invocation opt-in only. | `login` (providers with a declared fallback, e.g. `codex`) |
| `--show-identity` | bool | inherits global | Show resolved account/login string in `status` output. | `status` |

**Unattended-refusal rule (normative):** `login` for any provider whose only auth flow is a device-code or interactive-browser flow (Copilot device flow, and any CodexBar-derived provider using the same pattern per Annex A) MUST detect a non-interactive stdin (`!isatty(stdin)`) or the presence of `WHICH_MODEL_NONINTERACTIVE=1`, and MUST refuse to start the flow, exiting `2` with a message directing the operator to run it from an interactive terminal. This is a strengthening of the prototype's posture (`copilot-usage.mjs` had no such guard because it always ran interactively as a human-invoked skill) made necessary because `which-model` is agent-invoked and a hung device-flow poll in a non-interactive context is a silent stall, not a safe failure.

**stdout (text, `status`):**
```
$ which-model auth status claude codex
claude   ok       oauth   (expires 2026-09-01T00:00:00Z)
codex    ok       oauth
copilot  missing  -       run: which-model auth login copilot
```

**stdout (`--json`, `status`):**
```json
[
  {"provider": "claude", "status": "ok", "source": "oauth", "expires_at": "2026-09-01T00:00:00Z"},
  {"provider": "copilot", "status": "missing", "source": null}
]
```

**stdout (`login`, text, Copilot device flow):**
```
$ which-model auth login copilot
Open https://github.com/login/device and enter code WXYZ-1234.
```
(stderr carries `waiting for confirmation...` progress; exact two validated fields only, per `research/usage-allowance-checks-spec.md` §2.3/§4 table — device code and raw token are never printed.)

**Exit codes:** `0` (status: all queried providers `ok`; login/logout: success); `1` runtime; `2` bad flags, unattended device-flow refusal, or invalid `--trust-configured-origin`; `5` (`status` only, without `--login`: one or more queried providers have no usable credential).

### 2.3 `which-model catalog refresh|benchmarks|scores|list|providers|workflow`

**Synopsis:** `which-model catalog {refresh|benchmarks|scores|list|providers|workflow} [flags]`

**Description:** Model-data pipeline commands, replacing `get_provider_models.py`, `get_benchmarks.py`, `get_aa_api_values.py`/`get_aa_page_values.py`, `update_raw_values.py`, and `generate_scores.py` (§5 migration table) as one Go subcommand group backed by `internal/catalog`. The pipeline is the two independently invocable stages of §1.6, Collect and Derive: `catalog benchmarks` runs Collect alone, `catalog scores` runs Derive alone, and `catalog refresh` runs both in strict Collect-then-Derive order (§1.6 rule 1) — the subcommand form of the global `--refresh-benchmarks` + `--refresh-scores` combination. Usage has no catalog-command form; use `which-model usage --refresh-usage` / the global `--refresh-usage` flag instead.

| flag | type | default | meaning | applies to |
|---|---|---|---|---|
| `--provider <id>` | string, repeatable | all configured | Restrict to named provider(s) from `providers.toml`-equivalent config; ported from `get_provider_models.py --provider` / `update_raw_values.py --provider` (research doc §2, §8.4 "partial `--provider` refresh updates only selected providers' rows and preserves the rest"). | `refresh`, `benchmarks`, `providers` |
| `--provider-config <path>` | path | `internal/config` default | Path to the provider allow-list/exclusion TOML (replaces `providers.toml`). | `refresh`, `benchmarks`, `providers` |
| `--add aa_page` | string, repeatable, choices `{aa_page}` | none | Opt in an optional collector; currently only `aa_page` (AA public model-page scrape), ported verbatim from `update_raw_values.py --add`/`OPTIONAL_SOURCES` (research doc §1.4, §8.4). Never enabled by default, and never run by the scheduled Action (§2.3a, Annex B §8.5). | `refresh`, `benchmarks` |
| `--benchmarks <path>` | path | `internal/config` default | Path to the benchmark-group selection TOML (replaces `benchmarks.toml`). | `refresh`, `scores` |
| `--in <path>` | path | resolved catalog dir | Input CSV path override. | `scores` (raw CSV in), `list` |
| `--out <path>` | path | resolved catalog dir | Output CSV path override. | `refresh` (both CSVs), `benchmarks` (raw CSV out), `scores` (scores CSV out), `workflow` (workflow YAML out) |
| `--write` | bool | `false` | Generate/overwrite the workflow file from `[catalog.publish]` (§4.2, §2.3a). | `workflow` |
| `--check` | bool | `false` | Render the workflow from `[catalog.publish]` in-memory and diff it against the committed file; exit non-zero with the diff on stderr if they differ. Mutually exclusive with `--write` (exit `2` if both given). Intended for a CI lint job. | `workflow` |

`refresh` runs Collect (`get_provider_models`-equivalent + AA v2 + optional AA page) then Derive, and performs the same transactional write as the prototype for each output CSV: temp-file write, fsync, compare-and-swap against the on-disk CSV, timestamped `.bak`, atomic rename (research doc §9, `csv_store.py:248-273`) — reimplemented via `os.CreateTemp` in the target dir + `os.Rename`. `benchmarks` runs Collect alone (§1.6 rule 2: warns on next scores read rather than erroring). `scores` runs Derive alone: a pure function of the raw CSV + benchmark config (research doc §9: "regenerated wholesale, no incremental merge"), no network, no `ARTIFICIAL_ANALYSIS_API`, explicitly permitted under `--offline` (§1.6 rules 3-4). `list`/`providers` are read-only views over the on-disk catalog.

### 2.3a `which-model catalog workflow`

Generates `.github/workflows/refresh-model-data.yml` from `[catalog.publish]` (§4.2). GitHub Actions' `on.schedule` cron MUST be literal YAML at trigger time — the Action runner cannot read it from a config file — so `catalog workflow --write` is the single source of truth that renders `[catalog.publish]` into that literal YAML, keeping the two from drifting apart by hand-edit. The generated workflow performs the Action-side equivalent of `catalog refresh` (Collect then Derive, using a repository `ARTIFICIAL_ANALYSIS_API` secret; Annex B §8.5 — usage refresh is never part of the Action, since CI holds no provider credentials), stages both CSVs with `git add`, commits only if `git diff --cached --quiet` reports a change, runs the fail-closed test gate first, and for each of `[catalog.publish].branches` in listed order either opens a PR and runs `gh pr merge --auto --<merge_method>` (`mode = "pull-request"`) or pushes directly (`mode = "direct-push"`) — exactly one commit applied per branch, a failure on one branch never aborting the others. `--check` renders the same template and diffs it against the committed file, exiting non-zero on drift — wired into CI, a `[catalog.publish]` edit nobody regenerated the workflow for fails the build instead of silently diverging. `enabled = false` makes `--write` a no-op that removes an existing generated workflow (emits nothing if none exists) — the escape hatch for on-demand-refresh-only users.

**stdout (`refresh`, text):**
```console
$ which-model catalog refresh
collected 39 providers, 412 models -> /Users/will/.cache/which-model/catalog/available_model_raw_values.csv
derived 39 rows -> /Users/will/.cache/which-model/catalog/available_model_scores.csv
```

**stdout (`benchmarks`, text):**
```console
$ which-model catalog benchmarks
collected 39 providers, 412 models -> /Users/will/.cache/which-model/catalog/available_model_raw_values.csv
warning: scores CSV is stale relative to raw CSV; run `which-model catalog scores` (or `--refresh-scores`) to rebuild
```

**stdout (`scores`, text):**
```console
$ which-model catalog scores
wrote 39 rows -> /Users/will/.cache/which-model/catalog/available_model_scores.csv
```

**stdout (`list`, `--json`):**
```json
[{"model": "Claude Opus 5", "reasoning": "max", "intelligence_index": "63.1", "cost_per_intelligence_index_task_usd": "2.34"}]
```

**stdout (`providers`, text):**
```console
$ which-model catalog providers
anthropic       12 models   3 excluded
openai           9 models   0 excluded
github-copilot   4 models   1 excluded (grok-4.5)
```

**stdout (`workflow --write`, text):**
```console
$ which-model catalog workflow --write
wrote .github/workflows/refresh-model-data.yml (schedule="0 6 * * *", branches=[main], mode=pull-request)
```

**stdout (`workflow --check`, drift found):**
```console
$ which-model catalog workflow --check; echo $?
--- .github/workflows/refresh-model-data.yml (committed)
+++ .github/workflows/refresh-model-data.yml (generated from [catalog.publish])
@@ -3,7 +3,7 @@
 on:
   schedule:
-    - cron: "0 6 * * *"
+    - cron: "0 8 * * *"
1
```

**Exit codes:** `0` success (including `workflow --check` with no drift); `1` collector/transport error (network, malformed upstream payload, CSV compare-and-swap conflict — research doc §9's "a fetch failure leaves the original file untouched and creates no backup"), or `workflow --check` reporting drift; `2` bad flags (unknown `--add` value, unknown provider id, `--write`+`--check` both given, `benchmarks`/Collect run without `ARTIFICIAL_ANALYSIS_API`, or `benchmarks --offline`).

**Examples:**
```console
$ which-model catalog refresh --provider claude --provider codex
collected 2 providers, 21 models -> /Users/will/.cache/which-model/catalog/available_model_raw_values.csv
derived 39 rows -> /Users/will/.cache/which-model/catalog/available_model_scores.csv

$ which-model catalog benchmarks --offline; echo $?
which-model catalog benchmarks: [arguments] Collect requires network access; incompatible with --offline
2

$ ARTIFICIAL_ANALYSIS_API= which-model catalog benchmarks; echo $?
which-model catalog benchmarks: [arguments] ARTIFICIAL_ANALYSIS_API is not set; the Collect stage requires an Artificial Analysis API key
2

$ which-model catalog scores --offline
wrote 39 rows -> /Users/will/.cache/which-model/catalog/available_model_scores.csv
```

### 2.4 `which-model pick`

**Synopsis:** `which-model pick [flags]`

**Description:** Selects one (or, with `--top`, ranked several) model+provider `Candidate`s by joining live usage (`internal/usage`) against catalog scores (`internal/catalog`) through `internal/routing`, then applying a `internal/pick` strategy. Supersedes and generalizes `rank_models.py` (research doc §6.8/§8.2) by adding provider-usage awareness (bands) and multiple strategies; `rank_models.py`'s pure ranking-by-profile behavior is exactly the `--strategy score` path with no usage weighting when `--offline` is set.

| flag | type | default | meaning |
|---|---|---|---|
| `--profile <name>` | string | `balanced_implementation` | Named scoring profile from `internal/pick` (ported set incl. `planning`, `orchestration`, `balanced_implementation`; Annex B owns the weight tables). |
| `--strategy` | enum | `score` | One of `score\|priority\|round-robin\|least-used\|weighted-random\|cost-optimal` (§3). |
| `--top <n>` | int | `1` | Number of ranked candidates to return; `1` = single pick (default CLI ergonomics), `>1` = ranked list, matches `rank_models.py --top` default `5` generalized. |
| `--seed <n>` | int64 | none | RNG seed; REQUIRED when `--strategy weighted-random` (exit 2 otherwise, §3). |
| `--weights-json <path\|->` | path or `-` for stdin | none | Full custom profile as JSON; ported from `rank_models.py --weights-json` (research doc §6.8 arg table). |
| `--tier1-weight NAME=VALUE` | string, repeatable | none | Ported from `rank_models.py --tier1-weight`, `metavar="NAME=VALUE"`. |
| `--tier2-weight NAME=VALUE` | string, repeatable | none | Ported from `rank_models.py --tier2-weight`. |
| `--available <path>` | path, repeatable | none | Live-availability filter file(s); ported from `rank_models.py --available` (`type=Path, action="append"`). |
| `--identity <model\|reasoning>` | string, repeatable | none | Ported from `rank_models.py --available-identity`, renamed `--identity` for brevity; format unchanged: `MODEL\|REASONING`. |
| `--provider <id>` | string, repeatable | all enabled | Allow-list of usage providers to consider for routing. |
| `--exclude-provider <id>` | string, repeatable | none | Deny-list; applied after `--provider`. |
| `--max-used-percent <pct>` | float | none | Drop any candidate whose gating window's `UsedPercent` exceeds this, independent of `[bands]` gating. |
| `--band-config <path>` | path | resolved config | Override the `[bands]`/`[[bands.tier]]` table (§4.2) for this invocation. |
| `--require-live` | bool | `false` | Reject candidates backed only by `Confidence: cached` or `estimated` usage snapshots; forces `Source` freshness. |
| `--dry-run` | bool | `false` | Compute and print the pick(s) without advancing any persisted state (round-robin cursor, weighted-random consumption) — see §3. |

**Mutual-exclusion rule (normative, ported verbatim in spirit from `rank_models.py`'s custom-profile handling, research doc §8.2 `test_custom_json_and_repeated_weights_require_tier_one`):** `--weights-json` and any use of `--tier1-weight`/`--tier2-weight` are mutually exclusive. Supplying `--weights-json` together with one or more `--tier1-weight`/`--tier2-weight` flags is an argument error, exit `2`, message naming both forms and instructing the caller to pick one. Both paths independently still enforce the Tier-1 completeness/validation rules from Annex B (a custom profile missing a mandatory Tier-1 metric weight is exit `2`, not silently defaulted).

**stdout (text):**
```
$ which-model pick --profile orchestration --top 3
1. codex        gpt-5.6-sol       reasoning=max     score=88.4  band=standard(0.85)  final=75.1
2. claude       claude-opus-5     reasoning=max     score=91.0  band=elevated(0.60)  final=54.6
3. copilot      gpt-5.6-sol       reasoning=high    score=79.2  band=low(1.00)       final=79.2
```

**stdout (`--json`):**
```json
{
  "candidates": [
    {
      "route": {"provider": "codex", "model_id": "gpt-5.6-sol", "model": "GPT-5.6 Sol", "reasoning": "max", "window_ids": ["primary window"]},
      "model_score": "88.4", "band": "standard", "band_weight": "0.85", "provider_weight": "1.0", "final_score": "75.14",
      "warnings": [], "evidence": {"catalog_row_hash": "...", "usage_snapshot_fetched_at": "2026-08-07T17:03:11Z"}
    }
  ],
  "strategy": "score", "profile": "orchestration", "picked_at": "2026-08-07T17:03:12Z"
}
```

**Exit codes:** `0` at least one candidate returned; `1` runtime error (catalog/usage fetch failure not otherwise classified); `2` bad flags including the mutual-exclusion violation above, missing `--seed` for `weighted-random`; `3` every routed candidate was excluded by filters (`--exclude-provider`, `--max-used-percent`, `--available`); `4` every eligible provider is gated (`gate_above_used_percent`) or returned a `Failure`; `5` no candidate provider has a usable credential.

**Examples:**
```
$ which-model pick --strategy cost-optimal --exclude-provider copilot --json | jq '.candidates[0].route.model'
"Kimi K2.7 Code"

$ which-model pick --weights-json ./custom-profile.json --tier1-weight cost=3; echo $?
which-model pick: [arguments] --weights-json and --tier1-weight/--tier2-weight are mutually exclusive; use one form.
2

$ which-model pick --strategy round-robin --dry-run
1. claude       claude-opus-5     reasoning=max     score=91.0  band=low(1.00)  final=91.0   (dry-run: cursor not advanced)
```

### 2.5 `which-model explain [--last | --pick-id <id>]`

**Synopsis:** `which-model explain [--last] [--pick-id <id>]`

**Description:** Reconstructs and prints the full scoring/filtering trail for a prior `which-model pick` invocation, read from the pick-history log (`internal/pick` writes one `Evidence`-bearing JSON line per pick to the state directory, §3/§4.5). Exactly one selector is required.

| flag | type | default | meaning |
|---|---|---|---|
| `--last` | bool | `false` | Explain the most recent pick recorded in this machine's state file. Mutually exclusive with `--pick-id`. |
| `--pick-id <id>` | string | none | Explain a specific historical pick by its recorded id (ULID). Mutually exclusive with `--last`. |

**stdout (text):**
```
$ which-model explain --last
pick 01J... at 2026-08-07T17:03:12Z, strategy=score, profile=orchestration
selected: codex / gpt-5.6-sol (reasoning=max)
  model_score  88.4  (tier1=92.1 x0.6 + tier2=81.0 x0.4)
  band         standard (used_percent=42.0, tier upper=50, weight=0.85)
  provider_weight 1.0
  final_score  75.14
rejected:
  copilot / gpt-5.6-sol: excluded by --exclude-provider
  claude  / claude-opus-5: final=54.6 (lower than selected)
```

**stdout (`--json`):** the full `Candidate[]` (selected + rejected, each with `Evidence` and rejection reason) plus the original `pick` invocation's resolved flags.

**Exit codes:** `0` found and printed; `1` state file unreadable/corrupt; `2` neither or both of `--last`/`--pick-id` given, or `--pick-id` malformed.

### 2.6 `which-model routes list|add|remove|refresh|verify`

**Synopsis:** `which-model routes {list|add|remove|refresh|verify} [flags]`

**Description:** Manages the `Route` table (provider-native model id ↔ catalog `Model`/`Reasoning` join) — new in `which-model`; neither prototype has this concept, since `usage-allowance-checks` never mentions model identity and `rank_models.py` only ever consumed catalog rows with no provider linkage.

| flag | type | default | meaning | applies to |
|---|---|---|---|---|
| `--provider <id>` | string | required (add/remove) | Usage provider id the route belongs to. | `add`, `remove`, `list` (filter) |
| `--model-id <id>` | string | required (add/remove) | Provider-native model identifier (e.g. `claude-opus-4-5-20251101`). | `add`, `remove` |
| `--model <name>` | string | required (add) | Catalog display name to join against (`ScoreRow.Model`). | `add` |
| `--reasoning <level>` | enum | `default` | Catalog `Reasoning` level to join against. | `add` |
| `--window <id>` | string, repeatable | none | Usage `Window.ID`(s) that gate this route. | `add` |
| `--auto` | bool | `false` | `refresh`: re-derive routes automatically by fuzzy-matching provider window `ModelScope` against catalog model names (best-effort; flags ambiguous matches as warnings, never silently drops one). | `refresh` |

**stdout (`list`, text):**
```
$ which-model routes list --provider codex
codex   gpt-5.6-sol            max     -> GPT-5.6 Sol       windows=[primary window]
codex   claude-opus-4-5-...    max     -> Claude Opus 5     windows=[primary window]
```

**stdout (`verify`, text):** reports routes whose `Route.Model`/`Route.Reasoning` no longer resolve against the current catalog scores CSV (stale after a `catalog refresh`), one per line, with exit `1` if any are stale.

**Exit codes:** `0` success; `1` verify found stale routes, or refresh I/O error; `2` bad/missing required flags on `add`/`remove`.

### 2.7 `which-model config show|set|path|validate`

**Synopsis:** `which-model config {show|set <key> <value>|path|validate}`

| subcommand | behavior |
|---|---|
| `show` | Prints the fully resolved config (after merging flags > env > project > user > defaults, §4.5) as TOML (text) or JSON (`--json`). |
| `set <key> <value>` | Writes `key = value` into the user config file (`~/.config/which-model/config.toml` / macOS equivalent, §4.6), creating it if absent. Dotted key path, e.g. `which-model config set providers.claude.priority 5`. |
| `path` | Prints the resolved config file path that would be read/written (one line, no other output — pipeline-friendly: `$EDITOR "$(which-model config path)"`). |
| `validate` | Parses the resolved config against the `internal/config` schema and reports every error (unknown keys, wrong types, out-of-range band weights) without applying it; exit `1` if any error found. |

**Exit codes:** `0` success (including `validate` with zero errors); `1` `validate` found errors, or `set`/`show` I/O error; `2` bad key path on `set`.

### 2.8 `which-model serve`

**Synopsis:** `which-model serve [--warm] [--interval <duration>] [--listen <addr>]`

**Description:** Long-running cache-warm daemon. **Background refresh is strictly opt-in and never implicit** — `which-model` never spawns background work as a side effect of any other command, matching the prototype's explicit no-daemonization posture (`research/usage-allowance-checks-spec.md` §9 checklist: *"No background polling, persistence, or auto-invocation — each script performs exactly one bounded, explicit, foreground provider check per invocation; no daemonization, no credential caching across runs, no automatic spawn gating"*). `which-model serve` is the ONLY command that runs continuously; every other command is a single bounded invocation that exits.

| flag | type | default | meaning |
|---|---|---|---|
| `--warm` | bool | `false` | Periodically pre-fetch usage snapshots and catalog data into the shared cache so foreground `which-model usage`/`which-model pick` calls hit warm cache. Without `--warm`, `serve` only exposes `--listen` (cache read API) and never performs its own fetches. |
| `--interval <duration>` | duration | `5m` | Warm-refresh period; only meaningful with `--warm`. |
| `--listen <addr>` | string | none (stdio-only) | Optional local HTTP/unix-socket address exposing cached `Snapshot`/`ScoreRow` reads to other local processes, avoiding redundant credential reads across concurrent agent invocations. |

**stdout:** structured log lines (JSON if `--json`, else plain text), one per warm cycle / request served; this is the one command where stdout is a log stream, not a single terminal document — callers that need machine output pipe through `--json` and consume newline-delimited JSON.

**Exit codes:** `0` clean shutdown (SIGINT/SIGTERM); `1` fatal startup error (bind failure, config error); `2` bad flags.

### 2.9 `which-model schema [command]`

**Synopsis:** `which-model schema [command...]`

**Description:** Emits JSON Schema for one command's `--json` output (or, with no argument, an index of all schemas). Equivalent to `which-model <command> --schema` but usable without constructing a full valid invocation of that command.

**stdout:** one JSON Schema document (or, with no args, `{"commands": ["usage", "pick", ...]}`).

**Exit codes:** `0` known command; `2` unknown command name.

### 2.10 `which-model version`

**Synopsis:** `which-model version [--json]`

**Description:** Build version/commit/date plus, per configured provider, a `LastVerified` summary sourced from Annex A's per-provider verification metadata (the date/commit at which that provider's auth flow and endpoints were last confirmed against the live service) — surfaces staleness risk for providers nobody has exercised recently.

**stdout (text):**
```
$ which-model version
which-model 0.4.0 (commit a1b2c3d, built 2026-08-01)
providers:
  claude    tier=1  last_verified=2026-07-28
  codex     tier=1  last_verified=2026-07-28
  copilot   tier=1  last_verified=2026-07-30
  cursor    tier=2  last_verified=2026-06-02
```

**stdout (`--json`):** `{"version": "0.4.0", "commit": "a1b2c3d", "built_at": "...", "providers": [{"id": "claude", "tier": 1, "last_verified": "2026-07-28"}]}`.

**Exit codes:** `0` only.

---

## 3. Determinism contract

| command / strategy | pure function of (inputs, config)? | notes |
|---|---|---|
| `catalog scores` (Derive) | Yes | Pure function of (raw CSV, benchmark config, active `Normalizer`, active `Aggregator`) — §1.6. No network, no clock dependency; byte-reproducible from the same inputs. Research doc §9: "entirely a pure function of the raw CSV + `benchmarks.toml`; regenerated wholesale." |
| `catalog benchmarks` (Collect) | No | Live network fetch against the AA v2 API + models.dev; output depends on upstream service state at call time — inherently non-reproducible. Incrementally merged against prior CSV (research doc §9). |
| `catalog refresh` | No | Collect then Derive, strictly ordered (§1.6 rule 1); inherits Collect's non-reproducibility even though the Derive half alone is pure. |
| `pick --strategy score` | Yes | Deterministic given (catalog scores, usage snapshots, band config, profile weights). No hidden state. |
| `pick --strategy priority` | Yes | Deterministic; ranks by configured `[providers.<id>].priority` then `score` as tiebreak. |
| `pick --strategy least-used` | Yes | Deterministic given the same usage snapshots (picks the lowest `UsedPercent` gating window); NOT deterministic across time since usage changes, but is a pure function of the snapshot passed in. |
| `pick --strategy cost-optimal` | Yes | Deterministic; ranks by `cost_per_intelligence_index_task_usd_score` (Annex B) among candidates meeting a minimum `model_score` floor. |
| `pick --strategy round-robin` | **No** | Reads and advances a persisted cursor (below). Two calls with identical inputs but different call order return different results by design. |
| `pick --strategy weighted-random` | Conditionally | Deterministic ONLY when `--seed` is supplied (uses a seeded PRNG, `math/rand/v2` with an explicit `NewPCG(seed, seed)` source); `--seed` is REQUIRED for this strategy (exit `2` without it) specifically so "deterministic" is opt-out-proof, not opt-in-by-accident. |

**Determinism boundary (Collect vs. Derive, §1.6).** `catalog scores`/`--refresh-scores` (Derive) is a pure function of (raw CSV, benchmark config, `Normalizer`, `Aggregator`) with no network and no clock dependency, so it is byte-reproducible given the same inputs. `catalog benchmarks`/`--refresh-benchmarks` (Collect) is inherently non-reproducible: it samples live upstream APIs (AA v2, models.dev) whose responses vary run to run. This is the same boundary the rest of §3 draws for `pick` strategies — a stage is deterministic exactly when it touches no live network state — and it is why `catalog scores` alone remains safely runnable under `--offline` while `catalog benchmarks` cannot be.

### 3.1 Round-robin state file

- **Path:** `<state_dir>/pick/round_robin.json` where `<state_dir>` follows §4.6 XDG/macOS resolution (e.g. `~/.local/state/which-model/pick/round_robin.json` on Linux, `~/Library/Application Support/which-model/state/pick/round_robin.json` on macOS).
- **Format:** JSON object keyed by a cursor scope key = `sha256(profile + "|" + sorted(candidate route keys))[:16]`, so distinct filter/profile combinations maintain independent cursors: `{"<scope_key>": {"index": 3, "updated_at": "2026-08-07T17:03:12Z"}}`.
- **Cursor advance semantics under contention:** every `which-model pick --strategy round-robin` invocation (unless `--dry-run`) performs: (1) open the state file with `O_RDWR|O_CREATE`, (2) take an exclusive advisory lock via `golang.org/x/sys/unix.Flock(fd, LOCK_EX)` (POSIX `flock(2)`; on the target macOS/Linux dev and CI hosts this is sufficient and avoids the portability cost of a full cross-platform file-locking abstraction), (3) read current cursor for the scope key (default `0` if absent), (4) compute `candidate := candidates[index % len(candidates)]`, (5) write `index+1` back, (6) fsync, (7) release the lock. Steps 3–6 are the critical section; the lock guarantees at-most-one writer at a time, so N concurrent `which-model pick --strategy round-robin` invocations against the same scope key deterministically partition into a contiguous cyclic sequence with no repeats and no skips, though the specific interleaving order among simultaneously-blocked callers is unspecified (OS lock-queue order). `--dry-run` skips step 5 entirely (still takes the read lock for a consistent read, never upgrades to a write).
- Corrupt/unreadable state file is treated as an empty cursor set (index `0` for every scope), never a fatal error — round-robin degrades gracefully rather than blocking picks.

### 3.2 Weighted-random

`--seed <int64>` is mandatory for `--strategy weighted-random` (exit `2` if omitted, message: `weighted-random requires --seed for reproducibility`). No state file — a seeded run is a pure function of `(seed, candidate scores)`. `--dry-run` has no distinct effect for this strategy since nothing is persisted regardless.

### 3.3 Usage toggle and degraded determinism

With usage off at any level, `which-model pick` becomes a pure function of `(scores CSV, config)` — no network, no cache dependency, no clock. It is therefore **more** deterministic than the enabled path, not less, and a degraded pick MUST be byte-reproducible from the same scores CSV.

| Strategy | Degraded | Behaviour |
|---|---|---|
| `score` | works | Unchanged. Default |
| `priority` | works | Static config `priority`; "first provider with capacity" becomes "first provider with any routed candidate" |
| `round-robin` | works | Rotates providers without consumption data — legitimate load-spreading without telemetry. **Still advances the §3.1 cursor**, so it remains the one stateful strategy in either mode |
| `weighted-random` | works | Samples proportional to `FinalScore`, which is now the pure model score |
| `cost-optimal` | works | Cost is a static catalog metric, not usage-derived |
| `least-used` | **refused** | Requires consumption data by definition. Exit `2`: `least-used requires usage data; usage is disabled by <source>`. **Never** silently falls back to `score` |

`least-used` is the only casualty. The refusal names the disabling source (`--no-usage`, the `[usage] enabled` key, or the `nousage` build) so the operator knows which lever to move.

### 3.4 Toggle levels and precedence

| Level | Mechanism | Effect |
|---|---|---|
| L0 | `--no-usage` | This invocation only |
| L1 | `[usage] enabled = false` | Every invocation; `usage`/`auth` refuse with exit `2` |
| L1a | `[providers.<id>].enabled` | Per provider; **default-deny** (§4.2) |
| L2 | `go build -tags nousage` | Not linked; `usage`/`auth` not registered in the command tree at all |

Precedence, highest first: **L2 beats everything** and cannot be re-enabled at runtime — no flag, env var, or config value can resurrect a `nousage` binary. Then `--no-usage` (flag) beats `[usage] enabled = true` (config), consistent with the §4.1 flags-over-config order. `WHICH_MODEL_USAGE_ENABLED` follows the standard `WHICH_MODEL_` env mapping and sits between flag and config as §4.1 specifies.

All toggle-related refusals use exit **`2`**, not `1`: they are configuration errors, not runtime failures. The distinction matters for agents — exit `1` invites a retry, exit `2` says the invocation itself was wrong and retrying it unchanged will fail identically. See Annex A §7 for the `usage_disabled` and `usage_compiled_out` codes.

---

## 4. Config file reference

### 4.1 Resolution order (highest to lowest precedence)

1. Command-line flags (§1.2, per-command tables).
2. Environment variables, prefix `WHICH_MODEL_`, dotted keys uppercased with `_` separators (e.g. `WHICH_MODEL_STRATEGY_DEFAULT=score`, `WHICH_MODEL_PROVIDERS_CLAUDE_PRIORITY=5`).
3. Project config: `./.which-model/config.toml` relative to CWD, walked upward to the nearest git root (first found wins; does not merge multiple project configs).
4. User config: resolved path per §4.6.
5. Built-in defaults compiled into `internal/config`.

`which-model config show` prints the result AFTER this full merge; `which-model config show --json` includes a `_sources` map noting which layer each top-level key was resolved from, for debugging.

### 4.2 Full annotated `config.toml`

```toml
# ~/.config/which-model/config.toml (Linux/XDG) or
# ~/Library/Application Support/which-model/config.toml (macOS) — see §4.6

[usage]
# Level L1 of the usage toggle (§3.4, master plan §6). Three-state:
#   false  - hard off. which-model usage / which-model auth refuse with exit 2 naming this key.
#            Usage packages are linked but never invoked.
#   true   - required. If zero providers are enabled, which-model pick exits 2 rather
#            than silently degrading: the user asked for usage, so a
#            misconfiguration should be loud.
#   "auto" - DEFAULT. Enabled iff at least one [providers.<id>] has
#            enabled = true. Makes a fresh install a pure model picker with
#            zero credential access until a provider is opted in.
enabled = "auto"

[scoring]
# Pluggable score methods (Annex B §4.0). Defaults reproduce generate_scores.py
# byte-for-byte; both names are recorded in which-model explain evidence so any score
# traces to the method that produced it. Changing a default is a deliberate,
# documented migration with a differential report - never a silent swap.
normalizer = "minmax-linear"
aggregator = "weighted-arithmetic-mean"

[strategy]
default = "score"          # default --strategy when which-model pick omits it
default_profile = "balanced_implementation"
tier1_share = 100           # percent weight of tier-1 metrics vs tier-2 categories, mirrors rank_models.py --tier1-share/--tier2-share
tier2_share = 0

[bands]
direction = "spread"        # "spread": high consumption LOWERS weight | "drain": high consumption RAISES weight
gate_above_used_percent = 98 # candidates whose gating window exceeds this are dropped (exit 4 if ALL candidates are gated)

[[bands.tier]]
name = "low"
upper_used_percent = 25
weight = 1.00

[[bands.tier]]
name = "standard"
upper_used_percent = 50
weight = 0.85

[[bands.tier]]
name = "elevated"
upper_used_percent = 75
weight = 0.60

[[bands.tier]]
name = "critical"
upper_used_percent = 100
weight = 0.25

[catalog]
raw_csv_path = ""              # empty = resolved cache/repo default, §4.6
scores_csv_path = ""
provider_config_path = ""      # providers.toml-equivalent
benchmark_config_path = ""     # benchmarks.toml-equivalent
cache_ttl = "24h"
warn_on_stale_scores = true    # raw-CSV hash mismatch warning (§1.6 rule 2). The scores CSV
                                # records the content hash of the raw CSV it was derived from;
                                # a mismatch at read time emits a staleness warning, never a hard
                                # error. Mirrors the route-table staleness check, Annex B §7.2.

[catalog.publish]
enabled = true                 # false: `catalog workflow --write` emits nothing and removes an
                                # existing generated workflow (§2.3a, §4.2)
schedule = "0 6 * * *"         # cron; rendered LITERALLY into the generated workflow's
                                # on.schedule (GitHub Actions cannot read cron from a config
                                # file at trigger time (§2.3a, §4.2)
timezone = "Europe/London"     # schedule timezone recorded in the generated workflow
branches = ["main"]            # plural - one PR or direct push produced per branch, processed
                                # in listed order; a failure on one never aborts the others
mode = "pull-request"          # "pull-request" opens a PR per branch and enables auto-merge;
                                # "direct-push" pushes straight to the branch (escape hatch for
                                # repos without branch protection). Both MUST be specified.
auto_merge = true              # pull-request mode only: `gh pr merge --auto --<merge_method>`,
                                # which still respects branch protection rules
merge_method = "squash"        # "squash" | "merge" | "rebase"
commit_message = "chore(data): refresh available model scores"
pr_title = "chore(data): refresh available model scores"
pr_labels = ["data", "automated"]
run_tests = true               # fail-closed test gate before any commit, exactly as today
[output]
color = "auto"                # auto | always | never (--no-color forces never)
timestamps = "rfc3339"
identity_default = false      # mirrors global --show-identity default; config MAY opt a trusted local machine in by default, CLI flag always overrides

# One [providers.<id>] table per known provider id (Annex A registry).
# DEFAULT-DENY: unlisted providers default to enabled = false. A fresh install
# reads NO credential file, queries NO keychain item, opens NO browser cookie
# database, and shells out to NO provider CLI until a provider is explicitly
# enabled here. Enablement is opt-in, per provider, always. (This corrects an
# earlier draft of this annex which defaulted unlisted providers to true —
# that would have made a fresh install poll all 66 providers.)
[providers.claude]
enabled = true
priority = 10                 # used by --strategy priority; higher = preferred
weight = 1.0                  # provider-level multiplier in FinalScore = ModelScore * BandWeight * ProviderWeight
cache_ttl = "5m"
source_preference = ["oauth"] # ordered AuthSource override; empty = use Descriptor default chain
credential_path = ""          # override e.g. ~/.claude/credentials.json; empty = provider default

[providers.codex]
enabled = true
priority = 10
weight = 1.0
cache_ttl = "5m"
source_preference = ["oauth"]
trusted_fallback_origin = ""  # NEVER auto-applied; which-model auth login --trust-configured-origin is required per-invocation even
                               # when this is set — this key only pre-fills the flag's suggested value in interactive prompts,
                               # it does not itself grant trust (see §2.2 exact-origin semantics; no persisted auto-trust)

[providers.copilot]
enabled = true
priority = 5
weight = 1.0
cache_ttl = "5m"
```

### 4.3 `[bands]` semantics

`direction = "spread"` (default) means higher consumption reduces `BandWeight`, biasing `which-model pick` selection toward less-consumed providers as they approach their quota — see [master plan §5.2](./README.md) and §4.2 `[bands]` for the canonical tier table. `direction = "drain"` inverts this (useful for burst-down-first cost strategies); the tier `upper_used_percent`/`weight` pairs are interpreted identically, only the selection consequence differs (Annex B owns the exact `FinalScore` math for both directions).

### 4.4 Env var mapping examples

| env var | equivalent config key |
|---|---|
| `WHICH_MODEL_STRATEGY_DEFAULT` | `strategy.default` |
| `WHICH_MODEL_BANDS_GATE_ABOVE_USED_PERCENT` | `bands.gate_above_used_percent` |
| `WHICH_MODEL_PROVIDERS_CLAUDE_ENABLED` | `providers.claude.enabled` |
| `WHICH_MODEL_CATALOG_CACHE_TTL` | `catalog.cache_ttl` |

`[[bands.tier]]` array entries are NOT individually addressable via env vars (arrays of tables have no stable env-var key shape); override the whole `[bands]` section via `--band-config` or a project config instead.

### 4.5 XDG / macOS path resolution

| purpose | Linux (XDG) | macOS |
|---|---|---|
| user config | `$XDG_CONFIG_HOME/which-model/config.toml` (default `~/.config/which-model/config.toml`) | `~/Library/Application Support/which-model/config.toml` |
| cache (catalog CSVs, usage snapshot cache) | `$XDG_CACHE_HOME/which-model/` (default `~/.cache/which-model/`) | `~/Library/Caches/which-model/` |
| state (round-robin cursor, pick history log) | `$XDG_STATE_HOME/which-model/` (default `~/.local/state/which-model/`) | `~/Library/Application Support/which-model/state/` |

Resolution implemented via `github.com/adrg/xdg` gated behind `runtime.GOOS`: on `darwin`, `which-model` uses the macOS column above unconditionally (ignoring `XDG_*` env vars, matching platform convention); on other OSes it follows XDG env vars with the stated defaults.

Project-local config and evidence live under `./.which-model/` (walked upward to the nearest git root, §4.1).

**`~/.wdm/` MUST NOT be used for anything.** An earlier draft of this plan proposed `~/.wdm/` as this tool's app-private directory; that path is already owned by a different, unrelated tool on the author's machines. Nothing in `which-model` may read, write, or create it — no config, no cache, no state, no provider credential cache. The only correct dotted-directory names are `./.which-model/` (project) and the XDG/macOS locations above (user). This is called out explicitly because "`.wdm`" is the kind of shorthand that gets reintroduced by autocomplete or by someone reading the repo name and assuming.

### 4.6 Usage-disabled behaviour, per command

Every command's exact behaviour under each toggle level (§3.4). "L1" covers both `--no-usage` and `[usage] enabled = false`.

| Command | L1 (flag or config) | L2 (`-tags nousage`) |
|---|---|---|
| `which-model usage` | Exit `2`, `usage_disabled`, message naming the disabling source | **Not registered** — absent from `which-model --help` |
| `which-model auth *` | Exit `2`, `usage_disabled` | **Not registered** |
| `which-model pick` | Degrades to pure score ranking; `usage_enabled: false` + `usage_disabled_reason` in JSON; `--strategy least-used` exits `2`; `--require-live`/`--max-used-percent`/`--band-config` become no-ops that warn **once** on stderr if explicitly supplied | Same, with `usage_disabled_reason: "compiled_out"` |
| `which-model explain` | Evidence omits band, band weight, pressure, snapshot age and confidence; states the disabled reason instead | Same |
| `which-model routes refresh` | Unauthenticated sources only (models.dev, user-declared); credentialed live-provider source skipped with **one** warning; `Provenance` records the reduced set (Annex B §7.1a) | Same |
| `which-model routes verify` | Reports provenance counts so a credential-free table is visibly distinguishable | Same |
| `which-model serve` | Exit `2` — there is nothing to warm | **Not registered** |
| `which-model catalog *` (`refresh`, `benchmarks`, `scores`, `list`, `providers`, `workflow`) and the `--refresh-benchmarks`/`--refresh-scores` global flags | Unaffected. The catalog has no usage dependency (Annex B §0); Collect/Derive read and write only catalog CSVs and config, never usage snapshots or credentials | Unaffected |
| `which-model config show` | Reports the resolved effective usage state and which layer decided it | Reports `compiled_out` |
| `which-model version` | `usage: enabled` | `usage: compiled-out` |

No-op warnings fire **once per invocation, not per candidate**. A per-candidate warning across a 39-row catalogue would bury the signal and make the default install unusable.

Worked examples:

```
$ which-model pick --profile balanced_implementation --no-usage --json | jq '{usage_enabled, usage_disabled_reason, model: .recommendation.route.model}'
{
  "usage_enabled": false,
  "usage_disabled_reason": "flag",
  "model": "Claude Opus 4.8"
}

$ which-model pick --strategy least-used --no-usage; echo $?
which-model pick: [arguments] least-used requires usage data; usage is disabled by --no-usage
2

$ which-model usage --all          # with [usage] enabled = false
which-model usage: [usage_disabled] usage is disabled by [usage] enabled = false in ~/.config/which-model/config.toml
$ echo $?
2

$ which-model version              # nousage build
which-model 0.1.0 (darwin/arm64)
usage: compiled-out
```

### 4.7 Build variants

```sh
go build -o which-model ./cmd/which-model                  # default: usage subsystem included
go build -tags nousage -o which-model ./cmd/which-model    # usage subsystem not linked
```

Under `nousage`, `internal/usage/**` is excluded by build tag and replaced by a stub returning `ErrUsageCompiledOut` (Annex A §1a.2). The provider registry is empty **by construction** — no `init()` self-registration runs — rather than filtered at runtime, and `which-model usage`/`which-model auth` are never added to the command tree.

Auditable verification (Annex A §1a.3):

```sh
strings which-model | grep -c 'chatgpt.com/backend-api\|api.anthropic.com\|copilot_internal'  # expect 0
go version -m which-model | grep -E 'kooky|go-keyring'                                        # expect no matches
which-model version                                                                            # expect "usage: compiled-out"
```

CI MUST build and test both variants on every change. Without the `nousage` job, the catalog's usage-independence (Annex B §0) rots the first time someone adds a convenience import.

---

## 5. Migration table

| legacy invocation | `which-model` equivalent | behavior changes |
|---|---|---|
| `node scripts/claude-usage.mjs` | `which-model usage claude` | Text report format unchanged (`formatUsageReport`, §2.1). `--json` is new — legacy script had no machine-readable mode. Legacy: any argument at all is an error (`arguments`, exit 1); `which-model usage claude` accepts the global flag set with exit `2` on genuinely unknown flags. |
| `node scripts/codex-usage.mjs [--trust-configured-origin <origin>]` | `which-model usage codex` for the read path; `which-model auth login codex --trust-configured-origin <origin>` for the opt-in fallback trust act | Split across two commands: `which-model usage` never itself prompts for trust — a gated 404-class primary failure surfaces as `Snapshot.Failure{code:"fallback_unavailable"}` in `usage`, and the operator re-runs with `which-model auth login codex --trust-configured-origin <origin>` (or sets `--trust-configured-origin` directly on a future `which-model usage codex --trust-configured-origin <origin>` if Annex A wires it as a usage-time flag too — kept as a `usage`-level flag identical to the legacy script's single-command shape is also valid; either way the exact-origin validation rules (§2.2) are unchanged). Legacy exit `1`/report-on-stdout unchanged in spirit, mapped to `which-model`'s finer-grained exit codes (`5` for auth, `1` for other runtime failures) instead of the flat `1`. |
| `node scripts/copilot-usage.mjs [--login] [--show-identity]` | `which-model usage copilot [--show-identity]` for read; `which-model auth login copilot` for the device flow | Legacy combined read+login in one script/one invocation (`--login` triggered inline). `which-model` splits these: `which-model usage copilot` alone returns exit `5` (`login_required`) if no credential exists, rather than silently blocking on an interactive device flow; login is a deliberate separate command, consistent with §2.2's unattended-refusal rule (a scripted/agent `which-model usage` call must never block on human interaction). `--show-identity` semantics (opt-in identity string) are unchanged. |
| `python3 .agents/skills/meta-orchestration-model-selection/scripts/rank_models.py --scores PATH --profile P --top N [--pretty] [--available PATH ...] [--available-identity M\|R ...] [--weights-json J \| --tier1-weight/--tier2-weight ...]` | `which-model pick --profile P --top N [--json for machine output, default text otherwise] --available PATH ... --identity M\|R ... [--weights-json J \| --tier1-weight/--tier2-weight ...]` | `--scores PATH` is removed as a required flag — `which-model pick` resolves the scores CSV from `[catalog].scores_csv_path`/cache by default; pass `--band-config`/config overrides instead of a raw path when needed for scripting/tests. `--pretty` is removed — `which-model`'s default JSON (`--json` without `--pretty`) is intentionally NOT pretty-printed (agent-consumption default); use `jq` or a shell pipe for human-readable JSON. `rank_models.py` had no usage-provider awareness at all — `which-model pick` additionally applies band weighting and provider gating by default; pass `--strategy score` with no `--provider` filtering and a full-100% `[bands]` gate to approximate the legacy pure-ranking behavior exactly. `--tier1-share`/`--tier2-share` moved to `[strategy]` config defaults, overridable per-profile in the profile JSON (Annex B), not top-level CLI flags. |
| `python3 .github/workflows/update_available_model_data/update_raw_values.py [--output PATH] [--env-file PATH] [--benchmark-config PATH] [--provider-config PATH] [--provider NAME ...] [--list-providers] [--add SOURCE ...]` | `which-model catalog benchmarks [--out PATH] [--provider-config PATH] [--benchmarks PATH] [--provider ID ...] [--add aa_page]` (Collect stage only, §1.6 §2.3) - equivalently, the global `--refresh-benchmarks` flag on `catalog`/`pick` | `--env-file` is removed as a *flag*, but the `.env` fallback itself is **retained** — Annex B §2.3 owns the exact chain: (1) `ARTIFICIAL_ANALYSIS_API` from the environment, (2) else `<repo root>/.env` parsed with the duplicate-key rejection, (3) else a hard error (mirrored by `catalog benchmarks`'s exit `2`, §2.3). The canonical env var stays **unprefixed** (`ARTIFICIAL_ANALYSIS_API`, not `WHICH_MODEL_`-prefixed) so the existing GitHub Actions secret keeps working unchanged (§8 of Annex B); `WHICH_MODEL_ARTIFICIAL_ANALYSIS_API` is accepted as an optional override for consistency with the rest of the env surface, but is never required. Only the ability to point at an *arbitrary* env file path is dropped, since the fixed repo-root location is the only one the pipeline ever used. `--list-providers` becomes the separate `which-model catalog providers` subcommand rather than a flag. Transactional write behavior (temp file, fsync, compare-and-swap, `.bak`, atomic rename) is preserved exactly (§2.3). Legacy `update_raw_values.py` never derived scores itself; `which-model catalog refresh`/`--refresh` is the new combined entry point for callers that previously chained both scripts by hand. |
| `python3 .github/workflows/update_available_model_data/generate_scores.py [--input PATH] [--output PATH] [--benchmark-config PATH]` | `which-model catalog scores [--in PATH] [--out PATH] [--benchmarks PATH]` (Derive stage, §1.6 §2.3) - equivalently, the global `--refresh-scores` flag | Purely a rename/regroup under the `catalog` subcommand; behavior (pure function of raw CSV + benchmark config, wholesale regeneration, `ROUND_HALF_UP` to integer 0–100 scores) is unchanged — Annex B owns the exact math port. Unlike the legacy script, `catalog scores`/`--refresh-scores` is explicitly documented as safe under `--offline` (§1.6 rule 3) since it was always a pure function; this annex just makes that guarantee load-bearing. |
| `.github/workflows/update_available_model_data/*.yml` (hand-maintained scheduled-refresh workflow, both legacy scripts chained with commit-if-changed) | `which-model catalog workflow --write`, generating `.github/workflows/refresh-model-data.yml` from `[catalog.publish]` (§2.3a, §4.2) | The workflow file is no longer hand-maintained: it is generated from config and re-generated with `catalog workflow --write` whenever `[catalog.publish]` changes, with `catalog workflow --check` as a CI lint catching drift (§2.3a). Schedule, target branches, PR-vs-direct-push mode, auto-merge, and commit/PR message text move from YAML literals into `[catalog.publish]` keys; the commit-only-if-changed `git diff --cached --quiet` gate and the fail-closed test-before-commit step are preserved verbatim (Annex B §8). Multi-branch publishing (`branches = [...]`) and the pull-request-with-auto-merge mode are new — the legacy workflow pushed directly to a single branch only. |
| `python3 .github/workflows/update_available_model_data/get_provider_models.py [--provider-config PATH] [--provider NAME ...]` | `which-model catalog providers [--provider-config PATH] [--provider ID ...]` | Legacy printed the raw per-provider model JSON directly to stdout as its entire purpose; `which-model catalog providers` is now a read-only summary view (§2.3 example) by default, with the full per-model JSON available via `--json` (equivalent payload shape, `{provider: [{"id":...,"name":...,"reasoning":[...]}]}}`, preserved for scripting compatibility). |
| `python3 .../rank_models.py …` **with no usage awareness wanted at all** | `which-model pick --no-usage …` (or `[usage] enabled = false`, or a `-tags nousage` binary) | The direct drop-in replacement for the legacy ranker. Degraded mode is *exactly* `rank_models.py` behaviour: `BandWeight = 1.0`, no provider polling, no credential access, pure profile-based score ranking — and it MUST reproduce `rank_models.py` output byte-for-byte on the same scores CSV (Annex B §9.5). A team that wants only the model-ranking half of `which-model` never has to configure a provider, and with `-tags nousage` gets a binary that cannot read credentials at all. |

All seven legacy entry points: text output shape is preserved where a direct human-facing equivalent exists (`usage`); JSON becomes the standard, stable, `--schema`-documented agent contract everywhere it previously did not exist (`rank_models.py` had ad hoc JSON with no schema; the collector scripts printed raw upstream-shaped JSON with no stability guarantee). No legacy invocation silently changes its exit-code meaning: `0`/non-`0` boundaries are preserved, and `which-model`'s finer-grained non-zero codes are all still non-zero, so any caller checking only `$? -eq 0` needs no changes.

---

## 6. Shell completion and man pages

- **Completion:** generated by cobra's built-in `completion` machinery, exposed as a hidden `which-model completion {bash|zsh|fish|powershell}` command (standard cobra convention, not separately flagged above since it is infrastructure rather than a domain command). Installation is documented, not automated by `which-model` itself: `which-model completion zsh > "${fpath[1]}/_which-model"` (zsh), `which-model completion bash > /etc/bash_completion.d/which-model` or `~/.local/share/bash-completion/completions/which-model` (bash), `which-model completion fish > ~/.config/fish/completions/which-model.fish` (fish). Packaging (Homebrew formula, `.deb`/`.rpm`) installs these files at package-install time by invoking `which-model completion` against the just-built binary in the packaging pipeline, not at first-run.
- **Man pages:** generated at release-build time via `github.com/spf13/cobra/doc`'s `GenManTree`, invoked from a `make man` / release CI step (not a runtime `which-model` subcommand — man-page generation reflects the exact flag/subcommand tree of the release binary and is a build-time artifact, avoiding runtime doc-generation drift). Output installed to the standard `MANPATH` location by the same packaging step as completions (`/usr/share/man/man1/which-model.1` and one page per subcommand, `which-model-usage.1`, `which-model-pick.1`, etc., per cobra's default per-command page convention).
