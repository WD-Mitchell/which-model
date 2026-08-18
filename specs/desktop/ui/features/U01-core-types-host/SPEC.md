---
kind: feature-spec
version: "1.0"
feature: U01-core-types-host
project: which-model-desktop
---

# U01-core-types-host — @which-model/core

## 1. Purpose

The complete `packages/core` package: TypeScript mirrors of every D00 CONTRACTS §2 DTO, the `EngineHost` interface (D00 CONTRACTS §5), the event enum (D00 CONTRACTS §3), the `EngineError` class over `ErrorDTO`, and a fully stateful in-memory `createMockEngineHost` whose fixture data is adapted from the mockup's `MODELS`/`SCALE`/`EXTRA`/`GROUP_DEFS`/`ALL_BENCH`/`DETECTED` constants. This package unblocks ALL UI features (U02–U14) without the Go backend. Zero runtime dependencies (D00 §2.1d).

Inherits `specs/desktop/global/*` and `specs/desktop/ui/*`. Depends on: D00 (spec only).

## 2. Behaviour

1. **Types mirror D00 exactly.** `src/types.ts` declares one exported interface (or type alias) per D00 CONTRACTS §2 struct, same names, `snake_case` keys identical to the Go JSON tags. Field mapping is mechanical: Go pointer (`*int`, `*ProfileDetail`) → `T | null`; a pointer that ALSO carries `omitempty` (only `RankRequest.Overrides`) → optional AND nullable (`overrides?: ProfileDetail | null`); everything else required. `ProfileDetail` is a type alias of `ProfileSummary` (as in Go). String enums (`GUISettings.layout` etc., `UsageWindow.id` excluded — it is open) are typed as union literals matching the D00 comments. No extra fields, no camelCase.

2. **Errors.** `src/errors.ts` exports `ErrorCode` — the closed union of the seven D00 CONTRACTS §4 codes — and `class EngineError extends Error implements ErrorDTO` with `code: ErrorCode` and `message: string`, plus the guard `isEngineError(e: unknown): e is EngineError`. Every rejected promise from any `EngineHost` implementation (mock included) rejects with an `EngineError`.

3. **Events.** `src/events.ts` exports the `EngineEvent` union of the five D00 CONTRACTS §3 names, an `ENGINE_EVENTS` readonly tuple, and `EngineEventPayloads` — a type map from each event name to its payload shape (CONTRACTS §5).

4. **Host.** `src/host.ts` declares `EngineHost` VERBATIM from D00 CONTRACTS §5 — same groups, same method names, same signatures — importing types from `types.ts`/`events.ts`. Shape changes to this file happen only by editing D00 (D00 CONTRACTS §7).

5. **Mock host — statefulness.** `createMockEngineHost(overrides?)` (`src/mock.ts`) returns `EngineHost & { data: MockData }` per U00 CONTRACTS §4. `data` is the live mutable state; every read method answers from it; every mutating method updates it AND synchronously fires exactly one mapped event (CONTRACTS §6) to all listeners registered via `on` before its promise resolves. `on` returns an unsubscribe function; unsubscribing during dispatch is safe. All methods still return Promises (resolved values are deep copies, so callers cannot mutate `data` through results).

6. **Mock host — seeded fixtures.** Initial `MockData` is the fixture set in CONTRACTS §4 (8 models, 8 profiles = 5 complexity + 3 extra, 11 builtin groups, benchmark catalogue = the mockup's `ALL_BENCH` list, 4 harnesses, 4 providers with the mockup's usage numbers, default `GUISettings`). `overrides` shallow-merges by top-level `MockData` key. Fixture data is deterministic: NO `Date.now()`, NO randomness anywhere in the package — all timestamps derive from the fixed base clock `MOCK_NOW = "2026-01-01T12:00:00Z"` (exported const).

