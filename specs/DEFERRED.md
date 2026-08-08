---
kind: deferred-notes
version: "1.0"
project: which-model
---

# Deferred seams

Contradictions found during normalization. Each is either resolved in-place (with the resolution recorded) or gated behind a fix task BEFORE the affected features' implementation starts.

## D1 — F14 transport type vs F11 `FetchFunc` — RESOLVED (2026-08-07)

**Conflict:** `specs/features/F14-usage-fetch/SPEC.md §12` originally said FetchAll builds one shared `httpkit.NewClient()` per call and hands it to provider fetches. But `specs/features/F11-usage-types/CONTRACTS.md §1` fixes `FetchFunc func(ctx, cred Credential, client *http.Client) (Snapshot, error)` — a raw `*http.Client`. F15/F16/F17 implement their fetch on `*http.Client` with their own `requestJSON` (a faithful core.mjs port returning `(int, json.RawMessage, error)`), which is the correct call for a byte-parity port. The contradiction was a compile-breaker sitting inside F14-T2's instructions.

**Resolution (applied):** F14 SPEC §12, SPEC D4, CONTRACTS §2 cross-feature row, and TASKS T1/T2 now specify the plain `&http.Client{}` seam (no `Timeout` field; per-provider deadlines from contexts). `MapError` keeps its defensive `httpkit.AsError` step (harmless — provider fetches return provider errors, not httpkit errors; the step exists for future direct httpkit consumers). F11/F15/F16/F17 untouched.

**Remaining action:** none — `specs/verify_sdd.py` stays clean and the F14 implementer follows SPEC §12 (plain client), not any older pin.

## D2 — Providers on raw `*http.Client` vs httpkit (accepted, not a defect)

F15/F16/F17 deliberately bypass `internal/httpkit`: their port keeps core.mjs's `requestJson` shape (status returned alongside the body) which httpkit's `Do([]byte, error)` cannot express without losing the provider-specific `mapStatus` messages. F04 SPEC §8/D10 record this. Revisit only if a future provider needs httpkit's retry/bounding semantics — at that point extend `FetchFunc` rather than converting the three ported adapters.

## D3 — CI workflow ownership gap — RESOLVED (2026-08-07)

**Conflict:** `specs/global/SPEC.md §7` mandates "build-matrix CI on every change" (default + `-tags nousage`), and F21 SPEC R5 requires CI to run both variants plus the strings-scan audit. F21-T8 creates `scripts/audit-nousage.sh` but NO feature task created the `.github/workflows/ci.yml` that runs it — the requirement was real with zero task ownership.

**Resolution (applied):** added F01-T10 (`specs/features/F01-config/TASKS.md`) — creates the CI workflow with pinned action SHAs (same pins F30 uses), `go build ./... && go vet ./... && go test ./...`, and a `hashFiles('scripts/audit-nousage.sh') != ''`-guarded step running `bash scripts/audit-nousage.sh`, so the matrix activates automatically when F21-T8 lands. F01 task_count raised 9 → 10.

**Remaining action:** none.

## D4 — F02-T4 test table vs mandated WeightedMean code — RESOLVED (2026-08-07)

**Conflict:** `specs/features/F02-decimal/TASKS.md` T4's test table contradicted its own mandated implementation in two cells: case 5 said `(10)` weights `(2)` → `"5"`, but the mandated formula `Σ(cᵢwᵢ)/Σwᵢ` gives (10·2)/2 = 10; case 12 expects the 34-digit quotient `1.66666666666666666666666666666667` for 5/3, but shopspring/decimal's default `DivisionPrecision` (16) yields `1.6666666666666667`.

**Resolution (applied):** case 5 corrected to `"10"` (the formula in SPEC §4 is authoritative; the table cell was a transcription error). Case 12 is correct as written and expresses the real intent — verified empirically: `decimal.DivisionPrecision = 34` reproduces case 12 exactly AND the F10-T4 golden `655/7 = 93.5714285714285714285714285714285714` (34 digits) that M1's byte-parity milestone depends on. T4 now mandates a package `init` in `internal/decimal/decimal.go` setting `decimal.DivisionPrecision = 34`.

**Remaining action:** none — `internal/decimal` is the repo's only decimal layer (CONTRACTS §8), so the process-global pin lives exactly where it belongs.

## D5 — Canonical Snapshot missing `UsageKnown` — RESOLVED (2026-08-07)

**Conflict:** `specs/global/CONTRACTS.md` §1.5 defined `Snapshot` without a `UsageKnown` field, yet four features require it: F11-T1 test case 6 pins the Snapshot JSON `{"...","confidence":"","usage_known":false}`; F13-T2 case 1 constructs `Snapshot{UsageKnown: true}` (compile-breaker without the field) and case 2 asserts `snapshot.usage_known == true` in the cache file; F13 CONTRACTS §cache-file shows `"usage_known": false` at snapshot level; F15 golden fixtures and F24 T8 case 4 assert snapshot-level `usage_known` round-trip. Only `Window.UsageKnown` (§1.4) existed — insufficient for all four consumers.

