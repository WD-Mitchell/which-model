---
kind: feature-spec
version: "1.1"
feature: F20-strategies
project: which-model
---

# F20 — Strategies: SPEC

## 1. Purpose

F20 implements five `which-model pick` strategies in `internal/pick/strategy`: `priority`, `round-robin`, `least-used`, `most-used`, and `closest-to-reset`. Each strategy selects one eligible `pick.Candidate` and returns the candidates it passed over.

`priority` is the default. The removed `score`, `weighted-random`, and `cost-optimal` names are invalid and must return `ErrUnknownStrategy`.

## 2. Shared behaviour

1. Input contains only eligible candidates; routing, score-row, authentication, provider, and band-gating exclusions happen before strategy selection.
2. Empty input returns `ErrNoCandidates`.
3. Strategies never mutate the caller's candidate slice.
4. Ties use higher `FinalScore`, then route key ascending. Route key is `provider + "/" + model_id + "/" + reasoning`.
5. Returned exclusions contain exactly the input minus the selected occurrence and are sorted by route key.

## 3. Strategies

### 3.1 `priority`

Providers are considered in `State.ProviderPriority` order. The first provider with a candidate wins; within it, the highest `FinalScore` wins. Providers absent from the configured list follow in provider-ID order. An empty priority list therefore selects the alphabetically first provider.

### 3.2 `round-robin`

Rotation uses the route-key-sorted candidate list and the persisted cursor at `<State.DataDir>/pick/round_robin.json`. The file is guarded by `github.com/gofrs/flock`, directories use `0700`, and the file uses `0600`. `State.DryRun` reads but does not advance the cursor.

### 3.3 `least-used`

Select the provider with the lowest value in `State.PressureByProvider`. Pressure is the maximum relevant-window used percent computed by F26. Usage disabled returns `ErrLeastUsedRequiresUsage`; missing pressure returns `ErrMissingPressure`.

### 3.4 `most-used`

Select the provider with the highest value in `State.PressureByProvider`. This drains the provider with the least allowance remaining. Usage disabled returns `ErrMostUsedRequiresUsage`; missing pressure returns `ErrMissingPressure`.

### 3.5 `closest-to-reset`

Select the provider with the earliest timestamp in `State.ResetAtByProvider`. F26 supplies each provider's earliest non-zero `Window.ResetsAt` value from its usage snapshot. Usage disabled returns `ErrClosestToResetRequiresUsage`; a provider without reset metadata returns `ErrMissingReset`.

## 4. Registry and defaults

`ParseStrategy("")` returns `pick.StrategyPriority`. `New` and `ParseStrategy` accept exactly the five canonical hyphenated strategy names. Unknown and removed names wrap `ErrUnknownStrategy`.

Only `priority` and `round-robin` work when usage is disabled. `least-used`, `most-used`, and `closest-to-reset` refuse rather than silently degrading.

## 5. Error behaviour

| Error | Message | Condition |
|---|---|---|
| `ErrNoCandidates` | `no candidates to pick from` | empty input |
| `ErrUnknownStrategy` | `unknown strategy (valid: priority, round-robin, least-used, most-used, closest-to-reset)` | unknown or removed strategy |
| `ErrLeastUsedRequiresUsage` | `least-used requires usage data; usage is disabled by <source>` | usage disabled |
| `ErrMostUsedRequiresUsage` | `most-used requires usage data; usage is disabled by <source>` | usage disabled |
| `ErrClosestToResetRequiresUsage` | `closest-to-reset requires usage data; usage is disabled by <source>` | usage disabled |
| `ErrMissingPressure` | `no usage pressure data for provider "<provider>"` | pressure missing |
| `ErrMissingReset` | `no usage reset data for provider "<provider>"` | reset timestamp missing |

## 6. Out of scope

- Candidate scoring and `FinalScore` calculation — F10/F19.
- Usage fetching and normalization — F14 and provider adapters.
- CLI rendering, history, and exit-code mapping — F26.
