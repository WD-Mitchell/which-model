---
kind: feature-spec
feature: F29-agent-hooks
version: "1.0"
project: which-model
---

# F29 — Agent Hooks: Spec

## Purpose

Wire `which-model` into AI-agent dispatch lifecycles (Claude Code and the generic harness convention from `docs/plan/annex-c-agent-integration.md` §3) through four hooks: a session-start usage cache warm, a pre-dispatch model resolution gate, a post-dispatch evidence recorder, and a quota-guard advisory. Each hook is a `which-model hooks run <hook>` invocation that executes the annex-c underlying command in-process and wraps its output in a machine-readable decision envelope (`decision`/`reason`/`hookSpecificOutput`). Installation is merge-safe (never clobbers foreign hook config), variant-aware (usage-disabled installs only the two usage-independent hooks, annex-c §3.5), and removable without trace.

## Behaviour

1. **Command surface** (DECISION A wiring; `docs/plan/annex-c-agent-integration.md` §3, `docs/plan/annex-d-cli-reference.md` §2.10):
   - `which-model hooks install [--target claude|generic] [--usage|--no-usage] [--repo PATH]`
   - `which-model hooks remove  [--target claude|generic] [--repo PATH]`
   - `which-model hooks run <hook> [args...]`
   - `--target` default `claude`. `--usage`/`--no-usage` force a variant; both or neither-absent is auto-detected at install time (behaviour 9). `--repo` overrides repo-root discovery (same rule as F28's `--repo`).

2. **Hook inventory** — exactly four hooks, per annex-c §3.1–§3.4 (underlying commands verbatim from the annex tables; `hooks run` replaces the raw invocation in the installed config):

   | id | annex-c | claude event / matcher | timeout (s) | underlying command (built by `hooks run`) |
   |---|---|---|---|---|
   | `usage-refresh` | §3.1 session-start cache warm | `SessionStart` / `*` | 5 | `which-model usage --all --json --quiet --refresh-usage --timeout 5s` |
   | `quota-guard` | §3.4 quota-guard advisory | `SessionStart` / `*` | 5 | `which-model usage --all --json --band-at-or-above critical --quiet` |
   | `spawn-gate` | §3.2 pre-dispatch model resolution | `PreToolUse` / `Task` | 8 | `which-model pick --profile "${WHICH_MODEL_TASK_PROFILE:-balanced_implementation}" --strategy priority --json` |
   | `model-audit` | §3.3 post-dispatch evidence recording | `PostToolUse` / `Task` | 5 | `which-model explain --last --json`; explicit passthrough `--pick-id <history ULID>` selects a record instead |

   `hooks run <hook> [args...]` appends any extra args AFTER the defaults, so an installed variant can override (`--no-usage --profile balanced_implementation --quiet`).

3. **Decision protocol** — every successful `hooks run` prints exactly one JSON object on stdout (nothing else), the Claude Code hook envelope:
   ```json
   {"decision":"approve","reason":"...","hookSpecificOutput":{...}}
   ```
   `decision` is `"approve"` or `"block"`; `reason` a short human string; `hookSpecificOutput` a JSON object carrying hook payload (empty `{}` when the hook injects nothing). Envelope is written compact (no indentation) with a single trailing `\n`. Nothing is ever printed to stdout besides the envelope; diagnostics go to stderr.

4. **`hooks run` execution model** — the hook executes its underlying command in-process via F22's `pkg/whichmodel.ExecuteCommand(args []string, stdout, stderr io.Writer) int` (no subprocess; annex-c §3 commands remain the installed command semantics). Non-empty stdin is an object containing host event context, never a replacement command result. The underlying runner always executes once; tests inject its stdout through `Runner`. Empty/whitespace input is allowed; malformed JSON, arrays, null and scalars are rejected. Nested CLI execution builds fresh command objects and restores the outer flags and output streams so the envelope reaches the host. Exit codes:
   - `0` on every successfully-formed run, INCLUDING underlying command failure (fail-open, behaviour 6) — unless the failure mode demands silence (behaviour 6).
   - `2` with a stderr message for: unknown hook name (`which-model hooks run nonsense`), or a non-empty stdin that is not a valid JSON object.
   - No other exit code is ever produced by `hooks run`.

5. **Underlying exit-code interpretation** (global exit codes, `specs/global/SPEC.md` §5; annex-c §4.7):
   - `spawn-gate` and `model-audit` treat the underlying `pick`/`explain` exit code as the authority: `0` = parse the JSON; `4` (all band-gated) = block path; `1`/`2`/`3`/`5` = fail-open approve.
   - `usage-refresh` and `quota-guard` treat ANY non-zero underlying exit as fail-open silence.
   - Hook-level timeouts are enforced by the harness (the `timeout` field in the installed config), not by `hooks run`; underlying network work is bounded by the command's own `--timeout` flags (annex-c §3.1–§3.4).

6. **Fail-open semantics** (annex-c §3 preamble, §3.1–§3.4 "Failure posture"):
   - `usage-refresh` / `quota-guard` (SessionStart, inject-only hooks): underlying non-zero exit or unparseable output → print NOTHING to stdout (empty output = nothing injected), exit `0`. A hook that emits nothing never blocks or annotates the session.
   - `spawn-gate` / `model-audit` (dispatch-boundary hooks): underlying non-zero exit (`1`/`2`/`3`/`5`) or unparseable output → emit `{"decision":"approve","reason":"fail-open: <hook> underlying command exited <N>","hookSpecificOutput":{}}`, exit `0`. The dispatch proceeds on the harness's own defaults; the hook never aborts a dispatch.
   - An envelope is NEVER emitted for the underlying `4` path except the documented block envelope (behaviour 7).

7. **Per-hook payloads**:
   - `usage-refresh` (annex-c §3.1): underlying success → `{"decision":"approve","reason":"usage cache refreshed","hookSpecificOutput":{}}`. Injects nothing; it exists purely to warm `internal/usage/cache/cache.go`.
   - `quota-guard` (annex-c §3.4 + assignment deviation recorded in Decisions): underlying success → parse the `usage --all --json --band-at-or-above critical --quiet` output; if it lists at least one provider:
     ```json
     {"decision":"block","reason":"quota guard: 2 provider(s) at or above critical band","hookSpecificOutput":{"critical_providers":["claude","codex"]}}
     ```
     `critical_providers` = the provider names from the output's snapshots (field `provider`), in output order, de-duplicated. If the output lists no providers: `{"decision":"approve","reason":"no providers at or above critical band","hookSpecificOutput":{}}`.
   - `spawn-gate` (annex-c §3.2): underlying `pick` exit `0` → `{"decision":"approve","reason":"dispatch approved: <candidates[0].candidate_id>","hookSpecificOutput":{"candidate":{...}}}` where `candidate` is the FULL first candidate object from the pick JSON (annex-c §4.2), verbatim. Underlying exit `4` → `{"decision":"block","reason":"all eligible providers band-gated: <reason_code==\"band_gated\" exclusions, comma-joined>","hookSpecificOutput":{"excluded_candidates":[...]}}` where `excluded_candidates` is the full `excluded_candidates` array from the pick JSON, verbatim.
   - `model-audit` (annex-c §3.3): underlying `explain` exit `0` → `{"decision":"approve","reason":"dispatch evidence recorded","hookSpecificOutput":{"evidence_logged":"<repoRoot>/.which-model/evidence.jsonl","mismatch":false}}`; decodes only the documented F26 fields and appends the sanitized explain JSON object (annex-c §4.3: `schema_version`, `candidate`, `evidence`) as ONE line to `<repoRoot>/.which-model/evidence.jsonl` (`O_APPEND|O_CREATE|O_WRONLY`, mode `0600`). If env `WHICH_MODEL_DISPATCHED_MODEL` is set AND differs from the model-id portion of F26's `candidate` string (`provider:model_id`, split only at the first colon), append one line `{"ts":"<RFC3339 UTC>","dispatched_model":"<env>","route_model_id":"<model_id>","evidence":{...full explain object...}}` to `<repoRoot>/.which-model/audit-mismatches.jsonl` (same append semantics) and set `"mismatch":true` in the envelope.

`WHICH_MODEL_CANDIDATE_ID`, when set, is an exact correlation check against the returned candidate string, not a history ULID or positional CLI argument. A mismatched candidate, missing evidence, unsupported schema, or invalid candidate string fails open without writing evidence. Explicit `--pick-id` suppresses the default `--last`.

8. **Evidence files** — both files live under `<repoRoot>/.which-model/` (repo root resolved exactly as F28's `internal/skills.RepoRoot()`, incl. `--repo` override). Appends never rewrite existing content; a malformed explain output is treated as underlying failure (fail-open, no append). Evidence lines are sanitized per annex-c §5: only the documented JSON fields, never credential material (canary rule, behaviour 12).

9. **Variant selection at install time** (annex-c §3.5): the installer detects the effective usage state ONCE, via F21's `usage.Enabled(cfg *config.Config) (config.UsageEnabled, string)` (from the `internal/usage/toggle` package), unless `--usage`/`--no-usage` forces it (`--usage`+`--no-usage` together → exit 2). Usage-ENABLED variant installs all four hooks; usage-DISABLED variant installs only `spawn-gate` and `model-audit` — never installed-and-failing (annex-c §3.5):
   - variant A: `usage-refresh` SessionStart/* 5s, `quota-guard` SessionStart/* 5s, `spawn-gate` PreToolUse/Task 8s, `model-audit` PostToolUse/Task 5s.
   - variant B: `spawn-gate` on `UserPromptSubmit`/* with timeout 10 and command `which-model hooks run spawn-gate --no-usage --profile balanced_implementation --quiet`; `model-audit` on PostToolUse/Task 5s with command `which-model hooks run model-audit --last` (both per annex-c §3.5's usage-disabled snippets).

10. **Install merge strategy**:
    - `--target claude` writes repo `.claude/settings.json` as follows. A sidecar manifest `.claude/which-model-hooks.json` records ownership: `{"version":1,"created_settings":<bool>,"hooks":[{"id":"usage-refresh","event":"SessionStart","matcher":"*","timeout":5,"command":"which-model hooks run usage-refresh"}]}` (one entry per installed hook, variant B has two). Install decodes the existing `settings.json` into a generic map, replaces/appends ONLY the entries whose (event, matcher, command) appear in the manifest, never touches any other key or hook entry (foreign hooks preserved byte-for-byte as far as JSON round-trip allows), and re-encodes with `json.MarshalIndent` 2-space + trailing `\n`. A pre-existing `settings.json` that is not valid JSON → error (exit 1). Missing file → created with exactly the owned hooks; `created_settings: true`.
    - `--target generic` writes/patches repo `agents/hooks.toml` inside a marker block: `# === which-model managed hooks (do not edit) ===` … `# === end which-model managed hooks ===`. Existing content outside the markers is preserved byte-for-byte (no TOML parse/rewrite). Missing file → created with the marker block; foreign markers already present are replaced, never duplicated.
    - Repeated install of the same variant is idempotent (file unchanged byte-for-byte on the second run; manifest unchanged).

11. **Remove semantics** (annex-c §2.4 clean-cutover spirit): `which-model hooks remove` deletes ONLY owned entries — for claude, the manifest-listed (event, matcher, command) triples plus the manifest itself; for generic, the marker block content. Foreign hooks and foreign file content are never touched. If the claude `settings.json` becomes `{}` after removal AND `created_settings` was true, the file is deleted; otherwise the remaining (foreign) content is kept. If the generic file becomes empty/whitespace-only after marker removal, it is deleted. Removing when nothing is installed is a no-op success (exit 0).

12. **Security invariants** (global SPEC §6; annex-c §7 anti-patterns):
    - The envelope contains ONLY the documented fields — never raw underlying stdout, never credential/device/token material, never provider bodies.
    - The canary-token test requirement applies to every usage-touching hook: a fixture containing a canary string in an unrelated field MUST NOT appear anywhere in the envelope output or in evidence files.
    - Evidence files are created `0600`; appends are single-write `O_APPEND` (no partial-line interleaving).
    - Hooks never follow redirects, never widen accepted origins, and never retry failed auth flows (delegated to the underlying commands' own invariants).
    - `hooks run` never forks a subprocess for the underlying command (in-process execution only), so hook output cannot leak through a shell.

13. **Compiles under `-tags nousage`**: `internal/hooks` never imports `internal/usage`; variant detection lives in the CLI layer (`pkg/whichmodel/hooks_cmd.go`) where `usage.Enabled` is only called when the binary is not compiled with the toggle disabled (F21's stub returns the `compiled_out` state; the installer then selects variant B — consistent with `which-model version` reporting `usage: compiled-out`).

## Error behaviour

| Condition | Exit | stdout | stderr |
|---|---|---|---|
| `hooks install` success (incl. idempotent re-install) | 0 | human summary lines | — |
| `hooks remove` success (incl. nothing installed) | 0 | human summary lines | — |
| `hooks run <known>` underlying success | 0 | envelope JSON + `\n` | — |
| `hooks run <known>` underlying non-zero (fail-open) | 0 | per behaviour 6 (silence or approve envelope) | — |
| `hooks run <unknown-hook>` | 2 | — | `which-model hooks: unknown hook "<name>" (known: usage-refresh, quota-guard, spawn-gate, model-audit)` |
| `hooks run <known>` with non-empty stdin that is not a JSON object | 2 | — | `which-model hooks run: [arguments] stdin is not valid JSON object` |
| `hooks install --usage --no-usage` | 2 | — | `which-model hooks: --usage and --no-usage are mutually exclusive` |
| `hooks install --target nonsense` / `--target codex` | 2 | — | `which-model hooks: unknown target "<t>" (known: claude, generic)` |
| `hooks install` when `settings.json` exists but is invalid JSON | 1 | — | parse error detail |
| `hooks install`/`remove` I/O failure, no repo root | 1 | — | error detail |
| underlying `pick` exit 4 (spawn-gate) | 0 | block envelope (behaviour 7) | — |
| `hooks run` when the binary's usage subsystem is compiled out and a usage hook is invoked directly | 0 | fail-open silence (underlying exit 2 → behaviour 6) | — |

All exit codes are within the fixed set (`specs/global/SPEC.md` §5): `0` success/fail-open, `1` runtime/I-O, `2` invocation/config errors. No new `Failure.Code` values.

## Decisions

| Decision | Value | Rationale |
|---|---|---|
| Envelope field names | `decision` (string, `"approve"`\|`"block"`), `reason` (string), `hookSpecificOutput` (object) | Claude Code's native hook-output contract; consumed as-is by `.claude/settings.json` hooks and by the generic adapter |
| `quota-guard` decision | `"block"` when any provider is at/above the critical band | Assignment requires block/warn when gated; recorded deviation from annex-c §3.4's advisory-only posture, with fail-open preserved (errors/timeouts emit nothing and never block) |
| Fail-open output | SessionStart hooks: empty stdout; dispatch hooks: `approve` envelope | Empty stdout is the cleanest "inject nothing" for `SessionStart`; an explicit `approve` keeps `PreToolUse`/`PostToolUse` behavior deterministic for the harness |
| Underlying execution | In-process via F22 `pkg/whichmodel.ExecuteCommand`, never a subprocess | No shell quoting issues, no env leakage into a child process, deterministic capture of stdout/stderr |
| Test seam | Injected `Runner` supplies command output; stdin remains host context | Hermetic tests exercise the same execution path as installed hooks |
| Claude merge strategy | Sidecar manifest `.claude/which-model-hooks.json` listing owned (event, matcher, command) triples; replace only manifest-listed entries | Ownership must survive JSON round-trips; foreign hooks are never identifiable by convention, so explicit ownership is required |
| Claude file format | Semantic JSON round-trip (`MarshalIndent`, 2-space, trailing `\n`), not byte-preserving | A JSON config cannot be spliced safely; semantic preservation is the contract |
| Generic merge strategy | Marker block `# === which-model managed hooks (do not edit) ===` … `# === end which-model managed hooks ===` in `agents/hooks.toml`; outside content byte-preserved | TOML splicing without parse/rewrite keeps foreign comments and formatting intact; the marker block is this project's own convention (annex-c §3 caveat: generic shape is ours, not a verified upstream schema) |
| Generic file deletion on remove | Delete `agents/hooks.toml` iff post-removal content is empty/whitespace-only | No sidecar manifest for TOML; whitespace-only post-removal proves nothing else lived there |
| Variant detection | At install time only, via F21 `usage.Enabled(cfg)`, or forced by `--usage`/`--no-usage`; never at runtime | Hook config is static data read by the harness before `which-model` runs (annex-c §3.5) |
| Variant B hook set | Only `spawn-gate` (UserPromptSubmit, 10s) + `model-audit` (PostToolUse, 5s) | Annex-c §3.5 verbatim: usage-dependent hooks are not installed-and-failing |
| `model-audit` history resolution | Explicit `--pick-id` or default `--last`; candidate env is correlation only | F26 history IDs select records; candidate IDs identify routes, correcting the annex example |
| `usage-refresh` payload | Empty `hookSpecificOutput` always | Annex-c §3.1 injects nothing; the hook exists only to warm the cache |
| Evidence file mode | `0600`, `O_APPEND` single-write lines | Usage-derived data is sensitive; appends must not interleave |
| Repo-root resolution | F28's `internal/skills.RepoRoot()` (`.git` upward walk + `--repo`) | Single source of truth for repo discovery (F29 depends on F28) |
| No `--user` flag on `hooks install` | — | Annex-c §3 only shows repo-local configs; user-level installation is out of scope |

## Out of scope

- `which-model serve --warm` daemon-mode equivalent (annex-c §3.1 note; F24/F25-owned).
- Verified Codex-native hook schema (annex-c §3 caveat: generic `agents/hooks.toml` is this project's own convention; a Codex adapter consuming it is a future feature).
- User-level (`~/.claude/settings.json`) hook installation.
- Runtime hook timeout enforcement (harness-owned; annex-c §3 tables).
- Historical evidence replay/backfill, evidence-file rotation or size caps.
- Shell-guard wrappers that self-detect usage state per turn (annex-c §3.5 explicitly rejects them).

## Corrections — review issues #162 and #163 (2026-09-04)

The former stdin fixture override contradicted normal host hook delivery and is superseded by mandatory execution with host context. The former positional explain command and object-shaped candidate contradicted F26's existing history selector and string candidate contracts; F26 remains authoritative. Sanitized JSONL preserves every documented F26 evidence field and removes unrelated fields.

Explicit global flags before `hooks run` are parsed before the hook name and forwarded to the underlying command. Arguments after the hook name override them. JSON remains required for underlying machine output; outer text/JSON rendering flags do not alter hook protocol. A regression covers outer offline/config/timeout and later timeout override.
