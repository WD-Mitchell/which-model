---
name: usage-aware-dispatch
description: >-
  Use when selecting which provider/model pair to dispatch a task to, given
  current usage allowance across multiple providers. Trigger when a task
  needs quota-aware routing, a specific selection strategy (score, priority,
  round-robin, least-used, weighted-random, cost-optimal), or documented
  evidence for why one candidate was chosen over excluded alternatives.
---

# Usage-aware dispatch: quota-aware pick with recorded evidence

Choose a dispatch strategy, run `which-model pick`, and record the
`which-model explain` evidence for the chosen candidate before dispatching.
This skill DEFERS to `model-selection` when the usage subsystem is disabled
(annex-c §2.5) — a disabled installation gets a score-only pick, never band
reasoning against absent data.

## When to use

- A task needs a provider/model pair chosen with awareness of current
  allowance across providers.
- A specific strategy is warranted (priority order, load spreading, quota
  balancing, seeded randomization, cost ceiling).
- Defensible evidence is required for why one candidate beat excluded
  alternatives.

## Check usage first

Check `usage_enabled` FIRST (annex-c §1, §4.6) in the `pick` output. If
`false`, defer: hand off to `model-selection`, run the pick score-only, and
record that the pick is score-only (`usage_disabled_reason`). Never cite a
band, pressure, or quota figure that is absent from the output.

## Commands

```bash
which-model pick --profile balanced_implementation --strategy score --json
which-model pick --profile research --strategy priority --json
which-model pick --profile simple_implementation --strategy least-used --json
which-model pick --profile complex_implementation --strategy weighted-random --seed 42 --json
which-model explain <candidate-id> --json
```

Strategy guidance (annex-c §2.3): `score` = no operational constraint beyond
quality/cost/speed; `priority` = explicit provider preference order;
`round-robin` = spread load across interchangeable providers; `least-used` =
balance consumed quota; `weighted-random` = avoid hot-provider bottlenecks
(MUST pass `--seed` for any evidence-bearing dispatch); `cost-optimal` =
budget ceiling dominates.

## Reading the output

From `which-model pick --json` (annex-c §4.2):

- `usage_enabled` — MUST be checked before any band reasoning.
- `profile`, `strategy`, `seed`.
- `candidates[0].candidate_id`, `candidates[0].route.{provider,model_id,model,reasoning}`,
  `candidates[0].band`, `.band_weight`, `.provider_weight`, `.final_score`,
  `.warnings`.
- `excluded_candidates[].reason_code` — `band_gated` means the provider was
  at or above `gate_above_used_percent`; such candidates are NEVER available
  and MUST NOT be retried without a fresh usage snapshot.

From `which-model explain <candidate-id> --json` read `evidence`:

- `evidence.profile`, `evidence.band.{name,used_percent,weight}`,
  `evidence.snapshot_age_seconds`, `evidence.confidence`,
  `evidence.route_provenance`, `evidence.excluded_candidates`,
  `evidence.last_verified`.
- A pick without recorded evidence is indistinguishable from a guess:
  record `Evidence.Profile`, `Evidence.Band`, `Evidence.SnapshotAge`, and
  `Evidence.Confidence` with the chosen model+effort.

## Failure handling (exit codes, annex-c §4.7)

| exit | meaning | action |
|---|---|---|
| 0 | at least one candidate returned | parse `--json`; do not dispatch before confirming exit 0 |
| 1 | runtime error | surface `Failure.message`; hard stop for this dispatch attempt |
| 2 | argument error (bad strategy/seed/flag combination) | fix the invocation; do not retry unchanged |
| 3 | no viable candidate after filtering | widen profile/`--available`/exclusions; ask the user |
| 4 | ALL eligible providers band-gated | surface the gated providers (`reason_code == "band_gated"` via explain/quota-guard); do NOT dispatch to a gated provider; warn the user; do not treat as a generic error |
| 5 | authentication required | route to `provider-usage`'s explicit, user-present `--login` flow; never unattended login |

## Recording evidence

Record the exact model ID and reasoning effort accepted by the target
harness AND the full `Evidence` object for the chosen candidate (annex-c §5)
— never a free-text availability claim. If `Evidence.Confidence` is
`"estimated"` (or `"cached"` near a `critical` band), the pick is NOT
quota-safe: re-fetch a live snapshot (`provider-usage`) before dispatching
to a critical-adjacent provider.

## Checklist

- [ ] Check `usage_enabled` first; if false, defer to `model-selection` and record the pick as score-only.
- [ ] Choose a strategy from the table by the actual operational constraint, not habit.
- [ ] For `weighted-random`, pass `--seed` for any evidence-bearing dispatch.
- [ ] Run `which-model pick --json`; do not dispatch before confirming exit 0.
- [ ] Run `which-model explain --json` for the chosen candidate and record its full `Evidence` object.
- [ ] Confirm `Evidence.Confidence != "estimated"` before treating the pick as quota-safe under a `critical`-band provider.
- [ ] Never treat a `band_gated` excluded candidate as available; do not retry it without a fresh usage snapshot.