**Resolution (applied):** added `UsageKnown bool `json:"usage_known"`` to `Snapshot` after `Confidence` (position chosen so F11-T1 case 6's pinned field order is byte-exact). No `omitempty` — the field always serializes, matching the pinned goldens (`"usage_known": false` present on zero snapshots).

**Remaining action:** none — F11's `types.go` is updated; all four consumers' tasks now match the canonical type.

## D6 — F06-T5 MergeRows instructions vs case 5 identity cells — RESOLVED (2026-08-07)

**Conflict:** `specs/features/F06-csvstore/TASKS.md` T5 step 4 said MergeRows skips the `model`/`reasoning` columns when merging a fresh row onto a matching existing row, but its own case 5 expects the merged row's reasoning cell to be `high` (the EXISTING row's value) when fresh said `default` — impossible under "skip identity columns" (fresh would keep `default`).

**Resolution (applied):** the case 5 table is authoritative: a matched row keeps the existing store's canonical identity cells (model and reasoning copied from the collapsed existing row). T5 instruction step 4 now states this rule explicitly. Rationale: the store's spelling is canonical — after `default→high` collapse the stored row is the source of truth for identity, and keeping it makes repeated merges idempotent.

**Remaining action:** none — implementation matches the corrected instruction and all 12 merge test cases pass.

## D7 — F22-T8 case 1 assumes a non-darwin user config path — RESOLVED (2026-08-07)

**Conflict:** `specs/features/F22-cli-skeleton/TASKS.md` T8 case 1 expects `config path` stdout `<tmp>/.config/which-model/config.toml`, but F01 D4 pins darwin paths unconditionally (`~/Library/Application Support/which-model/config.toml` on darwin; `specs/features/F01-config/CONTRACTS.md` §paths) and the tests run on darwin.

**Resolution (applied):** the F01 contract wins — F22's `config path`/`config set` target come from `config.ResolvePaths(runtime.GOOS, home, os.Getenv)`; T8's tests compute the expectation from that same call instead of hard-coding `.config`. No spec text change beyond this note: the table's intent (user config path under a temp HOME) holds on every GOOS via ResolvePaths.

**Remaining action:** none.

## D8 — F19 test tables vs inclusive-bound ladder rule; `Pressure` constructor name collision — RESOLVED (2026-08-07)

**Conflict (tables):** `specs/features/F19-bands/TASKS.md` T6 case 5 expected `p{74.99}` → `standard`, T7 row 1 expected gate-98 `p{97.99}` → `{elevated, 0.60}`, and T7 drain rows 7–9 expected `{low, 0.25}`/`{standard, 0.60}`/`{elevated, 0.85}` for pressures 30/60/90. All contradict the feature's own authoritative rule — SPEC §2.4 ("first tier whose bound is >= pressure; upper bound INCLUSIVE: exactly 25 maps to low, exactly 50 to standard, exactly 75 to elevated") and SPEC §2.5 (drain: tier N takes `weight[len-1-N]`) — which map 74.99 → elevated, 97.99 → critical (bound 100), and 30/60/90 → tiers 1/2/3. The tables applied exclusive-bound (off-by-one) arithmetic; T6 case 1 (`p{25}` → low, "inclusive edge") confirms inclusive intent.

**Conflict (name):** CONTRACTS §2 declared `func Pressure(snapshot usage.Snapshot, windowIDs []string) Pressure` — a Go type/function name collision that cannot compile.

**Resolution (applied):** T6 case 5 corrected to `{elevated, 0.60}`; T7 row 1 corrected to `{critical, 0.25}`; T7 drain rows 7–9 corrected to `{standard, 0.60}`/`{elevated, 0.85}`/`{critical, 1.00}` (row 10 was already correct). The constructor is renamed `NewPressure` (both implementing agents independently converged on it; CONTRACTS §1–§2 and TASKS T3 updated); `internal/pick/band/pressure.go` keeps the verbatim doc comment.

**Remaining action:** none — no other spec cites `band.Pressure(` as a constructor call; F18/F24/F26 construct pressures via `NewPressure` per this note.

## D9 — Canonical catalog types unowned + global §2.2 interface drift — RESOLVED (2026-08-07)

**Conflict (ownership):** `specs/global/CONTRACTS.md` §2.1 (`ScoreRow`, `package catalog`) and §4.3 (`Profile`) had no task creating their Go file — F09's Files lists are score-only and F10-T1 creates only `internal/pick/` files, yet both features consume `catalog.ScoreRow`/`catalog.Profile` (F10 CONTRACTS pins the file as `internal/catalog/types.go`).

