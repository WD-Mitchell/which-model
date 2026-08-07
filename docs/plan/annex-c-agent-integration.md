# Annex C: Agent Integration — Skills, Hooks, and Machine Contracts

This annex specifies how AI coding agents (Claude Code, Codex CLI/IDE extension, and generic tool-calling harnesses) consume `which-model`: the skills shipped under `agents/skills/`, the lifecycle hooks that wire `which-model` into a dispatch loop, and the JSON contracts those hooks and skills depend on. It supersedes the agent-facing artifacts of both prototypes (`usage-allowance-checks/SKILL.md` + `usage-allowance-checks/agents/openai.yaml`, and `available-model-data-export/.agents/skills/meta-orchestration-model-selection/SKILL.md`) with a clean cutover — see §2.4. It does not cover provider implementation internals (Annex A), scoring maths (Annex B), or the full CLI flag reference (Annex D); command names here are load-bearing but flags are illustrative only. See the [master plan](./README.md) for architecture and milestone context.

## 1. Design principles for agent-facing CLIs

An agent cannot ask `which-model` a clarifying question, retry with human judgment, or notice a subtly wrong answer the way a human operator would. Every principle below exists to make `which-model`'s behavior legible and safe to consume from inside an unattended tool-call loop.

| Principle | Concrete `which-model` behaviour |
|---|---|
| **Deterministic by default** | The default `pick` strategy is `score` (pure function of catalog + usage state, no RNG, no memory of prior picks). `weighted-random` and `round-robin` are opt-in via `--strategy` and MUST be explicitly requested; `weighted-random` MUST accept `--seed` and is non-deterministic only when `--seed` is omitted. `least-used` is deterministic given the same usage snapshot (tie-break by provider ID, not iteration order). |
| **Stateful/random strategies opt-in and seeded** | `round-robin` and `least-used` read/write rotation state to `internal/config` state file (`~/.which-model/state.toml`, machine-local, never committed); `which-model pick --json` MUST echo the state cursor consumed and produced so an agent can audit non-determinism after the fact. `weighted-random --seed N` MUST reproduce the exact same pick given the same catalog+usage snapshot. |
| **Stable machine schema** | Every command that agents consume MUST support `--json`, and its schema is versioned (`schema_version`, §4.5). Field additions are additive-only within a major `schema_version`; field removal or type change bumps it (§4.5). |
| **Evidence is emitted, not narrated** | `which-model explain --json` returns a structured `Evidence` object (§5) — profile inputs, band match, snapshot age, excluded candidates with reason codes. Agents MUST record this object, not a free-text explanation string, as availability proof (carried over verbatim from the `meta-orchestration-model-selection` "record the exact ID... not availability proof" rule, `available-model-data-export/.agents/skills/meta-orchestration-model-selection/SKILL.md:111-117`). |
| **Every failure has a stable code and a distinct exit code** | All `Failure.Code` values are drawn from a fixed enum (§4.7, `internal/usage/types.go` `Failure.Code`, inherited from the `UsageError` code taxonomy at `docs/plan/research/usage-allowance-checks-spec.md:16`); the process exit code (§4.7) is one of the fixed six values in Annex D §1.5, never a raw errno or panic exit. |
| **No hidden network calls in `--offline` run** | `which-model pick --offline`, `which-model usage --offline`, and `which-model catalog list --offline` MUST serve only cached/local state and MUST NOT make any network call. A stale-but-present cache is returned successfully with `Stale: true` and `Confidence: "cached"` (Annex A §6) — not an error. A **missing** cache (no file at all for that provider) fails with exit `1` and `Failure.Code = "fallback_unavailable"`. A contradictory flag combination (`--refresh-benchmarks --offline`) fails with exit `2` since Collect requires network by definition (Annex D §1.6 rule 4). The distinction: exit `2` means "the invocation itself is wrong" (retrying unchanged will fail identically); exit `1` means "the request made sense but can't be fulfilled right now" (populate the cache first, then retry). |
| **Secrets never in output** | Every `--json` and text renderer path MUST route through the same sanitization boundary the prototype enforces at the transport layer (`docs/plan/research/usage-allowance-checks-spec.md:16`, `:350-351`: tokens, headers, device codes, raw provider bodies never printed). This is a CLI-output guarantee in addition to the transport-layer guarantee — a bug in a renderer MUST NOT be able to leak a credential even if the fetch layer already sanitized its own errors. |
| **Capability state is declared, never inferred** | Every JSON output carries `usage_enabled` (boolean, always present) and `usage_disabled_reason` when false (§4.6). An agent MUST check `usage_enabled` before reading or citing any band, pressure, or quota field. **The trap: routing survives with usage off** ([master plan §6.3](./README.md)), so a degraded pick still carries a provider name and a route — a provider-attributed pick is therefore NOT evidence of usage-awareness. Inferring "it named a provider, so it must have checked quota" is exactly the mistake this field exists to prevent. |

## 2. Skill inventory

Skills ship at `agents/skills/<name>/SKILL.md`. Frontmatter format, description-as-trigger-condition phrasing, and the closing checklist convention are carried over from both prototype skills (`usage-allowance-checks/SKILL.md:1-4`, `available-model-data-export/.agents/skills/meta-orchestration-model-selection/SKILL.md:1-7`).

### 2.1 `model-selection`

Supersedes `meta-orchestration-model-selection`.

```yaml
---
name: model-selection
description: >-
  Use when choosing a verified model and reasoning-effort row for a dispatched
  task. Trigger when a task needs a deterministic model ranking from a task
  profile, explicit tier weights, or live target-harness availability
  filtering before dispatch.
---
```

