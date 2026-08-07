---
kind: feature-spec
version: "1.0"
feature: F26-cmd-pick
project: which-model
---

# F26 — cmd-pick: SPEC

## 1. Purpose

`which-model pick` selects a model for a task: it scores every routed candidate for a profile, applies usage-aware band gating, and returns the single best choice (or a weighted-random draw) as both text and structured JSON, with an audit-trail `explain` subcommand that reproduces the selection's evidence. `pick` is the command agents call inside their loops (annex-c §2), so its JSON contract — field names, enum values, and evidence shape — is fixed verbatim from annex-c §4.2/§4.3/§4.6/§5 and is the load-bearing contract of this feature. The exit code reports *why* nothing was picked, distinguishing auth failures (exit 5) from gating failures (exit 4) from score-only exclusions (exit 3). Under usage-disabled config the command degrades deterministically instead of refusing.

## 2. Behaviour

### 2.1 Profile selection (flags)

1. `--profile <name>` is REQUIRED unless `--task-category` is given; the value must be one of the eleven annex-c §2.1 profile names (verbatim): `simple_implementation`, `simple_action_execution`, `balanced_implementation`, `complex_implementation`, `ui_ux`, `complex_action_execution`, `financial_work`, `research`, `planning`, `orchestration`, `review`. Unknown name → exit 2 listing valid profiles. (Source: annex-c §2.1; annex-d §2.4; Decision D-1.)
2. `--task-category <category>` + `--complexity <level>` are an alternative profile selector, mutually exclusive with `--profile`; both must be given together (one without the other → exit 2). Mapping (Decision D-2): `(implementation, simple) → simple_implementation`, `(implementation, medium) → balanced_implementation`, `(implementation, complex) → complex_implementation`, `(action_execution, simple) → simple_action_execution`, `(action_execution, medium) → balanced_implementation`, `(action_execution, complex) → complex_action_execution`, and for `ui_ux`, `financial_work`, `research`, `planning`, `orchestration`, `review` the category maps to the same-named profile and `--complexity` is REJECTED (exit 2). Unknown category/complexity → exit 2. (Source: annex-c §2.1; Decision D-2.)
3. `--strategy <name>` selects the strategy; default `score`. Valid strategies are the F20 registry's names; unknown → exit 2. (Source: annex-d §2.4; Decisions D-3.)
4. `--seed <int>` is REQUIRED when the resolved strategy is `weighted_random` (determinism, master plan §3); missing → exit 2 with message `--seed is required for strategy "weighted_random"`. Providing `--seed` with any other strategy is allowed but has no effect (annex-d `--seed` flag semantics). (Source: annex-d §2.4; master plan §3; Decision D-4.)
5. `--available <path>` is repeatable; each occurrence restricts the candidate set to routes whose `model_id` (annex-c §4.2 field) is in the given allowlist file (one model id per line, `#` comments, trimmed; empty file = no candidates from that list). Repeated flags union their sets. A missing allowlist file → exit 2 `[arguments]`. (Source: annex-d §2.4; Decision D-5.)

### 2.2 Pipeline

6. The pipeline runs in this order, each stage feeding the next (master plan README §4 routing join; Decisions D-6):
   a. **Score**: for every routed candidate of the resolved profile, compute `model_score` (F10 scoring, decimal `Round(0)` per global CONTRACTS §5).
   b. **Filter**: drop candidates whose model is not in any `--available` list (when given); dropped rows are recorded in `excluded_candidates` with `reason_code: "not_in_availability_list"`. Rows with no score row (unknown model/reasoning in the CSV) are dropped with `reason_code: "no_score_row"` and a stderr warning `warning: no score row for <model>/<reasoning>; excluded`.
   c. **Rank**: order survivors by `model_score` descending (F10 rank; ties broken by provider order, then model_id lexical).
   d. **Routes join**: each survivor carries its route's `route` field (provider + model-id) and `provider_weight`; unrouted score rows (a model that scores but has no route) produce a stderr warning and are NOT candidates (they are never placed in `excluded_candidates` because the route field is required there — annex-c §4.2 additionalProperties:false; Decision D-6).
   e. **Usage**: call F14 fetch for each surviving provider (usage disabled → skip stage, see §2.5); snapshots drive band evaluation (F19). Usage-check failures that are auth-class map to `auth_required` (annex-c §4.7: exit 5); non-auth failures map to `provider_error` (Decision D-7).
   f. **Bands**: evaluate each survivor against its band (F19 `band.Evaluate`); `band_gated` survivors are excluded with `reason_code: "band_gated"` and recorded in `excluded_candidates`; the band name/used%/weight are attached to surviving candidates (annex-c §4.2 candidate `band`, `band_weight`).
   g. **Strategy**: apply the F20 strategy over the surviving candidates; `weighted_random` consumes `--seed`; `least_used` in degraded (usage-disabled) mode → exit 2 per master plan §6.4 (never falls back).
