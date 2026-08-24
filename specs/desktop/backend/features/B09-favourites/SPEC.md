---
kind: feature-spec
version: "1.0"
feature: B09-favourites
project: which-model-desktop
---

# B09-favourites — Pinned Models

## 1. Purpose

`FavouriteService` manages the `[favourites].pins` list — route keys the user has pinned in the Settings "Favourites" page (mockup `favRows` block). A pin is a full route identity (`provider/model_id@reasoning`, D00 CONTRACTS §1); the service reports, per pin, a resolved display name, a human route label, and whether the route is still in the availability set. Pinning never forces a model into rank output (the mockup footnote is normative copy for the UI); the backend only stores and annotates pins.

Depends on: B02 (Services core, sentinels, test helper), B06 (availability semantics via B00 CONTRACTS §6.3).

## 2. Behaviour

1. **Storage.** Pins live in config as `[favourites] pins = ["provider/model_id@reasoning", ...]` (B01 schema). `List` returns them in stored order — the order pins were added; no sorting.

2. **Model name resolution.** The display name for a pin comes from the routes table: the `routing.Route` whose `(Provider, ModelID, Reasoning)` equal the parsed pin. `Route.Model` is the display name (it equals the scores CSV `Model` column / `catalog.ScoreRow.Model`; `Route.ModelID` is the engine id used in route keys). When no exact route matches, fall back to any route with the same `ModelID` (first by provider asc, then reasoning order as stored); when none exists at all, `ModelName` = the pin's `model_id` verbatim.

3. **Route label.** `Favourite.RouteLabel` is `"<provider> · <reasoning>"` using the pin's own provider and reasoning (separator is space–middle-dot–space, U+00B7, matching the mockup). When the pin is out of range (§2.4) the provider part is replaced: `"no provider · <reasoning>"`.

4. **In range.** `Favourite.InRange` is true iff the pin's exact route is in the availability set of B00 CONTRACTS §6.3: the route exists in the routes table, its provider is enabled, and `model_id@reasoning` is not listed under `[routes.disabled].<provider>`.

5. **Pin.** Validates the route key grammar (`ParseRouteKey`); a well-formed key is accepted even if currently out of range — `InRange` reports that state, pinning is not availability-gated. Appends the key to `pins` and persists atomically, then emits `config:changed {"section":"favourites"}`. Pinning an already-pinned key is an idempotent success: no write, no event.

6. **Unpin.** Validates grammar, removes the key from `pins`, persists, emits `config:changed {"section":"favourites"}`. Unpinning a key that is not pinned is an idempotent success: no write, no event.

7. **Corrupt stored pins.** A stored pin that fails `ParseRouteKey` is still returned by `List` (RouteKey verbatim, `ModelName` = the raw string, `RouteLabel` = `"no provider · default"`, `InRange` false) so the user can see and unpin it; `List` never fails because of a bad stored value.

## 3. Error behaviour

- `Pin`/`Unpin` with an ill-formed route key → `errValidation`, mapped to `validation_failed`; message per CONTRACTS §4. No write, no event.
- Persist failure (marshal/atomic write) → `io_error`; in-memory pins unchanged (B00 SPEC §2.2).
- `List` is read-only and only fails on lock-free internal errors (none expected); it never emits.

## 4. Decisions

| Decision | Value | Rationale |
|---|---|---|
| Display-name source | Routes table `Route.Model` (equals scores CSV `Model` column), keyed by the pin's exact `(provider, model_id, reasoning)` | The routes table is the only place mapping engine `model_id` → display name; `ScoreRow` has no id column |
| Pin validation depth | Grammar only; availability is reported, not enforced | Mockup lets pins fall out of range when providers toggle off; unpinning must remain possible |
| Idempotent no-ops | Duplicate Pin / absent Unpin succeed with no write and no event | B00 CONTRACTS §6.5: events accompany real mutations only |
| Stored order | `pins` array order preserved verbatim | Matches user pin order in the mockup; no hidden re-sorting |
| Corrupt stored pin | Surfaced in List, never dropped silently, never fails List | User must be able to unpin garbage introduced by hand-editing config.toml |