**Teaches:** selecting a `which-model catalog rank` profile from task intent, reading ranked output with per-tier and per-category contributions, and filtering that ranking against what the *target dispatch harness* actually accepts before committing to a model+effort pair. The profile set and its optimization targets are preserved verbatim from the prototype (`available-model-data-export/.agents/skills/meta-orchestration-model-selection/SKILL.md:38-53`); see Annex B for the underlying weight maths.

Eleven profiles, unchanged names and intents:

| Profile | Optimizes |
|---|---|
| `simple_implementation` | cheap and fast, with instruction following |
| `simple_action_execution` | cheap and fast first, then instruction following and reliable evidence capture with tool/software-engineering support |
| `balanced_implementation` | balanced implementation value |
| `complex_implementation` | intelligence and software engineering with planning support |
| `ui_ux` | UI/UX and visual evidence plus implementation quality |
| `complex_action_execution` | tool execution, instruction following, and evidence capture |
| `financial_work` | finance, knowledge, reasoning, research, and instructions |
| `research` | research, knowledge, reasoning, and tool-assisted investigation |
| `planning` | the highest-capability planning signal |
| `orchestration` | planning and instruction following first, for delegated agent workflows |
| `review` | instruction following, software engineering, reasoning, security, and evidence capture |

**Commands documented:**

```bash
which-model catalog rank --profile balanced_implementation --top 5 --json
which-model catalog rank --profile simple_action_execution --available .tmp/live-model-efforts.txt --json
```

`--available` accepts the same shape as the prototype's live-availability file — a JSON list of `"model|reasoning"` strings or `{ "model": ..., "reasoning": ... }` objects (`available-model-data-export/.agents/skills/meta-orchestration-model-selection/SKILL.md:111-113`) — and filters *after* score calculation; it MUST NOT silently substitute another model, effort, provider, or harness for an unavailable row.

**Preserved hard rules (unchanged from prototype, non-negotiable):**
- Tier 1 (`intelligence`, `cost`, `speed`) MUST be present with positive weight in every profile; a row missing any Tier-1 score is excluded, never imputed (`available-model-data-export/.agents/skills/meta-orchestration-model-selection/SKILL.md:62-64`).
- The agent MUST inspect `warnings` and `excluded` in the ranking output before dispatch, not just `recommendation`.
- The agent MUST record the exact model ID and reasoning effort *accepted by the target harness*, sourced from `--available` filtering or a live `which-model explain` route — never invent, round, or substitute a nearby effort as a fallback when the exact row is unavailable (`available-model-data-export/.agents/skills/meta-orchestration-model-selection/SKILL.md:125-126`, `:111-117`).

**Checklist:**
- [ ] Classify task intent and complexity; choose the closest of the 11 profiles.
- [ ] Confirm intelligence, cost, and speed weights are present and positive in the run's output.
- [ ] Run `which-model catalog rank --json` and inspect `warnings`/`excluded` before trusting `recommendation`.
- [ ] Apply target-harness live availability (`--available`, or a live `which-model explain` route) with exact model + reasoning-effort IDs.
- [ ] Dispatch the exact recommended row or a listed alternative; never invent a nearby effort as fallback.
- [ ] Record model, reasoning effort, profile, and availability evidence per §5.

### 2.2 `provider-usage`

Supersedes `usage-allowance-checks`.

```yaml
---
name: provider-usage
description: >-
  Use when a user explicitly asks to inspect current usage allowance for a
  Claude, Codex, Copilot, or any other configured provider. Trigger when an
  interactive, read-only allowance report is needed without enabling
  automatic polling, spawn gating, or provider-consent enforcement.
---
```

**Teaches:** running `which-model usage` for one or more providers on explicit request only, reading normalized `Window` fields (percent/tokens/credits/USD/requests, reset hints), and the difference between a `live` and a `cached`/`estimated` snapshot for evidence purposes.

**Commands documented:**

```bash
which-model usage get claude --json
which-model usage get codex --trust-configured-origin https://trusted.example --json
which-model usage get copilot --login --json
which-model usage list --json
```

**Preserved posture (unchanged from prototype, non-negotiable):** read-only, explicit per-invocation, no background polling, no auto-spawn gating (`usage-allowance-checks/SKILL.md:8-12`). `which-model` MUST NOT schedule itself, watch a provider on a timer, or gate agent dispatch as a side effect of a `usage get` call; gating is an explicit, separate decision made by `which-model pick`/hooks (§3), never implicit in a usage read.

**Security checklist (inherited from `docs/plan/research/usage-allowance-checks-spec.md` §9, restated for the multi-provider surface):**
- [ ] Run only the provider(s) the user requested; never schedule or auto-wire a provider check.
- [ ] For any provider requiring a fallback/base-URL trust decision (e.g. Codex), confirm and explicitly pass the exact HTTPS origin to trust; never accept a near-miss or wildcard origin.
- [ ] Use `--login` only with the user present and interactive; use `--show-identity` only when identity display was explicitly requested.
- [ ] Keep output sanitized and ephemeral; never paste credential material, tokens, device codes, or raw provider response bodies into evidence, logs, or tracked files.
- [ ] Treat any provider-endpoint drift (unexpected shape/status) as a stable `unsupported_response`-class failure, not as license to follow a redirect or widen the accepted origin/response shape.

### 2.3 `usage-aware-dispatch`

New skill; no prototype predecessor.

```yaml
---
name: usage-aware-dispatch
description: >-
  Use when selecting which provider/model pair to dispatch a task to, given
  current usage allowance across multiple providers. Trigger when a task
  needs quota-aware routing, a specific selection strategy (score, priority,
  round-robin, least-used, weighted-random, cost-optimal), or documented
  evidence for why one candidate was chosen over excluded alternatives.
---
```

