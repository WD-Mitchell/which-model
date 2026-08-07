---
kind: feature-spec
version: "1.0"
feature: F20-strategies
project: which-model
---

# F20 — Strategies: SPEC

## 1. Purpose

F20 implements the six `which-model pick` strategies — `score`, `priority`, `round-robin`, `least-used`, `weighted-random`, `cost-optimal` — in the package `internal/pick/strategy`. Each strategy selects one `pick.Candidate` from the eligible candidate set and reports the candidates it passed over. The package is a pure library: all invocation context (profile name, data directory, provider priority order, usage state, pressure and cost data) arrives through a `State` struct, so every strategy except `round-robin` and `weighted-random` is a pure function of its inputs and the two stateful ones have fully specified, testable state semantics.

## 2. Behaviour

Input contract: the `candidates` slice passed to `Pick` contains only **eligible** (non-gated) candidates. Gating (`band_gated`) and the other upstream exclusions (`no_score_row`, `auth_required`, `provider_error`, `not_in_availability_list`) are produced by the pick assembly layer (F19/F26, reason codes per `docs/plan/annex-c-agent-integration.md` §4.2) and never reach a strategy. `excluded` in the return is the set of eligible candidates the strategy did not select.

### 2.1 Determinism and input ordering

1. Every strategy sorts a copy of its input by route key ascending (decision D6) before any comparison or sampling, so results are independent of caller input order (`docs/plan/annex-d-cli-reference.md` §3: `score`, `priority`, `least-used`, `cost-optimal` are "deterministic"; `round-robin` deterministic given the cursor; `weighted-random` deterministic given `--seed`).
2. `excluded` is returned in route-key ascending order and contains exactly the input candidates minus the pick. Ordering is stable and documented so `which-model explain` evidence is reproducible.

### 2.2 `score`

3. Pick the candidate with the maximum `FinalScore` (`docs/plan/README.md` §5.4 table row `score`). Ties are broken deterministically (decision D1): higher `FinalScore` first, then lower route key (provider ID, then model ID, then reasoning — byte-wise string comparison), which implements "higher score → lower provider ID → lower model ID". `score` is the default strategy (`strategy.default = "score"`, `docs/plan/annex-d-cli-reference.md` §4.2).

### 2.3 `priority`

4. Providers are considered in `State.ProviderPriority` order — the config-ordered list built from `[providers.<id>] priority` (higher = preferred, `docs/plan/annex-d-cli-reference.md` §4.2). The first provider in that order that has at least one candidate wins; within the winning provider, the maximum `FinalScore` candidate is picked (tie-break D1). Providers absent from the list sort after listed providers, by provider ID ascending (decision D9). An empty list degenerates to provider-ID ascending order (`docs/plan/README.md` §5.4 row `priority`; "first provider with capacity" becomes "first provider with any candidate" under degraded mode, `docs/plan/annex-d-cli-reference.md` §3.3).

### 2.4 `round-robin` and the state file

5. Rotation is over the sorted candidate list using a persisted cursor guarded by an exclusive advisory file lock (`docs/plan/README.md` §5.4 row `round-robin`; `docs/plan/annex-d-cli-reference.md` §3.1). The lock library is `github.com/gofrs/flock` (decision D2).
6. State file path: `<DataDir>/pick/round_robin.json` where `DataDir` is the resolved state directory (`docs/plan/annex-d-cli-reference.md` §4.5: Linux `$XDG_STATE_HOME/which-model`, macOS `~/Library/Application Support/which-model/state`). Parent directories are created with mode `0700`, the file with `0600` (decision D7).
7. Cursor scope: the file is a JSON object keyed by `scope_key = hex(sha256(profile + "|" + strings.Join(sortedRouteKeys, "|")))[:16]` where `profile` is `State.Profile` and route keys are the `RouteKey` of every input candidate, sorted ascending (decision D6; `docs/plan/annex-d-cli-reference.md` §3.1 — distinct filter/profile combinations keep independent cursors).
8. Pick protocol (read-modify-write under the lock, exactly `docs/plan/annex-d-cli-reference.md` §3.1 steps 1-7): open the state file, take the exclusive lock (blocking `Lock()`, no timeout — decision D8), read the cursor for the scope key (default `0` if absent), compute `candidate = candidates[index % len(candidates)]`, write `index+1` back with `updated_at` = RFC 3339 UTC now, fsync, release the lock. The stored `index` is the zero-based index of the **next** pick (decision D5). N concurrent picks therefore get different candidates, and two concurrent picks over a two-candidate set cover both.
9. A missing or corrupt/unreadable state file is treated as an empty cursor set (index `0` for every scope), never a fatal error — round-robin degrades gracefully (`docs/plan/annex-d-cli-reference.md` §3.1).
10. `State.DryRun = true` suppresses cursor advancement: the pick is computed from the current cursor but `index+1` is not written (`docs/plan/annex-d-cli-reference.md` §2.4 `--dry-run`).

