---
kind: deferred-notes
version: "1.0"
project: which-model
---

# Deferred seams

Contradictions found during normalization that are recorded here instead of fixed mid-flight, per orchestrator ruling. Each gets a fix task BEFORE the affected features' implementation starts.

## D1 — F14 transport type vs F11 `FetchFunc` (compile-breaker risk)

**Conflict:** `specs/features/F14-usage-fetch/SPEC.md §12` says FetchAll builds one shared `httpkit.NewClient()` per call and hands it to provider fetches. But `specs/features/F11-usage-types/CONTRACTS.md §1` fixes `FetchFunc func(ctx, cred Credential, client *http.Client) (Snapshot, error)` — a raw `*http.Client`. F15/F16/F17 implement their fetch on `*http.Client` with their own `requestJSON` (a faithful core.mjs port returning `(int, json.RawMessage, error)`), which is the correct call for a byte-parity port.

**Ruling:** F11/F15/F16/F17 stay as written. F14 SPEC §12's `httpkit.NewClient()` sentence is wrong and must be corrected to a plain `&http.Client{}` with a timeout-free transport (per-provider deadlines come from contexts per F14 SPEC §5). F14's `MapError` step (2) `httpkit.AsError` remains valid for other call paths but is not on the provider fetch path.

**Fix task:** edit `specs/features/F14-usage-fetch/SPEC.md §12` and `CONTRACTS.md §2` cross-feature surface row (the `httpkit.NewClient()` mention) to the `*http.Client` seam; add a one-line note that httpkit is consumed by catalog collectors (F08), not by provider adapters. Owner: orchestrator, before F14-T2 implementation begins.

## D2 — Providers on raw `*http.Client` vs httpkit (accepted, not a defect)

F15/F16/F17 deliberately bypass `internal/httpkit`: their port keeps core.mjs's `requestJson` shape (status returned alongside the body) which httpkit's `Do([]byte, error)` cannot express without losing the provider-specific `mapStatus` messages. F04 SPEC §8/D10 record this. Revisit only if a future provider needs httpkit's retry/bounding semantics — at that point extend `FetchFunc` rather than converting the three ported adapters.