**Teaches:** invoking `which-model pick` with a strategy appropriate to the situation, how usage bands ([master plan §5.2](./README.md), Annex D §4.2 `[bands]` config) modulate `ModelScore` into `FinalScore`, and how to read `which-model explain --json` for defensible dispatch evidence.

**Commands documented:**

```bash
which-model pick --profile balanced_implementation --strategy score --json
which-model pick --profile research --strategy priority --json
which-model pick --profile simple_implementation --strategy least-used --json
which-model pick --profile complex_implementation --strategy weighted-random --seed 42 --json
which-model explain <candidate-id> --json
```

**Strategy selection guidance:**

| Strategy | Use when | Behaviour |
|---|---|---|
| `score` (default) | No operational constraint beyond model quality/cost/speed fit | Pure `FinalScore = ModelScore * BandWeight * ProviderWeight` ranking; deterministic. |
| `priority` | An explicit provider preference order exists (e.g. contractual, cost-tiered) | Filters/orders candidates by a configured provider priority list first, `FinalScore` breaks ties within a priority tier. |
| `round-robin` | Spreading load evenly across N interchangeable providers matters more than marginal score differences | Rotates through eligible providers by a persisted cursor (see §1 determinism note); ties within a provider by `FinalScore`. |
| `least-used` | Actively balancing consumed quota, independent of score, is the goal | Picks the eligible candidate whose provider has the lowest current `UsedPercent` in the relevant window; deterministic given the snapshot. |
| `weighted-random` | Deliberately avoiding a single hot provider becoming a bottleneck under many parallel dispatches | Samples proportional to `FinalScore`; MUST be seeded (`--seed`) for any evidence-bearing dispatch — unseeded runs are for interactive/exploratory use only. |
| `cost-optimal` | Budget constraint dominates | Filters to candidates under a cost ceiling, then ranks by `FinalScore`; ties broken by lowest `UsdEstimate` in `Evidence.ScoreInputs`. |

**Band interpretation:** each provider's current usage percent is mapped to a band tier (`low`/`standard`/`elevated`/`critical` per [master plan §5.2](./README.md) and Annex D §4.2 `[bands]` config), each tier carries a `weight` multiplier, and `direction` (`spread` vs `drain`) determines whether high consumption lowers or raises that multiplier. A candidate whose provider is at or above `gate_above_used_percent` MUST be excluded from `pick` output entirely, not merely down-weighted; `which-model explain` MUST list it under `ExcludedCandidates` with reason `band_gated`.

**Reading `which-model explain --json`:** every returned `Candidate` traces to an `Evidence` object (§5). Agents MUST record `Evidence.Profile`, `Evidence.Band`, `Evidence.SnapshotAge`, and `Evidence.Confidence` alongside the chosen model+effort — a `pick` without recorded evidence is not distinguishable, after the fact, from a guess.

**Checklist:**
- [ ] Choose a strategy from the table above based on the actual operational constraint, not habit.
- [ ] For `weighted-random`, pass `--seed` for any evidence-bearing (non-exploratory) dispatch.
- [ ] Run `which-model pick --json`; do not dispatch before confirming `exit_code == 0`.
- [ ] Run `which-model explain --json` for the chosen candidate and record its `Evidence` object.
- [ ] Confirm `Evidence.Confidence != "estimated"` before treating the pick as quota-safe under a `critical`-band provider; escalate to `provider-usage` (§2.2) for a fresh live read if stale.
- [ ] Never treat a `band_gated` excluded candidate as available; do not retry it without a fresh usage snapshot.

### 2.4 Migration note (clean cutover)

`usage-allowance-checks` (`usage-allowance-checks/SKILL.md`, `usage-allowance-checks/agents/openai.yaml`) and `meta-orchestration-model-selection` (`available-model-data-export/.agents/skills/meta-orchestration-model-selection/SKILL.md`) MUST be deleted, not retained as aliases, redirects, or deprecated stubs, once `provider-usage` and `model-selection` ship. Any harness configuration, documentation, or hook referencing the old skill names MUST be updated in the same change that deletes them — there is no dual-running period.

### 2.5 Skill applicability under a disabled usage subsystem

All three toggle levels ([master plan §6](./README.md)) change which skills apply. Detection is cheap and MUST happen before any usage reasoning: `which-model config show --json` reports the resolved effective state, and any `which-model usage`/`which-model auth` invocation returns exit `2` with `usage_disabled` or `usage_compiled_out`.

| Skill | Usage disabled | Required behaviour |
|---|---|---|
| `model-selection` | **fully applicable** | Works unchanged — this is its original prototype behaviour. It is the correct skill for a usage-disabled installation, and its checklist needs no amendment. |
| `provider-usage` | **inapplicable** | MUST report the toggle and its source to the user, then stop. MUST NOT try alternative credential paths, MUST NOT suggest re-enabling usage, MUST NOT treat exit `2` as retryable. |
| `usage-aware-dispatch` | **defers** | MUST check `usage_enabled` up front and, when false, hand off to `model-selection` rather than attempting band reasoning against absent data. |

Added to the `provider-usage` checklist:

- [ ] Before fetching, confirm usage is enabled. If disabled, report which lever disabled it (`--no-usage`, `[usage] enabled`, or a `nousage` build) and stop — do not work around it.

Added to the `usage-aware-dispatch` checklist:

- [ ] Check `usage_enabled` before any band reasoning. If false, defer to `model-selection` and record that the pick is score-only.
- [ ] Never cite a band, pressure, or quota figure that is absent from the output (§4.6, §5.1).