7. **Result**: if ≥1 candidate survives the strategy stage, exit 0 and emit the top candidate (single choice) — or the strategy's distribution when the strategy says so — as `PickResult` JSON (annex-c §4.2 verbatim shape) or text. If zero survive, exit per §2.6. (Source: annex-c §4.2, §4.6.)

### 2.3 Output

8. **JSON output** (`--json`, and ALWAYS when stdout is not a TTY — annex-c §4.6 agent mode; Decision D-8): `PickResult` per annex-c §4.2 + §4.6 VERBATIM:
   ```json
   {
     "schema_version": "2.0",
     "usage_enabled": true,
     "usage_disabled_reason": null,
     "profile": "complex_implementation",
     "strategy": "score",
     "seed": null,
     "normalizer": "minmax-linear",
     "aggregator": "weighted-arithmetic-mean",
     "candidates": [
       {
         "candidate_id": "claude:claude-sonnet-4-5",
         "route": {"provider": "claude", "model_id": "claude-sonnet-4-5", "model": "claude-sonnet-4-5", "reasoning": "default", "window_ids": ["5h", "7d"]},
         "model_score": 92,
         "band": "five hour",
         "band_weight": 0.8,
         "provider_weight": 1.0,
         "final_score": 73.6,
         "warnings": []
       }
     ],
     "excluded_candidates": [
       {"route": {"provider": "codex", "model_id": "gpt-5-codex", "model": "gpt-5-codex", "reasoning": "default", "window_ids": []}, "reason_code": "band_gated", "reason": "band usage 95% > gate 90%"}
     ]
   }
   ```
   Field rules (all verbatim annex-c §4.2/§4.6): `schema_version` "2.0"; `usage_disabled_reason` is `null` when usage enabled, a string when disabled (Decision D-9); `seed` null unless `weighted_random`; `route` is the FULL Route object (`provider`, `model_id`, `model`, `reasoning`, `window_ids` — Decision D-17); `band`/`band_weight` OMITTED (not null) when usage disabled (§4.6); `normalizer`/`aggregator` are the F10/`Global` names; numbers are JSON numbers (decimal → float64). `excluded_candidates[]` = `{route, reason_code, reason}` with `reason_code` one of the verbatim enum `band_gated|no_score_row|auth_required|provider_error|not_in_availability_list` (annex-c §4.5: adding values later is a minor change). (Source: annex-c §4.2, §4.5, §4.6; Decisions D-17, D-18.)
