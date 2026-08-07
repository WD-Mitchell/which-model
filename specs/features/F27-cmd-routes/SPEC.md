---
kind: feature-spec
version: "1.0"
feature: F27-cmd-routes
project: which-model
---

# F27 — cmd-routes: SPEC

## 1. Purpose

`which-model routes` manages the route table that connects provider+model-id pairs to scored models: `list` inspects it, `add`/`remove` edit it (user-declared routes), `refresh` regenerates the machine-produced portion (overrides preserve user declarations), and `verify` audits the table against the current scores CSV so stale routes surface before agents trust them. The command is the operational surface for F18's routing state (annex-d §2.6): routes are how `pick` knows which scored model a provider alias serves, so `verify` is the guard that keeps `pick`'s join honest.

## 2. Behaviour

1. **Command shape.** `which-model routes list|add|remove|refresh|verify` as five subcommands of one self-registered command (F22 wiring; `pkg/whichmodel/routes_cmd.go`). Route state lives at `<state_dir>/routes.json` (F18-owned format; F27 consumes `routing.LoadRoutes`/`SaveRoutes`/`ProduceRoutes`/`RoutesPath`). (Source: annex-d §2.6; Decisions D-1.)

2. **add.** `routes add --provider <id> --model-id <model_id> --model <scored-model> [--reasoning default] [--window <id>...]`. Validation: `--provider` must be a registry id (unknown → exit 2, `valid providers:` list); `--model-id` and `--model` non-empty (exit 2); `--reasoning` defaults to `default`; duplicate route (same provider + model-id) → exit 2 with `route <provider>:<model-id> already exists` (use `remove` first). The added route is written with `Provenance = "user_declared"` (global Provenance enum) and replaces no other route. Success → nothing on stdout, exit 0 (the change is visible in `list`). (Source: annex-d §2.6; Decisions D-2.)

3. **remove.** `routes remove --provider <id> --model-id <model_id>`: exact-match removal; no such route → exit 1 with `[no_route] no route <provider>:<model_id>` (no wildcards); success → silent, exit 0. Removing a route whose provenance is not `user_declared` is allowed (user takes ownership of the machine route's removal — the next `refresh` may recreate it). (Source: annex-d §2.6; Decisions D-3.)

4. **list.** `routes list [--provider <id>] [--json]`. Text: `text/tabwriter` (padding 2) with header `provider  model_id  model  reasoning  windows  provenance` and one row per route; `windows` = comma-joined window ids (empty → `-`); routes in file order (load order). `--provider` filters to that provider (unknown provider → exit 2). `--json` → `RouteList` document: `{schema_version: "2.0", routes: [...]}` where each route echoes the canonical `routing.Route` JSON tags. Missing `routes.json` → empty table/list, exit 0 (first run; nothing to list). IO error → exit 1. (Source: annex-d §2.6; Decisions D-4.)

5. **refresh.** `routes refresh [--auto <fuzzy-name>]`: calls F18's production pipeline (`routing.ProduceRoutes(cfg)`) and writes the result to the routes file. User-declared routes are preserved across refresh (production output never removes them; where production and user routes collide on (provider, model-id), the user route wins — F18's contract for merging; F27 simply persists what ProduceRoutes returns). Idempotent: refresh when nothing changed writes the same file bytes (compare-and-skip, exit 0). `--auto <fuzzy-name>`: after production, also add a user_declared route whose `model` fuzzy-matches the given name against available score rows (`--auto claude-sonnet` picks the closest row by substring/prefix; zero or ambiguous matches → exit 2 with `[arguments] no score row matching "x"`). (Source: annex-d §2.6; Decisions D-5.)

6. **refresh under usage-disabled.** Under L0/L1, refresh runs with reduced sources (no live provider inventory — only the static catalog) and emits exactly ONE stderr warning `warning: usage is disabled; refresh uses static sources only` (annex-d §4.6); the command still succeeds. L2 is not registered at all? NO — routes is NOT usage-touching in the toggle sense: it must still run; `routes` is registered in all builds (no build tag), consistent with `pick`. The reduced-source rule comes from F18's own behavior; F27's only duty is the warning when the toggle resolves disabled. (Source: annex-d §4.6; Decisions D-6.)