`model-selection` is deliberately the fallback rather than a degraded variant of `usage-aware-dispatch`: it already encodes exactly the right behaviour for a world without usage data, so a disabled installation gets a skill written for its situation rather than one apologising for missing inputs.

## 3. Hook inventory

All hooks below are **fail-open**: a hook that errors, times out, or returns non-zero MUST NOT block, cancel, or degrade the agent turn it's attached to. Each hook is advisory context injection or best-effort cache warming — never a hard gate. (`which-model pick`'s own `band_gated` exclusion, §2.3, is the actual gate; it is a synchronous CLI decision the agent makes deliberately, not a hook side effect.)

Two ecosystems are covered: Claude Code hooks (`.claude/settings.json`) and a generic/Codex-oriented convention. **Codex CLI's hook/lifecycle-event mechanism and its exact event names are not independently verified in this research pass** — no Codex hook-config file was inspected under this repo or upstream Codex docs during this task. The generic snippets below are written against a plausible `agents/hooks.toml` convention this project would define for any non-Claude-Code harness (including a bespoke Codex adapter); they are NOT claimed to match a pre-existing Codex-native hook schema. Treat the "generic config" column as this project's own hook file, consumed by an adapter this project owns, not as a verified upstream Codex feature.

### 3.1 session-start cache warm

| | |
|---|---|
| Trigger | Agent session/conversation start |
| Command | `which-model usage refresh --json --quiet --timeout 5s` (bounded refresh of all configured providers into cache; `serve --warm` is the daemon-mode equivalent when `which-model serve` is already running) |
| Timeout | 5s hard cap (hook-level timeout in addition to the command's own `--timeout`) |
| Failure posture | Fail-open: hook exit code is ignored by the harness; a failed/timed-out warm leaves the cache as it was (possibly cold), and the first `which-model pick` simply pays the live-fetch cost itself rather than the session paying it upfront. |
| Injects | Nothing into the transcript directly; this hook exists purely to reduce first-`pick` latency by pre-populating `internal/usage/cache.go`. |

**Claude Code (`.claude/settings.json`):**

```json
{
  "hooks": {
    "SessionStart": [
      {
        "matcher": "*",
        "hooks": [
          {
            "type": "command",
            "command": "which-model usage refresh --json --quiet --timeout 5s",
            "timeout": 5
          }
        ]
      }
    ]
  }
}
```

**Generic (`agents/hooks.toml`):**

```toml
[[hooks]]
event = "session_start"
command = "which-model usage refresh --json --quiet --timeout 5s"
timeout_ms = 5000
on_failure = "ignore"   # fail-open: never blocks session start
```

### 3.2 pre-dispatch model resolution

| | |
|---|---|
| Trigger | Immediately before an agent hands a task off to a specific model/provider (subagent spawn, tool-dispatch boundary) |
| Command | `which-model pick --profile "$WHICH_MODEL_TASK_PROFILE" --strategy score --json` |
| Timeout | 8s |
| Failure posture | Fail-open: on any non-zero exit or timeout, the hook injects nothing and the harness falls back to its own default model selection; it MUST NOT abort the dispatch. |
| Injects | The `Candidate` JSON (§4.2) for the top pick — model ID, reasoning effort, provider, `FinalScore`, and a pointer (`candidate_id`) usable with `which-model explain` in the post-dispatch hook. |

**Claude Code (`.claude/settings.json`):**

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Task",
        "hooks": [
          {
            "type": "command",
            "command": "which-model pick --profile \"${WHICH_MODEL_TASK_PROFILE:-balanced_implementation}\" --strategy score --json",
            "timeout": 8
          }
        ]
      }
    ]
  }
}
```

**Generic (`agents/hooks.toml`):**

```toml
[[hooks]]
event = "pre_dispatch"
command = "which-model pick --profile ${WHICH_MODEL_TASK_PROFILE} --strategy score --json"
timeout_ms = 8000
on_failure = "ignore"
inject_as = "context.which_model_pick"
```

### 3.3 post-dispatch evidence recording

| | |
|---|---|
| Trigger | Immediately after a dispatched task completes (subagent yield, tool-call completion) |
| Command | `which-model explain "$WHICH_MODEL_CANDIDATE_ID" --json >> .which-model/evidence.jsonl` |
| Timeout | 5s |
| Failure posture | Fail-open: a failed evidence write MUST NOT fail or retry the parent task; it is a best-effort audit trail, appended, never blocking. |
| Injects | Nothing into the live transcript; appends one `Evidence` object (§5) per dispatch to a local append-only log for later audit/debugging. |

**Claude Code (`.claude/settings.json`):**

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Task",
        "hooks": [
          {
            "type": "command",
            "command": "which-model explain \"$WHICH_MODEL_CANDIDATE_ID\" --json >> .which-model/evidence.jsonl",
            "timeout": 5
          }
        ]
      }
    ]
  }
}
```

**Generic (`agents/hooks.toml`):**

```toml
[[hooks]]
event = "post_dispatch"
command = "which-model explain ${WHICH_MODEL_CANDIDATE_ID} --json >> .which-model/evidence.jsonl"
timeout_ms = 5000
on_failure = "ignore"
```

### 3.4 quota-guard advisory

