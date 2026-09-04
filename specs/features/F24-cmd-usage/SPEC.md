---
kind: feature-spec
version: "1.0"
feature: F24-cmd-usage
project: which-model
---

# F24 — cmd-usage: SPEC

## 1. Purpose

`which-model usage [provider...] [--all]` fetches and reports usage `Snapshot`s for named providers (or every enabled provider with `--all`), generalizing the prototype's three scripts (`claude-usage.mjs`, `codex-usage.mjs`, `copilot-usage.mjs`) to N providers via the F11 registry and F14 fan-out fetch. It is the CLI surface the `provider-usage` skill and quota-guard hook call (`which-model usage --all --json --band-at-or-above critical`), so its JSON shape and exit codes are a machine contract, not presentation. The command never prints credential material: account identity is opt-in via the global `--show-identity` flag, and tokens never appear under any flag (canary-tested).

## 2. Behaviour

1. **Command shape.** `which-model usage [provider...] [--all]` with per-command flags `--all`, `--source`, and `--band-at-or-above`, and global flags `--json`, `--max-age`, `--refresh-usage`, `--show-identity`, `--offline`, `--no-usage`, `--timeout` consumed via F22's `Global` struct. Positional args are provider `Descriptor.ID`s from the F11 registry. `--all` is mutually exclusive with positional args (exit 2 if both given). No providers and no `--all` is an argument error (exit 2). (Source: `docs/plan/annex-d-cli-reference.md` §2.1, §1.2; F22 root flag set.)

2. **Wiring.** The command is self-registered per the F22 command-wiring contract: `pkg/whichmodel/usage_cmd.go` carries `func init() { register(NewUsageCmd) }` and `func NewUsageCmd() *cobra.Command`; F22's `registeredCommands()` places it at display position "usage". The file is build-tagged `//go:build !nousage`, so a `nousage` binary does not register the command at all (annex-d §4.6 L2); the logic file `pkg/whichmodel/usage.go` compiles under both tags because F21's stubs stand in for `internal/usage/**`. (Source: F22 command-wiring contract; annex-d §4.6.)

3. **Owned flag `--all`.** Report every provider with `enabled = true` in the resolved config's `[providers.<id>]` tables (F01), instead of requiring positional names. (Source: annex-d §2.1.)

4. **Owned flag `--source <oauth|api|cli|web|local|cache>`.** Force a specific `Source` instead of the descriptor's ordered fallback chain. Valid values are exactly `oauth`, `api`, `cli`, `web`, `local`, `cache`; the default is unset (= auto fallback chain). An explicit value the provider's descriptor does not declare is an argument error (exit 2, message names the provider and its valid sources). `--source cache` means "report from the usage cache only": a provider with no cache entry yields an inline `Snapshot.Failure{code: "fallback_unavailable"}` and counts as failed in the exit logic, mirroring `--offline` cache-miss semantics. (Source: annex-d §2.1; Decisions D-1, D-7.)

5. **Global flags wired into the fetch.** `--refresh-usage` forces a live fetch ignoring cache TTLs; `--max-age <duration>` treats cached data older than the duration as stale (overrides the descriptor/config TTL for this invocation); `--timeout <duration>` is the per-request timeout; `--offline` forbids network (cache-only). All are passed through to F14 `FetchAll` as `FetchAllOptions`. (Source: annex-d §1.2, §1.6 rule 5; F24 CONTRACTS §8.2.)

6. **Provider argument validation.** Every positional provider is validated against the F11 registry. An unknown id is an argument error: exit 2, stderr failure line `which-model usage: [arguments] unknown provider "x"; valid providers: <comma-separated registry IDs>`. (Source: annex-d §1.4, §2.1; Decisions D-6.)

7. **Text output.** One block per provider in request order, format ported verbatim from the prototype's `formatUsageReport` (`docs/plan/research/usage-allowance-checks-spec.md` core.mjs:230-247): header line `"<DisplayName> usage allowance"`, then one line per window `"- <label>: <detail1>; <detail2>; ..."`. Detail field order per window: `used_percent` → `<n>% used`; `100 - used_percent` → `<n>% available` (only when `UsedPercent` is set and `Remaining` is not); `remaining` → `<n> remaining`; `limit` → `<n> total`; `unlimited: true` renders the single detail `unlimited` and suppresses all percent/count details. `resets_at` is always last: `resets <RFC3339>`. Numbers render with a trailing `.0` stripped. (Source: annex-d §2.1; research spec core.mjs:230-247.)

8. **Reset hints.** When `ResetsAt` is absent but `ResetHint` is non-empty, the last detail is the hint verbatim when it already starts with "resets", else `resets <hint>`. When neither is present, no reset detail is rendered. Never render both. (Source: global CONTRACTS §1.4 `Window.ResetHint`; Decisions D-8.)

