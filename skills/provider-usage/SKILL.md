---
name: provider-usage
description: >-
  Use when a user explicitly asks to inspect current usage allowance for a
  Claude, Codex, Copilot, or any other configured provider. Trigger when an
  interactive, read-only allowance report is needed without enabling
  automatic polling, spawn gating, or provider-consent enforcement.
---

# Provider usage: explicit, read-only allowance reports

Run one explicit `which-model usage` report for the provider(s) the user
asked about. These reads never schedule anything, never poll in the
background, and never gate agent spawns (annex-c §2.2 posture).

## When to use

- The user explicitly asks for current usage/allowance for a named provider.
- A live allowance read is needed before a quota-sensitive decision
  (`usage-aware-dispatch` asks for this when a pick's
  `evidence.confidence` is `cached` or `estimated` near a `critical` band).

## Commands

```bash
which-model usage claude --json
which-model usage codex --trust-configured-origin https://trusted.example --json
which-model usage copilot --login --json
which-model usage --all --json
```

- `--trust-configured-origin <origin>` is required for any provider with a
  configured fallback base URL (Codex); the origin must match exactly.
- `--login` runs the device/browser flow and MUST only be used with the user
  present; unattended login is refused (exit 2).
- `--show-identity` only when the user explicitly asked for identity.

## Before fetching

Check `usage_enabled` FIRST (annex-c §2.5, §4.6): run
`which-model config show --json` and read `usage_enabled`. If `false`, report
which lever disabled it (`usage_disabled_reason`: flag/config/compiled_out/
no_providers_enabled) and STOP. Do not try alternative credential paths, do
not suggest re-enabling usage, and do not treat exit 2 as retryable.

## Reading the output

From `which-model usage <provider> --json` read the `snapshots` array (one
entry per provider, request order). For each snapshot:

- `provider`, `confidence` (`live` | `cached` | `estimated`), `source`,
  `stale`, `fetched_at`.
- `windows[].id`, `windows[].label`, `windows[].unit`, `windows[].used_percent`,
  `windows[].used`, `windows[].limit`, `windows[].remaining`, `windows[].unlimited`,
  `windows[].resets_at`, `windows[].reset_hint`, `windows[].usage_known`.
- `error` — an inline `Failure` (code + sanitized message) means that
  provider failed; the other snapshots are still reported.

A `cached`/`estimated` snapshot is NOT live proof of current quota —
re-fetch before a quota-sensitive decision.

## Failure handling (exit codes, annex-c §4.7)

| exit | meaning | action |
|---|---|---|
| 0 | all requested providers reported (even with inline per-provider `Failure`) | parse per the shapes above |
| 1 | runtime error | surface `Failure.message`; do not retry blindly |
| 2 | argument/config error, or usage disabled | fix the invocation or report the disabled lever; NEVER retry a `usage_disabled`/`usage_compiled_out` exit unchanged |
| 3 | no provider produced a usable allowance snapshot | report that no viable usage result remains; do not invent allowance data |
| 4 | `--fail-on-gated` and a gate was crossed | report which provider/window crossed `gate_above_used_percent` |
| 5 | every requested provider failed auth (`unauthorized`/`login_required`-class) | run the explicit, user-present `--login` flow |

## Recording evidence

Never paste credential material, tokens, device codes, or raw provider
bodies into evidence, logs, or tracked files — output is sanitized; keep it
that way. Record the snapshot fields above (provider, confidence, source,
windows) when they back a quota claim; never record a `cached` snapshot as a
live reading.

## Checklist

- [ ] Confirm `usage_enabled` before fetching; if disabled, report the lever and stop.
- [ ] Run only the provider(s) the user requested; never schedule or auto-wire a provider check.
- [ ] For fallback/base-URL trust (Codex), pass the exact HTTPS origin; never a near-miss or wildcard.
- [ ] Use `--login` only with the user present; `--show-identity` only when requested.
- [ ] Keep output sanitized and ephemeral; never paste credential or raw provider material into evidence.
- [ ] Treat endpoint drift as a stable `unsupported_response`-class failure; never follow redirects or widen the accepted origin/shape.
