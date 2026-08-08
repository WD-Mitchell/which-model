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