**Conflict (signatures):** global §2.2 declared `Normalize(values []decimal.Decimal, higherIsBetter bool) []decimal.Decimal` and `Aggregate(components, weights) decimal.Decimal`, while F09's CONTRACTS/TASKS pin `Normalize(raw, min, max decimal.Decimal) decimal.Decimal` and `Aggregate(values, weights) (decimal.Decimal, bool)`. F09's whole test suite (T1/T2 goldens) and SPEC D1 (blank denominator → false, never zero-imputed) are written against the F09 signatures.

**Resolution (applied):** created `internal/catalog/types.go` (`package catalog`) with `ScoreRow` + `Profile` verbatim from §2.1/§4.3. Rewrote global §2.2 to F09's pinned signatures (direction moves to the derive layer's reflection helper, per F09 CONTRACTS §2.2 commentary). Fixed §4.3 with the `package catalog` declaration and file note.

**Remaining action:** none — F09 imports `catalog.ScoreRow`; F10 imports both types; neither feature redefines them in `internal/catalog/score` or `internal/pick`.

## D10 — F09-T6 raw golden fixture vs scores golden — RESOLVED (2026-08-07)

**Conflict:** the F09-T6 raw fixture cells as printed in TASKS.md do not derive into `testdata/scores_golden.csv` byte-exactly (two benchmark cells for GPT and Kimi rows were misassigned between Toolathlon/Finance Agent/Program Bench/MCP Atlas columns).

**Resolution (applied):** the scores golden is the contract (byte-exact Derive output is the acceptance criterion; TASKS.md sanctions recomputing `raw_sha256`). The two raw cells were corrected so `Derive(raw) == scores_golden.csv` byte-for-byte, and the provenance constant was recomputed to `4469f49a0fe94cc0f778a9a7e30dc8f7f79327ca5501ea3200cbe44e7d5e0cd3` (self-verified in `derive_test.go`). Secondary ports noted by the implementer: singleton-column ranges derive blank scores (Python rejects min==max only for 2+ equal values); mandatory-column "no published values" is unreachable behind the zero-eligible error.

**Remaining action:** none.

## D11 — `routing.Route`/`Provenance` and `pick.Candidate`/`Strategy` unowned — RESOLVED (2026-08-07)

**Conflict:** global CONTRACTS §3.1 (`routing.Route` + `Provenance`) and §4.1/§4.2 (`pick.Candidate` + `Strategy`) had no task creating their Go files. F21-T7 (degraded assembly) needs `pick.Candidate`; F20 consumes `pick.Candidate` + `pick.Strategy`; global §4.1's `Candidate` embeds `routing.Route`, and `internal/routing` did not exist. F10's CONTRACTS lists only ModelScore/ExcludedRow/Result — it does not own Candidate.

**Resolution (applied):** created `internal/routing/types.go` (`package routing`) with `Route` + `Provenance` verbatim from §3.1, and `internal/pick/candidate.go` (`package pick`) with `Candidate` (embedding `routing.Route` per §4.1) + the six `Strategy` constants from §4.2. Neither feature may redefine these.

**Remaining action:** F18-T1 ("Declare the canonical Route and Provenance types") MUST NOT redeclare them in `route.go` — it consumes `internal/routing/types.go` and implements only the ProduceRoutes/persistence layer around it. Candidate field access is `c.Route.Provider` / `c.Route.ModelID` / `c.Route.Reasoning` (nested, never flattened). Related: F21's nousage `fetch.Options` stub carries `CacheDir string` field-for-field with F14's real Options (added to F21 CONTRACTS §4 during W5).

## D12 — F10-T4 warning-string order pins contradict SPEC D2; decimal JSON quoting — RESOLVED (2026-08-07)

**Conflict (warnings):** F10-T4's task text and table pinned missing-category warning orders that contradict the feature's own SPEC D2 / CONTRACTS §3 ("names in CategoryNames order") and were mutually unsatisfiable across rows (e.g. row 1 listed `software_engineering, instruction_following, agentic_tools` while row 5 required `CategoryNames order`). `CategoryNames` (annex-b §5.1) is `reasoning, knowledge, research, planning_capability, instruction_following, software_engineering, ui_visual, agentic_tools, finance, evidence_capture, security, data_ml`.

**Conflict (JSON):** CONTRACTS §2.4 says serialize via `decimal.Decimal.MarshalJSON`; T7 pins unquoted JSON numbers.

**Resolution (applied):** implementation follows SPEC D2 — warnings list missing categories in `CategoryNames` order (`instruction_following, software_engineering, agentic_tools, evidence_capture` for simple_action_execution; `instruction_following, agentic_tools` for the balanced partial row per the Python ground truth). T4's contradictory pins corrected to match. JSON numbers are unquoted via `decimal.MarshalJSONWithoutQuotes = true` set in `internal/pick`'s init (still `decimal.Decimal.MarshalJSON`; precision unchanged; no other package asserts quoted decimal output).

**Remaining action:** none.