9. **Text output** (non-JSON, TTY): `picked <model_id> via <provider> (score <final_score>)\n` followed by reason lines: `  profile: <profile>`, `  strategy: <strategy>`, `  band: <name> (<used_percent>% used, weight <band_weight>)` when usage enabled, `  warnings: <n>` when n>0. No pick → the exit-class message on stderr (no stdout). (Source: annex-d §2.4; Decision D-10.)
10. **History log**: every run appends one line to `<state_dir>/pick/history.jsonl` (F01 `StateDir()`): `{"ulid": "<26-char ULID>", "ts": "<RFC3339>", "profile": ..., "strategy": ..., "seed": ..., "candidate_id": ..., "final_score": ..., "excluded_count": n, "evidence": {...}}` — the evidence object is the full §11 evidence shape (D-13, so `explain` can reconstruct without live state). ULIDs via `github.com/oklog/ulid/v2` (Decision D-11); append-only; write failure → stderr warning `warning: could not write pick history: <err>` (never fails the run — Decision D-12). The file is created lazily on first run.
11. **`explain` subcommand** (NEW command file, same wiring pattern): `which-model explain --last` or `which-model explain --pick-id <ulid>` (exactly one of the two, else exit 2). Reads the history file; `--last` = last line, `--pick-id` = exact ULID match (not found → exit 1 `no record <ulid>`). Emits the `ExplainResult` document per annex-c §4.3 VERBATIM — root `{schema_version, candidate, evidence}`:
    ```json
    {
      "schema_version": "2.0",
      "candidate": "claude:claude-sonnet-4-5",
      "evidence": {
        "profile": "complex_implementation",
        "score_inputs": {"tier1": 84, "category": 8},
        "band": {"name": "five hour", "used_percent": 25, "weight": 0.8},
        "snapshot_age_seconds": 300,
        "confidence": "live",
        "route_provenance": "provider_live",
        "excluded_candidates": [
          {"route": {"provider": "codex", "model_id": "gpt-5-codex", "model": "gpt-5-codex", "reasoning": "default", "window_ids": []}, "reason_code": "band_gated", "reason": "band usage 95% > gate 90%"}
        ],
        "last_verified": "2026-08-07T17:03:11Z"
      }
    }
    ```
    with Evidence required fields exactly `profile, score_inputs, band, snapshot_age_seconds, confidence, route_provenance, excluded_candidates, last_verified` (annex-c §4.3 + §5), `score_inputs` values NUMBERS (F10 tier1+category composites), `confidence ∈ live|cached` (degraded mode omits it — §5.1 omits, never `estimated`), `route_provenance` = the global Provenance enum (`provider_live|models_dev|user_declared`), `excluded_candidates` = full §4.2 ExcludedCandidate objects, `last_verified` = ONE RFC3339 date-time. The Evidence is reconstructed from the recorded history + re-read state at explain time (Decision D-13); by construction every pick writes the full evidence record, so no required field is ever missing at explain time. (Source: annex-c §4.3, §5.)
12. **Determinism**: with the same config, catalog, usage snapshot, and `--seed`, `pick` output is byte-identical across runs (master plan §3; degraded mode included — §6.4).

### 2.4 Usage wiring and degraded mode

13. F26 consumes F21's toggle directly: `toggle.ResolveUsageEnabled(flagNoUsage bool, cfg *config.Config) (bool, string)` with reason strings `flag|config|compiled_out|no_providers_enabled` (F21 contract). The resolved `(enabled, reason)` populates `usage_enabled` and `usage_disabled_reason` in the JSON. (Source: F21 usage-toggle contract; annex-c §4.6.)
14. **Strict no_providers case** (master plan §4.2): when the toggle resolves `reason == "no_providers_enabled"` AND the raw config value is `[usage] enabled = "true"` (UsageTrue), `pick` exits 2 with `which-model pick: [usage_config] usage is enabled but no providers are enabled; set [providers.<id>] enabled = true or [usage] enabled = "auto"` — the misconfiguration must be surfaced, never silently degraded. (Source: master plan §4.2; Decision D-14.)
15. **Degraded mode** (usage disabled via flag/config, L1): usage stage skipped; `usage_enabled: false`; `usage_disabled_reason` set to the toggle reason; candidates carry NO `band`/`band_weight`; `band_gated`/`auth_required`/`provider_error` exclusions cannot occur; `least_used` strategy → exit 2 with `[usage_disabled] strategy "least_used" requires usage data` (never falls back, master plan §6.4); every other strategy works; output must be byte-reproducible. Evidence in degraded mode (annex-c §5.1): `profile`, `score_inputs`, `route_provenance` (never `provider_live` — only `models_dev`/`user_declared`), `excluded_candidates` retained; `band`, `snapshot_age_seconds`, `confidence`, `last_verified` OMITTED. (Source: annex-d §4.6; master plan §6.3, §6.4; annex-c §5.1.)

### 2.5 Exit codes