7. **Mock host — ranking.** `pick.rank` recomputes on every call from current `data` using the mockup's formula (CONTRACTS §7): weighted core ratio and weighted task ratio, blended by `core_share/100`, ×100. Candidate set: for each model, the route provider is the FIRST enabled provider in priority order that the model lists and whose `(provider, model_id, reasoning)` route is not disabled; models with no route are excluded. Sort score descending (ties: model id ascending); `total` = candidates before truncation; truncate to `holds` (request value, or `data.settings.holds` when 0). `score` rounded to 2dp (`Math.round(x*100)/100`); ranks 1-based. `overrides` non-nil ⇒ rank with the override weights and record nothing.

8. **Mock host — CRUD error enforcement.** So UI error paths are testable, the mock enforces: builtin mutation → `builtin_readonly` (profiles.save/delete on builtin slugs; catalog.saveGroup/deleteGroup on builtin groups; harnesses.save/delete on builtin harnesses); unknown slug/id/name on get/detail/save-target/delete/duplicate → `not_found`; `catalog.saveGroup` with `renameTo` colliding with an existing slug, and `profiles.save` whose slug collides with a BUILTIN profile → `conflict` is NOT used there (that is `builtin_readonly`) — `conflict` fires only when `renameTo`/duplicate would create a slug that already exists; malformed route keys (grammar D00 CONTRACTS §1) to favourites/launch → `validation_failed`. `profiles.save` on an unknown non-builtin slug creates it (upsert). `duplicate*` appends `_copy`, then `_copy_2`, `_copy_3`… until free.

9. **Mock host — remaining groups.** `harnesses.launch` validates the route key and harness, returns `{copied: data.settings.copy_command_instead, command}` with `{model_id}`/`{reasoning}` substituted, increments the profile's `picks`, sets its `last_used` to `MOCK_NOW`, and fires `pick:recorded`. `usage.snapshots` maps provider fixtures to `UsageDTO` (disabled provider → excluded). `window.*` methods resolve `undefined` without effect (no event, no state change). `pick.catalogLine` computes `{models: model count, providers_on, harnesses}` from live `data`.

10. **Package mechanics.** `"@which-model/core"`, `"type": "module"`, built by `tsc` to `dist/` (ESM + `.d.ts`), TS `strict`, target ES2022 (U00 CONTRACTS §1). Exports map exposes `.` (barrel: types, events, errors, host) and `./mock` (mock only), so app production bundles importing `.` never pull fixture data. `index.ts` re-exports everything except `mock.ts`.

## 3. Error behaviour

- Every mock rejection is `new EngineError(code, message)`; messages are human-readable and name the offending slug/id/key.
- Type-only files (`types.ts`, `events.ts`, `host.ts`) contain no runtime logic and cannot fail.
- `createMockEngineHost` never throws on valid `overrides`; unknown keys in `overrides` are a TypeScript compile error, not a runtime check.
- Event listener exceptions propagate to the mutating caller (no swallowing) — tests rely on synchronous dispatch.

## 4. Decisions

| Decision | Value | Rationale |
|---|---|---|
| Pointer mapping | Go `*T` → `T \| null`; `*T,omitempty` → `field?: T \| null` | Mechanical, lossless mirror of the wire JSON |
| Mock event dispatch | Synchronous, before the promise resolves | Deterministic tests; refetch-on-event assertions need ordering |
| benchScore | Fixed per-(model, home-group) table, no hashing | Mockup's string-hash noise adds nothing; determinism for a weak implementer |
| Complexity slugs | B03's five slugs (`simple_action_execution`, …), not the mockup's `simple_action` | Mock must match what the real backend will return |
| Clock | Exported `MOCK_NOW` const; no `Date.now()` | Snapshot-stable fixtures and tests |
| `./mock` subpath export | Separate entry, excluded from barrel | Fixture data stays out of production bundles |
| Returned values are deep copies | `structuredClone` on every resolve | Callers mutating results must not corrupt `data` |
| Ranking tie-break | score desc, then model id asc | Total order ⇒ byte-identical results across runs |

## 5. Out of scope

- The real Wails-backed host (`apps/desktop/src/host/wailsHost.ts`) — S04.
- Components, hooks, CSS — U02+. Query keys / invalidation wiring — U05.
- Go DTO structs and route-key parser — B02 (`internal/service/dto.go`).
- Faithful reproduction of the mockup's `benchScore` hash and `coverage` simulation (simplified per Decisions).