9. **Identity redaction.** By default `Snapshot.Account` is omitted from both text and JSON output entirely — not masked. With the global `--show-identity` flag, JSON includes `account` and text appends `- account: <account>` as the last line of the provider block. No output under any flag combination contains credential/token material (canary-tested; global SPEC §6.5, §6.7). The prototype's always-on `GitHub identity verified.` line is not carried forward. (Source: annex-d §1.2, §2.1; Decisions D-4.)

10. **JSON output (`--json`).** A single JSON document on stdout: root object `{schema_version: "2.0", usage_enabled: true, snapshots: [Snapshot...], last_verified: {<provider>: <RFC3339>}}` per F24 CONTRACTS §6. In normal mode, `snapshots` are canonical `usage.Snapshot` values (global CONTRACTS §1.5) in request order, one per requested provider (failed providers carry `error` inline). With `--band-at-or-above`, the command keeps only snapshots meeting behaviour 14 and removes excluded providers from `last_verified`. `last_verified` maps provider id → timestamp of that provider's last successful live verification, as returned by F14 `FetchAll`; omitted entirely when empty. (Source: global CONTRACTS §6; annex-c §4.1 + §4.6; Decisions D-3.)

11. **Exit codes.** `0` when at least one requested provider succeeded (individual failures stay inline as `Snapshot.Failure`, never a nonzero exit); `1` when every requested provider failed and none of the failures is auth-class; `5` when every requested provider failed and at least one failure is auth-class (exit-5 codes: `unauthorized`, `login_required`, `expired_credential`, `credential_file`, `credential_json`, `unsafe_credential`, `access_denied`, `device_expired`, `cookie_unavailable`, `signing_failed`); `2` for argument errors and usage-disabled refusal. (Source: annex-d §2.1; global CONTRACTS §1.6; Decisions D-2.)

12. **Usage-disabled behaviour.** Under L0 (`--no-usage` flag) or L1 (`[usage] enabled = false` in config), `which-model usage` exits 2 with stderr `which-model usage: [usage_disabled] usage is disabled by <--no-usage|[usage] enabled = false in <config path>>`. F24 resolves this inline via F01's `UnmarshalKey("usage.enabled", ...)` and the global flag (F21 is not in F24's dependency graph; its M4 canonical resolution is unavailable at F24's milestone). Under L2 (`-tags nousage`) the command is not registered (behaviour 2). (Source: annex-d §4.6, §3.4; Decisions D-5.)

13. **stdout/stderr discipline.** stdout carries only the report (text) or the single JSON document (`--json`). On nonzero exit in text mode stdout is empty; in `--json` mode F22 renders the error document `{"schema_version": "2.0", "error": {"code", "message"}}` on stdout. All diagnostics, warnings (e.g. broad credential-permission warnings surfaced by F12/F14), and the final failure line go to stderr. Failure-line format: `which-model usage: [<code>] <message>` on one stderr line (F22 via F03 `output.WriteFailure`); `<code>` is `arguments` for argument errors, `usage_disabled` for the disabled refusal, or a provider `Failure.Code` for an all-failed runtime exit. Messages are sanitized (never credential material). Exit signalling uses the F22 exit contract (F24 CONTRACTS §8.1): RunE returns an error implementing `interface{ ExitCode() int; ExitCodeStr() string }`. (Source: annex-d §1.3; global SPEC §6.5; Decisions D-10.)

14. **Owned flag `--band-at-or-above <tier>`.** Resolve `<tier>` against the configured F19 `[bands]` ladder and retain only snapshots whose maximum known pressure across all windows evaluates to that tier or a later tier. A pressure at the hard gate counts as above every tier; unknown pressure does not match. Filtering happens only after the normal all-failed exit classification, so provider failures remain fail-open inputs for F29. Unknown tier names are argument errors before fetch: `[arguments] invalid --band-at-or-above "<tier>"; valid: <ordered configured tier names>`. The default is unset and preserves the unfiltered F24 report.

## 3. Error behaviour