| | |
|---|---|
| Trigger | Session start, and/or periodically before pre-dispatch (harness-scheduled; `which-model` itself never self-schedules per §2.2) |
| Command | `which-model usage list --json --band-at-or-above critical` |
| Timeout | 5s |
| Failure posture | Fail-open: no output or a failure means no warning is shown; this hook only ever adds an advisory, it never blocks a subsequent `pick` (the actual gate is `pick`'s own `band_gated` exclusion, §2.3). |
| Injects | A short text/JSON advisory listing any provider currently at or above the `critical` band threshold, so the agent can proactively choose `priority`/`least-used` strategy or warn the user, before `pick` silently excludes that provider. |

**Claude Code (`.claude/settings.json`):**

```json
{
  "hooks": {
    "SessionStart": [
      {
        "matcher": "*",
        "hooks": [
          {
            "type": "command",
            "command": "which-model usage list --json --band-at-or-above critical --quiet",
            "timeout": 5
          }
        ]
      }
    ]
  }
}
```

**Generic (`agents/hooks.toml`):**

```toml
[[hooks]]
event = "session_start"
command = "which-model usage list --json --band-at-or-above critical --quiet"
timeout_ms = 5000
on_failure = "ignore"
inject_as = "context.which_model_quota_guard"
```

### 3.5 Hook installation under a disabled usage subsystem

Two of the four hooks depend on usage data. They MUST NOT be installed in a usage-disabled installation — not installed-and-failing. Fail-open makes a wrongly-installed hook *harmless*, but a guaranteed-failing subprocess on every session start is still pure noise, and noise trains operators to ignore hook failures that matter.

| Hook | Usage disabled |
|---|---|
| §3.1 session-start cache warm | **Do not install** — nothing to warm; `which-model usage`/`which-model serve` exit `2` |
| §3.2 pre-dispatch model resolution | **Install** — works in degraded mode; resolves model+effort from scores alone |
| §3.3 post-dispatch evidence recording | **Install** — records degraded evidence per §5.1, which is still valid evidence for the claim it makes |
| §3.4 quota-guard advisory | **Do not install** — there is no quota to guard |

Rather than one config with runtime conditionals, ship **two config variants**. A hook config is static data read by the harness before `which-model` runs, so it cannot branch on `which-model config show` output; attempting to make it self-detecting would mean wrapping every hook in a shell guard, which is harder to read and still spawns a process to decide not to spawn a process.

`.claude/settings.json`, usage-disabled variant — §3.1 and §3.4 entries simply absent:

```json
{
  "hooks": {
    "UserPromptSubmit": [
      {
        "matcher": "*",
        "hooks": [
          {
            "type": "command",
            "command": "which-model pick --profile balanced_implementation --no-usage --json --quiet",
            "timeout": 10
          }
        ]
      }
    ]
  }
}
```

`agents/hooks.toml`, usage-disabled variant:

```toml
# No session_start warm hook and no quota_guard hook: usage is disabled.
[[hooks]]
event = "pre_dispatch"
command = "which-model pick --profile balanced_implementation --no-usage --json --quiet"
timeout_ms = 10000
on_failure = "ignore"

[[hooks]]
event = "post_dispatch"
command = "which-model explain --last --json"
timeout_ms = 5000
on_failure = "ignore"
```

Installation guidance: a `which-model`-aware installer MAY select the variant by shelling `which-model config show --json` once at install time (or checking `which-model version` for `usage: compiled-out`) and writing the appropriate file. That is a one-off decision at install, not a per-turn runtime check. The same caveat from §3 applies — the generic `agents/hooks.toml` shape is this project's own convention, not a verified upstream Codex schema.

All hooks remain **fail-open** in both variants. Conditional installation reduces noise; it does not replace the fail-open guarantee.

## 4. Machine contracts

`schema_version` is present on every JSON object below and MUST be checked by consuming agents/hooks before parsing (§4.5). `which-model schema <command>` emits the JSON Schema for that command's current output (e.g. `which-model schema usage`, `which-model schema pick`, `which-model schema explain`) so a harness can validate output structurally instead of hand-parsing.

### 4.1 `which-model usage --json` → `Snapshot`

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://github.com/WD-Mitchell/which-model/schema/usage-snapshot.json",
  "title": "which-model usage --json output",
  "type": "object",
  "required": ["schema_version", "snapshots"],
  "properties": {
    "schema_version": { "type": "string", "const": "1.0" },
    "snapshots": {
      "type": "array",
      "items": { "$ref": "#/$defs/Snapshot" }
    }
  },
  "$defs": {
    "Unit": {
      "type": "string",
      "enum": ["percent", "tokens", "credits", "usd", "requests", "kwh", "none"]
    },
    "Window": {
      "type": "object",
      "required": ["id", "label", "unit", "usage_known"],
      "properties": {
        "id": { "type": "string" },
        "label": { "type": "string" },
        "unit": { "$ref": "#/$defs/Unit" },
        "used_percent": { "type": ["number", "null"] },
        "used": { "type": ["number", "null"] },
        "limit": { "type": ["number", "null"] },
        "remaining": { "type": ["number", "null"] },
        "unlimited": { "type": "boolean", "default": false },
        "window_minutes": { "type": ["integer", "null"] },
        "resets_at": { "type": ["string", "null"], "format": "date-time" },
        "reset_hint": { "type": "string" },
        "model_scope": { "type": "array", "items": { "type": "string" } },
        "synthetic": { "type": "boolean", "default": false },
        "usage_known": { "type": "boolean" }
      },
      "additionalProperties": false
    },
    "Failure": {
      "type": "object",
      "required": ["code", "message"],
      "properties": {
        "code": { "type": "string" },
        "message": { "type": "string" }
      },
      "additionalProperties": false
    },
    "Snapshot": {
      "type": "object",
      "required": ["provider", "windows", "fetched_at", "source", "confidence"],
      "properties": {
        "provider": { "type": "string" },
        "account": { "type": "string" },
        "plan": { "type": "string" },
        "windows": { "type": "array", "items": { "$ref": "#/$defs/Window" } },
        "fetched_at": { "type": "string", "format": "date-time" },
        "source": { "type": "string", "enum": ["oauth", "api", "cli", "web", "local", "cache"] },
        "confidence": { "type": "string", "enum": ["live", "cached", "estimated"] },
        "stale": { "type": "boolean", "default": false },
        "error": { "oneOf": [{ "$ref": "#/$defs/Failure" }, { "type": "null" }] }
      },
      "additionalProperties": false
    }
  }
}
```

### 4.2 `which-model pick --json` → ranked `Candidate` list

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://github.com/WD-Mitchell/which-model/schema/pick-result.json",
  "title": "which-model pick --json output",
  "type": "object",
  "required": ["schema_version", "profile", "strategy", "candidates", "excluded_candidates"],
  "properties": {
    "schema_version": { "type": "string", "const": "1.0" },
    "profile": { "type": "string" },
    "strategy": {
      "type": "string",
      "enum": ["score", "priority", "round-robin", "least-used", "weighted-random", "cost-optimal"]
    },
    "seed": { "type": ["integer", "null"] },
    "candidates": {
      "type": "array",
      "items": { "$ref": "#/$defs/Candidate" }
    },
    "excluded_candidates": {
      "type": "array",
      "items": { "$ref": "#/$defs/ExcludedCandidate" }
    }
  },
  "$defs": {
    "Route": {
      "type": "object",
      "required": ["provider", "model_id", "model", "reasoning", "window_ids"],
      "properties": {
        "provider": { "type": "string" },
        "model_id": { "type": "string" },
        "model": { "type": "string" },
        "reasoning": {
          "type": "string",
          "enum": ["minimal", "low", "medium", "high", "xhigh", "max", "default"]
        },
        "window_ids": { "type": "array", "items": { "type": "string" } }
      },
      "additionalProperties": false
    },
    "Candidate": {
      "type": "object",
      "required": ["candidate_id", "route", "model_score", "band", "band_weight", "provider_weight", "final_score"],
      "properties": {
        "candidate_id": { "type": "string" },
        "route": { "$ref": "#/$defs/Route" },
        "model_score": { "type": "number" },
        "band": { "type": "string" },
        "band_weight": { "type": "number" },
        "provider_weight": { "type": "number" },
        "final_score": { "type": "number" },
        "warnings": { "type": "array", "items": { "type": "string" } }
      },
      "additionalProperties": false
    },
    "ExcludedCandidate": {
      "type": "object",
      "required": ["route", "reason_code", "reason"],
      "properties": {
        "route": { "$ref": "#/$defs/Route" },
        "reason_code": {
          "type": "string",
          "enum": ["band_gated", "no_score_row", "auth_required", "provider_error", "not_in_availability_list"]
        },
        "reason": { "type": "string" }
      },
      "additionalProperties": false
    }
  }
}
```