16. Exit classification when zero candidates survive (precedence high → low; Decision D-15):
    - any `auth_required` exclusion → exit 5 (`[no_pick] auth required; run which-model auth status`),
    - else any `band_gated` or `provider_error` exclusion → exit 4 (`[no_pick] usage gating excluded every candidate`),
    - else (only `not_in_availability_list`/`no_score_row`) → exit 3 (`[no_pick] no candidate matched the request`).
    - Any ≥1 survivor → exit 0. Runtime errors (fetch/read failures outside the exit classes) → exit 1. Argument errors → exit 2. Usage-disabled + `least_used` → exit 2. Strict `no_providers` misconfiguration → exit 2 `usage_config`. (Source: annex-d §2.4 exit mapping; annex-c §4.7; master plan §4.2.)
17. Exit signalling via the F22 exit contract (F26 CONTRACTS §8.1): `RunE` returns the F22 error types; the failure line `which-model pick: [<code>] <message>` is rendered by F22. (Source: F22 contract; Decisions D-16.)

## 3. Error behaviour

| Condition | Exit | stdout / message |
|---|---|---|
| `--profile` missing and no `--task-category` | 2 | `[arguments] --profile or --task-category is required` |
| Unknown `--profile` | 2 | `[arguments] unknown profile "x"; valid: <11 names>` |
| `--task-category` without `--complexity` (or vice versa) | 2 | `[arguments] --task-category and --complexity must be given together` |
| Unknown category / complexity value | 2 | `[arguments] unknown task category "x"` / `unknown complexity "x"` |
| `--complexity` with category that maps 1:1 (ui_ux etc.) | 2 | `[arguments] --complexity is not valid for task category "ui_ux"` |
| Unknown `--strategy` | 2 | `[arguments] unknown strategy "x"; valid: <F20 names>` |
| `--seed` missing for `weighted_random` | 2 | `[arguments] --seed is required for strategy "weighted_random"` |
| `--available` file missing | 2 | `[arguments] allowlist file "x" not found` |
| `--last` and `--pick-id` both/neither (explain) | 2 | `[arguments] exactly one of --last or --pick-id is required` |
| `--pick-id` not found (explain) | 1 | `[no_record] no record <ulid>` |
| Strict `no_providers` misconfig | 2 | `[usage_config] usage is enabled but no providers are enabled; set [providers.<id>] enabled = true or [usage] enabled = "auto"` |
| `least_used` + usage disabled | 2 | `[usage_disabled] strategy "least_used" requires usage data` |
| All excluded, any auth_required | 5 | `[no_pick] auth required; run which-model auth status` |
| All excluded, any band_gated/provider_error | 4 | `[no_pick] usage gating excluded every candidate` |
| All excluded, only availability/score | 3 | `[no_pick] no candidate matched the request` |
| Unrouted score row | — | stderr warning `warning: no route for score row <model>/<reasoning>; ignored` (no exit effect) |
| No score row | — | stderr warning `warning: no score row for <model>/<reasoning>; excluded` |

## 4. Decisions