7. **verify.** `routes verify [--json]` audits the routes file against the current scores CSV (`catalog.scores_csv_path`, read via F06 `csvstore.ReadScores`):
   - Stale route = its `model` (or `model`+`reasoning`) no longer resolves to a score row → printed on stdout, one per line: `stale route <provider>:<model-id> (<model>/<reasoning>)`; exit 1. Zero stale routes → exit 0.
   - Unrouted score rows = score rows with no route → stderr warnings `warning: score row <model>/<reasoning> has no route; it cannot be picked` (no exit effect).
   - Provenance counts: `X user_declared, Y provider_live, Z models_dev` reported as a stderr summary line `routes: <n> total (<x> user_declared, <y> provider_live, <z> models_dev)`.
   - `scores_sha256` (F18-owned metadata): when the stored hash differs from the live CSV hash → stderr warning `warning: scores CSV changed since routes were produced; run which-model routes refresh` (no exit effect).
   - Missing `routes.json` → "empty table" semantics: zero stale routes, exit 0, provenance summary `routes: 0 total (0 user_declared, 0 provider_live, 0 models_dev)`.
   - IO error (unreadable CSV, unreadable routes file) → exit 1.
   - `--json` → `VerifyReport`: `{schema_version: "2.0", stale_routes: [<provider>:<model-id>], unrouted: [{"model": ..., "reasoning": ...}], provenance_counts: {"user_declared": x, "provider_live": y, "models_dev": z}, scores_sha256_matches: true|false}`; exit codes unchanged (report + exit 1 when stale — the report is the primary output; see D-7). (Source: annex-d §2.6; Decisions D-7.)

8. **Exit codes.** `add`/`remove`: 0 success; 2 argument/validation errors; 1 IO error (unreadable/writable routes file). `list`: 0 always except 2 (unknown --provider, bad flags) and 1 (IO). `refresh`: 0 success; 1 IO error; 2 bad `--auto` match or unknown flags. `verify`: 0 clean; 1 stale routes present or IO error; 2 bad flags. (Source: annex-d §2.6.)

9. **stdout/stderr discipline.** stdout = list table / verify report / verify stale lines; stderr = warnings, summary lines, and the F22-rendered failure line. F27 RunE returns `UsageError`/`CodedError` per the F22 contract; F27 never writes failure lines itself (D-8). (Source: F22 contract.)

## 3. Error behaviour

| Condition | Exit | stderr / message |
|---|---|---|
| `add` unknown `--provider` | 2 | `[arguments] unknown provider "x"; valid providers: <ids>` |
| `add` empty `--model-id` or `--model` | 2 | `[arguments] --model-id and --model are required` |
| `add` duplicate (provider, model-id) | 2 | `[arguments] route "x:y" already exists; remove it first` |
| `remove` no such route | 1 | `[no_route] no route "x:y"` |
| `list` unknown `--provider` | 2 | `[arguments] unknown provider "x"` |
| `refresh --auto` no/ambiguous match | 2 | `[arguments] no score row matching "x"` |
| Usage disabled (refresh) | 0 | single warning `warning: usage is disabled; refresh uses static sources only` |
| `verify` stale routes | 1 | stale lines stay on stdout (report is the deliverable, via F22 `ReportedError`); summary line on stderr; F22 renders `[stale_routes] n stale route(s); run which-model routes refresh` |
| `verify` scores hash mismatch | 0 | `warning: scores CSV changed since routes were produced; run which-model routes refresh` |
| Unrouted score rows | 0 | one warning per row on stderr |
| IO error (any subcommand) | 1 | `[runtime] <message>` |

## 4. Decisions

| Decision | Value | Rationale |
|---|---|---|
| D-1 | Five subcommands in one file `routes_cmd.go`, one `register(NewRoutesCmd)` | Matches the F22 one-file-per-command contract; subcommands are intra-feature |
| D-2 | `add` requires explicit `--model-id` and `--model`; duplicate → exit 2 | The two fields are the route's identity and payload; silent overwrite would corrupt user data |
| D-3 | `remove` is exact-match, exit 1 when absent; provenance is not a removal barrier | Idempotent-ish scripting (exit 1 = "wasn't there") and no provenance police |
| D-4 | `list` JSON = `RouteList{schema_version, routes}` under the F22 envelope | Global CONTRACTS §6 envelope rule; bare arrays are not documents |
| D-5 | `--auto` adds a `user_declared` route with the fuzzy-matched model, erroring on zero/ambiguous matches | The flag's contract is "one decisive route", not "maybe"; determinism first |
| D-6 | Refresh under usage-disabled warns once and proceeds | annex-d §4.6: routes refresh = reduced sources + one warning; refusal would break `pick`'s offline setup |
| D-7 | `verify` with stale routes still writes the report to stdout (it IS the deliverable) and returns exit 1 | Same "report is primary output" rule as `auth status` (F25 D-9); exit 1 is the signal, the report is the payload |
| D-8 | Exit signalling purely via F22 error types; F27 never renders failure lines | One render point (F22 `ExecuteArgs` + F03 `output.WriteFailure`) keeps the `[<code>] <message>` shape stable |

## 5. Out of scope

- Route merging semantics and the production pipeline itself: F18 owns `ProduceRoutes` and the merge rules; F27 only persists/prints.
- Scores CSV parsing and hashing: F06 owns `csvstore.ReadScores` and the sha256 helper; F27 consumes both.
- Provider inventory discovery (live sources): F14/F18 own; F27's refresh just calls `ProduceRoutes`.
- `pick`'s join logic: F26 consumes routes; F27 never touches it.
- History/audit of route edits (no undo, no changelog).
- Multi-provider batch flags (e.g. `add --provider a --provider b`): each call edits one route.