### 4.3 `which-model explain --json` → `Evidence`

Schema for `Evidence` is defined once here and referenced from §4.2/§5; `which-model explain <candidate-id> --json` returns a single object of this shape (not wrapped in an array), plus `schema_version` and `candidate` for convenience.

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://github.com/WD-Mitchell/which-model/schema/explain-result.json",
  "title": "which-model explain --json output",
  "type": "object",
  "required": ["schema_version", "candidate", "evidence"],
  "properties": {
    "schema_version": { "type": "string", "const": "1.0" },
    "candidate": { "type": "string", "description": "candidate_id echoed back" },
    "evidence": { "$ref": "#/$defs/Evidence" }
  },
  "$defs": {
    "Evidence": {
      "type": "object",
      "required": ["profile", "score_inputs", "band", "snapshot_age_seconds", "confidence", "route_provenance", "excluded_candidates", "last_verified"],
      "properties": {
        "profile": { "type": "string" },
        "score_inputs": {
          "type": "object",
          "description": "tier1 + category composite values that produced model_score",
          "additionalProperties": { "type": "number" }
        },
        "band": {
          "type": "object",
          "required": ["name", "used_percent", "weight"],
          "properties": {
            "name": { "type": "string" },
            "used_percent": { "type": "number" },
            "weight": { "type": "number" }
          },
          "additionalProperties": false
        },
        "snapshot_age_seconds": { "type": "number" },
        "confidence": { "type": "string", "enum": ["live", "cached", "estimated"] },
        "route_provenance": {
          "type": "string",
          "description": "how the provider<->model route was resolved, e.g. 'static-config' | 'live-availability-probe'"
        },
        "excluded_candidates": {
          "type": "array",
          "items": { "$ref": "https://github.com/WD-Mitchell/which-model/schema/pick-result.json#/$defs/ExcludedCandidate" }
        },
        "last_verified": {
          "type": "string",
          "format": "date-time",
          "description": "when the provider adapter last confirmed this route/model was live-accepted by the target harness"
        }
      },
      "additionalProperties": false
    }
  }
}
```

### 4.4 `which-model schema` command

`which-model schema <command>` (e.g. `which-model schema usage`, `which-model schema pick`, `which-model schema explain`) MUST print the JSON Schema document matching that command's `--json` output at the currently-installed `which-model` version, byte-identical to the schemas above at `schema_version: "1.0"`. This lets a harness validate structurally (`ajv`, `jsonschema`, etc.) rather than hand-parsing, and lets an agent detect a schema drift before parsing fails opaquely.

### 4.5 `schema_version` compatibility policy

- `schema_version` is a `"MAJOR.MINOR"` string, independent of the `which-model` binary's own release version.
- **Non-breaking (MINOR bump, e.g. `1.0` → `1.1`):** adding a new optional field; adding a new enum value to an open-ended enum (`Failure.code`, `ExcludedCandidate.reason_code`); adding a new top-level command output. Consumers MUST ignore unknown fields and unknown enum values they don't recognize rather than erroring.
- **Breaking (MAJOR bump, e.g. `1.x` → `2.0`):** removing or renaming a field; changing a field's type or nullability; removing an enum value; changing the meaning of an existing field (e.g. `used_percent` semantics). A MAJOR bump MUST ship in the same release as a `which-model` major/minor version bump, never silently.
- **Consumer pinning:** agents/hooks that parse `--json` output SHOULD assert `schema_version` starts with the MAJOR they were written against (e.g. reject if `schema_version` is not `1.*`) and fail closed (treat as `Failure.Code = "schema_incompatible"`, not exit 0) rather than attempting best-effort parsing against an unknown MAJOR.
- `which-model --version` and `which-model schema <command>`'s `$id` together allow an agent to correlate a running binary to the exact schema it emits.

### 4.6 `usage_enabled` and `usage_disabled_reason`

Added to the root object of **all three** outputs (`which-model usage`, `which-model pick`, `which-model explain`):

```json
{
  "usage_enabled": {
    "type": "boolean",
    "description": "Whether the usage subsystem was active for this invocation. When false, band/pressure/quota fields are absent and the pick is pure score ranking."
  },
  "usage_disabled_reason": {
    "type": "string",
    "enum": ["flag", "config", "compiled_out", "no_providers_enabled"],
    "description": "Present if and only if usage_enabled is false. 'flag' = --no-usage; 'config' = [usage] enabled = false; 'compiled_out' = binary built with -tags nousage; 'no_providers_enabled' = [usage] enabled = \"auto\" resolved off because no provider is enabled."
  }
}
```

`usage_enabled` is in the `required` array of every root object. `usage_disabled_reason` is conditionally required via `if/then` on `usage_enabled: false`.

**Degraded `which-model pick --json`** — note what is *absent* rather than falsified:

```json
{
  "schema_version": "2.0",
  "usage_enabled": false,
  "usage_disabled_reason": "flag",
  "profile": "balanced_implementation",
  "strategy": "score",
  "normalizer": "minmax-linear",
  "aggregator": "weighted-arithmetic-mean",
  "recommendation": {
    "route": {
      "provider": "claude",
      "model_id": "claude-opus-4-8-20260115",
      "model": "Claude Opus 4.8",
      "reasoning": "max",
      "provenance": "models_dev"
    },
    "model_score": "84",
    "provider_weight": "1.0",
    "final_score": "84",
    "warnings": []
  },
  "alternatives": [],
  "excluded": []
}
```

`band`, `band_weight`, and `pressure` are **omitted entirely** — not `null`, and emphatically not `1.0` paired with a fabricated band name like `"low"`. A synthetic band value would be indistinguishable from a genuine measurement of an idle provider, which is the single worst outcome for an agent recording evidence. Absence is unambiguous; a plausible-looking default is not.

Note also that `route.provider` is still populated. That is routing, which survives with usage off ([master plan §6.3](./README.md)) — hence the §1 principle that a provider name is not evidence of usage-awareness.

**Compatibility classification.** Adding a field to `required` is, by §4.5's own rules, a **breaking change** — a consumer written against `1.0` that validates strictly would reject output lacking a field it never knew about, and more importantly a `1.0` consumer has no way to know it must check `usage_enabled` before trusting a band field. Per §4.5 this is therefore a **MAJOR bump to `schema_version: "2.0"`**, shipped alongside a `which-model` minor version bump, not a silent `1.1`.

The alternative — making `usage_enabled` optional and defaulting absence to `true` — was rejected: it would mean an old consumer silently reads a degraded pick as usage-aware, which is precisely the failure this field exists to prevent. A loud MAJOR bump that forces consumers to look is the correct cost.

### 4.7 Exit codes

| Code | Meaning | Agent-side handling |
|---|---|---|
| `0` | Success | Parse `--json` output per the schema above; proceed. |
| `1` | Runtime error (unexpected failure, not classified below) | Do not retry blindly; surface `Failure.message` to the user/log; treat as a hard stop for this dispatch attempt. |
| `2` | Argument/usage error | A bug in the calling hook/skill, not a transient condition; fix the invocation, do not retry unchanged. |
| `3` | No viable candidate after filtering (`which-model pick` only) | Every candidate was excluded (score/availability filtering, not quota gating). Widen the profile/`--available` list or ask the user; do not silently fall back to an unranked model. |
| `4` | All eligible providers band-gated or exhausted | This is the quota-specific empty result — every candidate's provider exceeded `gate_above_used_percent`. `--offline` failures are NOT exit `4`; a missing cache under `--offline` is exit `1` (`fallback_unavailable`), and a contradictory flag combination like `--refresh-benchmarks --offline` is exit `2` (Annex D §1.5, §1.6). Surface which providers were gated (`ExcludedCandidate.reason_code == "band_gated"`) via `which-model explain`/quota-guard hook (§3.4); do not treat as a generic error. |
| `5` | Authentication required | Route to the `provider-usage` skill's explicit, user-present `--login` flow (§2.2); never attempt unattended login. |

## 5. Evidence format

`Evidence` (schema in §4.3) is the object an agent MUST paste as availability proof — carrying forward the prototype's "record the exact ID and resolved effort accepted by the target harness; model names or Artificial Analysis slugs alone are not availability proof" rule (`available-model-data-export/.agents/skills/meta-orchestration-model-selection/SKILL.md:111-117`) into a structured, machine-checkable form instead of a prose reminder.

Fields and their purpose:

| Field | Purpose |
|---|---|
| `profile` | Which of the 11 task profiles (§2.1) produced this candidate's ranking — ties the pick back to task intent. |
| `score_inputs` | The Tier-1 + category composite values that produced `model_score`, so the ranking is reproducible/auditable without re-running `which-model catalog rank`. |
| `band.{name,used_percent,weight}` | Which usage band this provider was in at pick time and what multiplier it applied — explains why `final_score` differs from raw `model_score`. |
| `snapshot_age_seconds` | Age of the usage snapshot the band decision was made against — required to judge whether the pick is still trustworthy at read time. |
| `confidence` | `live` / `cached` / `estimated`, echoed from the underlying `Snapshot` (§4.1) — a `cached`/`estimated` pick against a `critical`-adjacent band is weaker evidence than a `live` one and MUST be flagged as such by the consuming skill (§2.3 checklist). |
| `route_provenance` | How the provider↔model route was established (static config vs. a live availability probe) — distinguishes "we assume this model is servable" from "we confirmed it." |
| `excluded_candidates` | Every candidate that was filtered out and why (`reason_code`), so a reviewer can see the full search space, not just the winner. |
| `last_verified` | Timestamp the provider adapter last confirmed this exact route was live-accepted by the target harness — the closest analogue to the prototype's "exact accepted row" proof requirement. |

### 5.1 Evidence in degraded mode

With usage off at any level, `Evidence` reports what it actually knows and omits the rest. Omission, never a plausible-looking default.

| Field | Degraded | Reason |
|---|---|---|
| `profile`, `score_inputs`, `model_score` | **retained** | Pure catalog data; independent of usage |
| `normalizer`, `aggregator` | **retained** | Annex B §4.0 — a score must always be traceable to its method |
| `excluded_candidates` | **retained** | Filtering still happens (`--provider`, availability, Tier-1 completeness) |
| `route_provenance` | **retained**, but never `provider_live` | Credentialed confirmation is unavailable, so provenance is `models_dev` or `user_declared` (Annex B §7.1a) |
| `catalog_data_age` | **retained** | The scores CSV still has a vintage worth recording |
| `band`, `band_weight`, `pressure` | **omitted** | Never measured |
| `snapshot_age`, `confidence` | **omitted** | No snapshot exists |
| `last_verified` | **omitted** | Provider adapter was never consulted |
| `usage_enabled`, `usage_disabled_reason` | **added** | §4.6 |

Normative rules:

- An agent MUST NOT present degraded evidence as usage-aware. The `usage_enabled: false` field makes this checkable rather than a matter of trust.
- Emitted evidence MUST be **self-describing**: a reviewer reading the object cold, with no access to the invocation that produced it, MUST be able to tell which mode produced it. That is why `usage_disabled_reason` names the specific lever (`flag` / `config` / `compiled_out` / `no_providers_enabled`) instead of a bare boolean.
- Degraded evidence is still legitimate evidence for the claim it actually makes — "this is the best-scoring model for this profile" — and remains valid proof of the exact model and reasoning effort. It simply makes no claim about allowance, and MUST NOT be read as making one.

## 6. Agent interface descriptors

One descriptor per skill, ported forward from the `interface:` shape in `usage-allowance-checks/agents/openai.yaml:1-4`.

`agents/skills/model-selection/agents/openai.yaml`:

```yaml
interface:
  display_name: "Model selection"
  short_description: "Rank models by task profile and filter to live harness availability"
  default_prompt: "Use $model-selection to choose a deterministic model and reasoning-effort row for this task from a task profile, then confirm it against the target harness's live availability before dispatch."