| Decision | Value | Rationale |
|---|---|---|
| D-1 | `--profile` is required (assignment contract), validated against the exact 11 annex-c §2.1 names | The assignment pins profile as required; the plan's default (`balanced_implementation`) is rejected because an explicit contract beats a default |
| D-2 | `--task-category`/`--complexity` mapping table (7 rows) with hard rejection of `--complexity` on 1:1 categories | Deterministic, testable mapping; avoids inventing profile semantics |
| D-3 | Strategy names come from the F20 registry at runtime; `score` is the default | F20 owns strategy availability (master plan §6.4); F26 never hardcodes the registry |
| D-4 | `--seed` required iff `weighted_random` | Determinism master plan §3; other strategies are deterministic without it |
| D-5 | `--available` = union of allowlist files, `not_in_availability_list` exclusion | Matches annex-c §4.2's existing reason_code; file-based so agents can write the list once |
| D-6 | Unrouted score rows are stderr warnings, never `excluded_candidates` entries | annex-c §4.2 requires `route` on candidates and `additionalProperties:false`; a "route-less" row cannot be represented, and warning beats silent drop |
| D-7 | Usage-check failures map: auth-class → `auth_required` (exit 5), else → `provider_error` | annex-c §4.7 routes auth failures to exit 5; the two reason codes cover both classes |
| D-8 | Non-TTY stdout forces JSON (annex-c §4.6 agent mode) | Agents consume structured output; TTY keeps human text |
| D-9 | `usage_disabled_reason` = `null` when enabled, toggle reason string when disabled | §4.6 schema keeps the key always present; null is the honest enabled value |
| D-10 | Text layout = `picked <model> via <provider> (score <n>)` + 2-space reason lines | annex-d §2.4's example shape, golden-tested |
| D-11 | ULID (`github.com/oklog/ulid/v2`) for history ids | Sortable, unique, dependency-graph-approved; ULID beats UUID for `--last` + chronological explain |
| D-12 | History write failure = stderr warning, exit unaffected | A read-only agent run must not fail because the state dir is unwritable |
| D-13 | Evidence reconstructed at explain time from history + current state; full evidence record is written per pick so reconstruction is total | annex-c §4.3 needs live-ish fields (`snapshot_age_seconds`, `confidence`) that change after the pick; recording everything at pick time keeps explain byte-stable |
| D-14 | Strict refusal on `usage.enabled=true` + zero providers (exit 2 `usage_config`), never silent degradation | master plan §4.2: the config is contradictory; degrade-then-succeed would mask a broken setup |
| D-15 | Exit precedence 5 > 4 > 3 for zero-survivor classes | The classes are not disjoint; the agent's most actionable signal is auth, then gating, then availability |
| D-16 | Exit signalling purely via F22 error types; F26 never calls `os.Exit` | Single rendering point in F22 keeps the failure line format consistent |
| D-17 | `Candidate.route` and `ExcludedCandidate.route` are the full Route object `{provider, model_id, model, reasoning, window_ids}` (annex-c §4.2 `$defs/Route`, additionalProperties:false), built from the F18 route (`window_ids` ← route windows, `reasoning` passes through, default `default`) | Annex-c §4.2 requires `route: {$ref: Route}` and `required: [..., route, ...]` on both defs; a bare provider string would not validate |
| D-18 | The §4.6 degraded example's `recommendation`/`alternatives`/`excluded` field names are superseded: the 2.0 pick root keeps §4.2's `candidates`/`excluded_candidates` and adds only §4.6's normative `usage_enabled`/`usage_disabled_reason` (plus `normalizer`/`aggregator` per the same example) | §4.6's normative text says usage_enabled is "added to the root object" of the §4.2 output and defines omissions by field absence; the example's renamed containers conflict with §4.2's required arrays and are treated as draft drift — the schema `required` lists in §4.2 are the load-bearing contract |
| D-19 | Strategy names come from F20's registry (`strategy.Names()`), e.g. `score`, `least_used`, `weighted_random`; the §4.2 strategy enum's hyphenated spellings (`least-used`, `weighted-random`) are superseded | F20 owns strategy availability (master plan §6.4); one registry avoids two spellings of the same strategy |
| D-20 | Evidence `excluded_candidates` holds full `ExcludedCandidate` objects (annex-c §4.3 refs the pick-result `$defs/ExcludedCandidate`) and `last_verified` is ONE RFC3339 string (the picked provider's last verification), not a per-provider map | Annex-c §4.3's schema is explicit on both; the record's single pick needs only the picked provider's timestamp |

## 5. Out of scope

- `--top N`, `--weights-json`, `--tier`/band-config flags, `--provider`/`--exclude-provider` filters, `--max-used-percent`, `--require-live`, `--dry-run` (annex-d §2.4 flags not in this feature's contract; F19/F20 own band/strategy configuration).
- Scoring math: F10 owns `model_score`, `normalizer`, `aggregator`; F26 only consumes.
- Band definitions: F19 owns band config and `band.Evaluate`; F26 consumes.
- Strategy internals: F20 owns the registry and `strategy.Apply`; F26 consumes.
- Usage fetching/caching: F14 owns fetch and cache; F26 consumes `fetch.FetchAll` and the last-verified map.
- Credential/device flows: F12/F25 own; `pick` only reports `auth_required`.
- The F21 toggle internals and the `nousage` stubs: F21 owns; F26 consumes `toggle.ResolveUsageEnabled` and compiles under `-tags nousage`.
- Route CRUD: F27 (`routes` command) owns; F26 only joins existing routes.
- History rotation/cleanup policies beyond append-only.