| Condition | Exit | stderr |
|---|---|---|
| Unknown provider id | 2 | `[arguments] unknown provider "x"; valid providers: claude, codex, copilot, ...` |
| `--all` + positional providers | 2 | `[arguments] --all and provider arguments are mutually exclusive` |
| No providers and no `--all` | 2 | `[arguments] no providers requested; name providers or pass --all` |
| `--source` value outside `{oauth,api,cli,web,local,cache}` | 2 | `[arguments] invalid --source "x"; valid: oauth, api, cli, web, local, cache` |
| `--source` value the provider does not declare | 2 | `[arguments] provider "x" has no <src> source; valid sources: <...>` |
| Unknown `--band-at-or-above` tier | 2 | `[arguments] invalid --band-at-or-above "<tier>"; valid: <ordered configured tier names>` |
| Usage disabled (L0 flag / L1 config) | 2 | `[usage_disabled] usage is disabled by <source>` |
| Zero successes, no auth-class failures | 1 | one failure line per provider, each `[<code>] <message>` |
| Zero successes, ≥1 auth-class failure | 5 | same, auth-class lines included |
| ≥1 success | 0 | failures reported inline in output, not on stderr |
| Malformed `--max-age`/`--timeout` duration | 2 | `[arguments] invalid duration "x"` (validated by F22's flag binding, not F24) |

## 4. Decisions

| Decision | Value | Rationale |
|---|---|---|
| D-1 | `--source` accepts exactly `oauth\|api\|cli\|web\|local\|cache`; the default is unset (auto); an explicit `--source auto` is rejected as an unknown value | The assignment contract lists the six values; `auto` is the zero state, and the failure line listing valid values stays truthful |
| D-2 | Exit 5 when zero successes and *any* single auth-class failure present | The assignment's decided rule ("any single 5-code failure with zero successes → 5") extends annex-d §2.1's "EVERY provider failed with auth-class" to mixed auth/non-auth all-failed sets — login is the actionable fix |
| D-3 | `last_verified` is a root-level map `{provider → RFC3339}` beside `snapshots` | annex-c §4.1 declares `additionalProperties: false` on `Snapshot`, so per-snapshot fields are forbidden; a new optional root field is a schema-compatible MINOR addition (§4.5) |
| D-4 | Text mode renders nothing about identity by default; with `--show-identity` appends `- account: <account>` | The prototype's always-on "GitHub identity verified." line conflicts with global SPEC §6.7 (identity display opt-in only) |
| D-5 | F24 resolves the usage-disabled state inline (F01 `UnmarshalKey("usage.enabled")` + global `--no-usage` flag) | F24 is M2 and its dependency graph (F14, F22) excludes F21 (M4); the L1 refusal behaviour is identical either way |
| D-6 | Unknown-provider failure line lists valid registry IDs in registry order | Agents need the exact allow-list to fix the invocation; matches the prototype's minimal explicit allow-list posture |
| D-7 | `--source cache` cache-miss → inline `Failure{code: "fallback_unavailable"}`; counts as failed in exit logic | Mirrors `--offline` cache-miss semantics (annex-d §1.2) so a forced-cache run cannot fabricate data |
| D-8 | Reset detail: `ResetsAt` → `resets <RFC3339>`; else non-empty `ResetHint` → hint verbatim, prefixed with `resets ` only when the hint does not already start with it; else none | `ResetHint` is the human-readable form provider adapters produce; RFC3339 is the machine form; never both |
| D-9 | Provider header in text mode is the registry display name (`Descriptor.Name`), falling back to the provider id | The prototype renders "Claude", "Codex", "GitHub Copilot"; `Snapshot.Provider` carries only the id |
| D-10 | F24 RunE returns F22's `UsageError` (argument errors) or `CodedError{Code, Message}` (everything else); F22's `ExecuteArgs` maps the returned error via `ExitCodeFor` and renders exactly one failure line via F03 `output.WriteFailure` (stderr; JSON error document on stdout in `--json` mode). F24 never calls `os.Exit` and never prints failure lines itself | F22 owns exit-code mapping and the failure-line format; one render point guarantees the fixed `[<code>] <message>` shape |
| D-11 | `--band-at-or-above` evaluates the maximum known pressure across all snapshot windows, then uses the configured F19 tier order; hard-gated pressure always matches | SessionStart quota guard has no route context but must honor custom band thresholds and return provider-level results |

## 5. Out of scope

- `--include-cost`, `--window <id>`, `--fail-on-gated` (annex-d §2.1 flags not in this feature's contract; `--fail-on-gated`'s exit 4 is therefore not produced by F24).
- `which-model schema <command>` emission (annex-c §4.4; schema subcommand is a later CLI feature).
- The `--login` inline-auth flag on `usage` (belongs to the auth feature family, annex-d §2.2).
- Any provider fetch logic itself: F24 only wires flags and renders; fetching is F14, credentials are F12.
- `which-model serve`, hooks, skills (other features).

## Forced-source execution correction (#184)

After #28 validates `--source`, F14 enforces it for the resolved credential, including managed fallback, before native fetch. A mismatch yields sanitized `login_required` without invoking the provider. A fresh online cache is eligible only when its original source matches the forced source; unknown/mismatched provenance proceeds to the matching live path. Empty source preserves automatic precedence; `--source cache` and offline mode remain cache-only. Regression coverage is pinned in F14 CONTRACTS: managed OAuth/API mismatch, matching/auto fallback, and matching/mismatched/unknown stored provenance. Human codeowner review is required before merge.