```

`agents/skills/provider-usage/agents/openai.yaml`:

```yaml
interface:
  display_name: "Provider usage"
  short_description: "Safely report usage allowance for any configured provider"
  default_prompt: "Use $provider-usage to run one explicit, read-only provider usage allowance report without automatic polling or enforcement."
```

`agents/skills/usage-aware-dispatch/agents/openai.yaml`:

```yaml
interface:
  display_name: "Usage-aware dispatch"
  short_description: "Pick a provider/model pair with a quota-aware strategy and record dispatch evidence"
  default_prompt: "Use $usage-aware-dispatch to select a dispatch strategy appropriate to the operational constraint, run which-model pick, and record which-model explain evidence for the chosen candidate before dispatching."
```

## 7. Anti-patterns

Agents integrating with `which-model` MUST NOT:

- Poll usage in a loop (busy-wait or fixed-interval re-check) instead of relying on the session-start/quota-guard hooks (§3.1, §3.4) and explicit `provider-usage` invocations (§2.2); this defeats the "explicit invocation, no background polling" posture inherited from the prototype.
- Enable `--login` unattended; device/OAuth login flows MUST only run with the user present and interactively completing the flow (§2.2 checklist).
- Print account/identity information without `--show-identity` explicitly passed by the user's own request.
- Treat a `confidence: "cached"` or `confidence: "estimated"` snapshot as live proof of current quota state, especially near a `critical` band boundary — re-fetch live before a quota-sensitive decision.
- Substitute a nearby reasoning effort when the exact model+effort row is unavailable in `--available`/live-probe output; an unavailable exact row is a hard exclusion, not a rounding problem (§2.1, §5).
- Parse text/human-readable output instead of `--json`; text formatting is not a stable contract and MAY change without a `schema_version` bump.
- Cite band, pressure, or quota evidence when `usage_enabled` is `false` (§1, §4.6). A degraded pick still names a provider; that is routing, not usage-awareness.
- Attempt to re-enable usage by editing the user's config, injecting `--no-usage`-inverting flags, or suggesting a rebuild. The toggle is a user decision; report it and proceed with `model-selection` (§2.1).
- Treat `usage_disabled` or `usage_compiled_out` (exit `2`) as retryable. Retrying the identical invocation will fail identically — these are configuration/build facts, not transient errors.
