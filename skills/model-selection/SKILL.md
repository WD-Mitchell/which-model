---
name: model-selection
description: >-
  Use when choosing a verified model and reasoning-effort row for a dispatched
  task. Trigger when a task needs a deterministic model ranking from a task
  profile, explicit tier weights, or live target-harness availability
  filtering before dispatch.
---

# Model selection: ranked scores plus live availability

Choose an exact model and reasoning-effort row from the scored catalog, then
confirm the row is accepted by the target dispatch harness before
dispatching. The score artifact is a data-driven prior; it cannot prove that
the target harness exposes a row. This skill works unchanged when the usage
subsystem is disabled (annex-c §2.5); it never reads band or quota fields.

## When to use

- A task needs a deterministic model ranking from a task profile.
- Explicit tier weights (`--tier1-weight`, `--tier2-weight`, `--weights-json`)
  are needed instead of a named profile.
- The dispatch target is a specific harness whose live availability must be
  applied before committing to a model+effort pair.

## Commands

Rank a task against one of the 11 profiles (`simple_implementation`,
`simple_action_execution`, `balanced_implementation`, `complex_implementation`,
`ui_ux`, `complex_action_execution`, `financial_work`, `research`, `planning`,
`orchestration`, `review`):

```bash
which-model pick --profile balanced_implementation --top 5 --json
```

Filter the ranking by the target harness's live availability file (JSON list
of `"model|reasoning"` strings or `{"model": ..., "reasoning": ...}` objects):

```bash
which-model pick --profile simple_action_execution --top 5 --available .tmp/live-model-efforts.txt --json
```

Record dispatch evidence for the chosen row:

```bash
which-model explain <candidate-id> --json
```

## Reading the output

Check `usage_enabled` FIRST on every `--json` output (annex-c §4.6). This
skill never reads band fields, so a `false` value does not change these
steps — but record it in the evidence.

From `which-model pick --json` read:

- `candidates[0].candidate_id` — the id to pass to `which-model explain`.
- `candidates[0].route.provider`, `.route.model_id`, `.route.model`,
  `.route.reasoning` — the exact row.
- `candidates[0].final_score`, `candidates[0].warnings`.
- `excluded_candidates[].reason_code` and `.reason` — inspect BEFORE
  trusting the recommendation. `reason_code: "not_in_availability_list"`
  means the harness availability filter removed the row.

From `which-model explain <candidate-id> --json` read `evidence`:

- `evidence.profile`, `evidence.score_inputs`.
- `evidence.excluded_candidates`, `evidence.route_provenance`,
  `evidence.last_verified` (`last_verified` is omitted when usage is off).
- `evidence.band`, `evidence.snapshot_age_seconds`, `evidence.confidence` —
  present only when `usage_enabled` is true; never read them when it is false.

## Failure handling (exit codes, annex-c §4.7)

| exit | meaning | action |
|---|---|---|
| 0 | success | parse `--json` per the shapes above and proceed |
| 1 | runtime error | surface `Failure.message`; do not retry blindly |
| 2 | argument/usage error | fix the invocation (bad profile, bad flag combination); do not retry unchanged |
| 3 | no viable candidate after filtering | widen the profile or `--available` list; ask the user; never silently fall back to an unranked model |
| 4 | all eligible providers band-gated | usage signal; defer to `usage-aware-dispatch`/`provider-usage` |
| 5 | authentication required | run the explicit, user-present login flow (`provider-usage`) |

## Recording evidence

Record the EXACT model ID and reasoning effort accepted by the target
harness and the full `Evidence` object from `which-model explain` — never a
free-text availability claim, never a model name or slug alone. Model names
are not availability proof. If `usage_enabled` is false, record
`usage_disabled_reason` with the evidence; degraded evidence is still valid
evidence for the claim it makes — "this is the best-scoring model for this
profile" (annex-c §5.1).

## Checklist

- [ ] Classify task intent and complexity; choose the closest of the 11 profiles.
- [ ] Confirm intelligence, cost, and speed weights are present and positive in the run's output.
- [ ] Run `which-model pick --json` and inspect `warnings`/`excluded_candidates` before trusting `candidates[0]`.
- [ ] Apply target-harness live availability (`--available`) with exact model + reasoning-effort IDs.
- [ ] Dispatch the exact recommended row or a listed alternative; never invent a nearby effort as a fallback.
- [ ] Record model, reasoning effort, profile, `usage_enabled`, and the full `Evidence` object.