### 2.5 `least-used`

11. Pick the candidate whose provider has the minimum pressure (`State.PressureByProvider`, per-provider max over the provider's windows — "max, not mean", `docs/plan/README.md` §5.1); ties are broken by higher `FinalScore`, then tie-break D1 (`docs/plan/README.md` §5.4 row `least-used`).
12. When `State.UsageEnabled` is false — usage disabled at ANY level (`--no-usage`, `[usage] enabled = false`, `nousage` build, or `auto` with zero providers enabled) — `least-used` **refuses** with `ErrLeastUsedRequiresUsage`; it never silently falls back to another strategy (`docs/plan/README.md` §6.4 row `least-used`; `docs/plan/annex-d-cli-reference.md` §3.3). The exact error message is `least-used requires usage data; usage is disabled by <source>` where `<source>` maps from `usage_disabled_reason` (decision D11, values from `specs/global/CONTRACTS.md` §6): `flag` → `--no-usage`, `config` → `[usage] enabled = false`, `compiled_out` → `nousage build`, `no_providers_enabled` → `no providers enabled`.
13. When usage is enabled but a candidate's provider has no pressure entry, `Pick` fails with `ErrMissingPressure` — a data-integrity error, never a fallback (decision D12).

### 2.6 `weighted-random`

14. Sampling is weighted per candidate by `BandWeight × ProviderWeight` (both fields on `pick.Candidate`; decision D13). In degraded mode `BandWeight` is `1.0`, so the weight reduces to `ProviderWeight`, which still applies (`docs/plan/README.md` §6.3). The PRNG is `math/rand/v2` with `rand.NewPCG(seed, seed)` (decision D3, `docs/plan/annex-d-cli-reference.md` §3.2).
15. `--seed` is mandatory: `State.HasSeed = false` makes `Pick` return `ErrSeedRequired` with the exact message `weighted-random requires --seed for reproducibility` (`docs/plan/annex-d-cli-reference.md` §3.2; the CLI maps this to exit `2`). A seeded run is a pure function of `(seed, sorted candidates, weights)` — no state file.
16. If the total weight is zero, sampling falls back to uniform over the sorted candidates (decision D14). `--dry-run` has no effect for this strategy (`docs/plan/annex-d-cli-reference.md` §3.2).

### 2.7 `cost-optimal`

17. Among candidates within `State.Config.ResolvedCostMaxScoreDrop()` FinalScore points of the maximum `FinalScore` in the input, pick the candidate with the maximum cost score from `State.CostScoreByRouteKey` (higher = cheaper, decision D15); ties are broken by higher `FinalScore`, then tie-break D1 (`docs/plan/README.md` §5.4 row `cost-optimal`).
18. Config key `strategy.cost_max_score_drop` (decision D4, default `5.0` FinalScore points, matching the plan's `--score-tolerance` default of 5). A candidate with no cost-score entry is excluded, never fatal (decision D16). Cost is a static catalog metric, so the strategy works identically under degraded mode (`docs/plan/annex-d-cli-reference.md` §3.3).

### 2.8 Registry and degraded availability

19. `New(s pick.Strategy)` returns the implementation for the enum values of `pick.Strategy` (`specs/global/CONTRACTS.md` §4.2) and `ParseStrategy` maps the `strategy.default` config string (`""` → `score`); an unknown value yields an error wrapping `ErrUnknownStrategy` (decision D17).
20. Under degraded mode every strategy except `least-used` works (`docs/plan/annex-d-cli-reference.md` §3.3 table): `score` unchanged; `priority` uses static priority; `round-robin` still rotates and still advances its cursor; `weighted-random` samples with `BandWeight = 1.0`; `cost-optimal` uses static cost. The package has no `internal/usage` import and compiles under `-tags nousage` (`specs/global/CONTRACTS.md` §8; `docs/plan/annex-b-catalog-port.md` §0).

## 3. Error behaviour

| Error | Message | Caller exit code (F26) | Condition |
|---|---|---|---|
| `ErrNoCandidates` (sentinel) | `no candidates to pick from` | 3 | empty input to any strategy |
| `ErrSeedRequired` (sentinel) | `weighted-random requires --seed for reproducibility` | 2 | weighted-random with `HasSeed = false` |
| `ErrUnknownStrategy` (sentinel, wrapped: `fmt.Errorf("%w: %q", ErrUnknownStrategy, s)`) | `unknown strategy (valid: score, priority, round-robin, least-used, weighted-random, cost-optimal): "<s>"` | 2 | `New`/`ParseStrategy` with an unrecognized value |
| `ErrLeastUsedRequiresUsage{Reason string}` | `least-used requires usage data; usage is disabled by <source>` | 2 | least-used with `UsageEnabled = false` (any disable level) |
| `ErrMissingPressure{Provider string}` | `no usage pressure data for provider "<provider>"` | 1 | least-used enabled but pressure missing |

Exit-code mapping is F26's; the table is the contract (`specs/global/SPEC.md` §5).

## 4. Decisions

| # | Decision | Value | Rationale |
|---|---|---|---|
| D1 | Score tie-break order | `FinalScore` desc, then route key asc (`provider_id`, then `model_id`, then `reasoning`, byte-wise) | Assignment: "higher score → lower provider ID → lower model ID"; the full route key extends it deterministically to identical providers/models and keeps results order-independent |
| D2 | Lock library | `github.com/gofrs/flock`, blocking `Lock()` | Assignment requirement; same OS primitive as annex-d §3.1's `unix.Flock` but portable (Windows) and with a testable API; blocking lock matches annex-d's at-most-one-writer semantics |
| D3 | Weighted-random PRNG | `math/rand/v2`, `rand.NewPCG(seed, seed)` | Annex-d §3.2 names it explicitly; platform-independent so seeded results are stable across CI hosts |
| D4 | Cost threshold default | `strategy.cost_max_score_drop = 5.0` (FinalScore points); zero state value ⇒ 5.0 | Matches master plan §5.4 `--score-tolerance` default 5; zero ⇒ unset ⇒ default keeps fresh installs sensible |
| D5 | Cursor semantics | Stored `index` = zero-based index of the next pick; default 0; write `index+1` after each pick | Annex-d §3.1 verbatim (candidate = `candidates[index % len]`) |
| D6 | Route key | `provider + "/" + model_id + "/" + reasoning`; scope key = `hex(sha256(profile + "|" + join(sortedRouteKeys, "|")))[:16]` | Annex-d §3.1 names the scope key but not the route key; this form is unique per routed candidate and stable across runs |
| D7 | State file perms | dirs `0700`, file `0600`; created on demand | Least-privilege; the file contains no secrets but there is no reason to widen it |
| D8 | Lock contention | Blocking exclusive lock, no timeout | Annex-d §3.1: at-most-one writer; a stuck holder blocks picks rather than double-advancing the cursor |
| D9 | Priority ordering | Descending configured `priority`; ties by provider ID ascending; unlisted providers after listed, by ID ascending | Annex-d §4.2 "higher = preferred"; unlisted/tic rules close the determinism gap the plan leaves open |
| D10 | Input normalization | All strategies sort a copy by route key ascending before any logic; `excluded` returned route-key ascending | Deterministic output regardless of caller order; reproducible `explain` evidence |
| D11 | Refusal source text | `flag`→`--no-usage`; `config`→`[usage] enabled = false`; `compiled_out`→`nousage build`; `no_providers_enabled`→`no providers enabled` | Annex-d §3.3 requires the refusal to name the disabling lever; the first two match its worked examples verbatim |
| D12 | Enabled-but-missing pressure | Hard error `ErrMissingPressure` | Fail loud: an enabled least-used pick without pressure is a wiring bug, not a policy choice |
| D13 | Weighted-random weights | Per-candidate `BandWeight × ProviderWeight` | Assignment: "weights = BandWeight × ProviderWeight"; refines plan §5.4's "∝ FinalScore" (model score already enters via eligibility and banding; sampling then expresses consumption + user preference only) |
| D14 | Zero total weight | Uniform over sorted candidates | Degenerate config (all weights 0) must not panic; uniform is the least-surprise fallback |
| D15 | Cost score direction | `CostScoreByRouteKey` holds the catalog cost score (0–100, higher = cheaper); pick the maximum | Plan §5.4 "take the best cost score"; same scale as `ModelScore`, so threshold arithmetic is uniform |
| D16 | Missing cost entry | Candidate excluded (listed in `excluded`), never fatal | A routable model without a cost score is a data gap, not a reason to abort the pick |
| D17 | Unknown strategy | `ErrUnknownStrategy` sentinel, wrapped with the offending value | Annex-d §1.4 unknown-flag rejection philosophy: loud, exit 2, names the value |

## 5. Out of scope

- Pressure derivation, band evaluation, gating, and `FinalScore` math — F19 (`internal/pick/band`).
- Candidate assembly, `band_gated`/upstream exclusions, `usage_enabled`/`usage_disabled_reason` output envelope, exit-code mapping, `--strategy`/`--seed`/`--dry-run` flags — F26 (`internal/pick` command) and F03 (`internal/output`).
- Usage fetch, toggle resolution (`ResolveUsageEnabled`), and `nousage` stubs — F14, F21.
- Config file loading and validation — F01.
- Scoring profiles and tier1/tier2 combination — F10 (`internal/pick` ranking).
