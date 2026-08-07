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
