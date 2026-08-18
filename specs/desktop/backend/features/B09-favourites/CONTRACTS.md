---
kind: feature-contracts
version: "1.0"
feature: B09-favourites
project: which-model-desktop
---

# B09-favourites — Contracts

## 1. Package and files

| File | Contents |
|---|---|
| `internal/service/favourites.go` | `FavouriteService`, `(*Services).Favourites()`, List/Pin/Unpin |
| `internal/service/favourites_test.go` | table tests per §5 |

Package `internal/service` (B00 import boundary applies). Uses D00's `Favourite` DTO and `ParseRouteKey` (not redefined here), B02's sentinels and test helper, and the B00 CONTRACTS §6.3 availability set (same computation B04/B06 use).

## 2. Exported API — `internal/service/favourites.go`

```go
package service

// FavouriteService is the favourites facet of Services; the host registers
// it as a Wails service. Zero value unusable; obtain via Favourites().
type FavouriteService struct { s *Services }

// Favourites returns the favourites facet (shares the Services lock/config).
func (s *Services) Favourites() *FavouriteService

// List returns [favourites].pins in stored order, each annotated per SPEC
// §2.2–2.4 (ModelName from the routes table, RouteLabel "provider ·
// reasoning" or "no provider · reasoning", InRange from the availability
// set). Ill-formed stored pins are surfaced, not dropped (SPEC §2.7).
func (f *FavouriteService) List(ctx context.Context) ([]Favourite, error)

// Pin validates routeKey grammar and appends it to [favourites].pins.
// Already pinned -> nil, no write, no event. Otherwise persists atomically
// and emits config:changed {"section":"favourites"}. Bad grammar ->
// errValidation (§4). Out-of-range keys are accepted (SPEC §2.5).
func (f *FavouriteService) Pin(ctx context.Context, routeKey string) error

// Unpin validates routeKey grammar and removes it from [favourites].pins.
// Not pinned -> nil, no write, no event. Otherwise persists atomically and
// emits config:changed {"section":"favourites"}.
func (f *FavouriteService) Unpin(ctx context.Context, routeKey string) error
```

## 3. Config keys and events

- Owns reads/writes of `[favourites] pins = []string` (schema + default `[]` owned by B01).
- Events: `Pin`/`Unpin` (when a write happens) emit exactly `config:changed` with payload `{"section":"favourites"}`; `List` never emits.

## 4. Error strings (exact)

| Condition | String |
|---|---|
| `Pin`/`Unpin` grammar failure | `favourites: invalid route key %q` (wraps the `ParseRouteKey` error; outer format exact, checked with `strings.HasPrefix`) |

Maps to `validation_failed` via `toErrorDTO`.

## 5. Test fixtures (`favourites_test.go`)

Built on `newTestServices(t, ...)` (B02) with the default synthetic routes table (providers `claude`,`codex`). Required cases:

1. **List order + resolution**: config with two pins in non-alphabetical order → returned in stored order; `ModelName` equals the fixture `Route.Model` for each; `RouteLabel` = `"claude · high"` style.
2. **InRange transitions**: pin in the availability set → `InRange` true; disable its provider (config) or add `model_id@reasoning` to `[routes.disabled]` → `InRange` false and `RouteLabel` starts `"no provider · "`.
3. **Pin round-trip**: Pin → config.toml on disk contains the key under `[favourites].pins`; recorder shows exactly one `config:changed {"section":"favourites"}`.
4. **Idempotence**: second Pin of the same key and Unpin of an absent key → nil error, zero events, file mtime/content unchanged.
5. **Grammar rejection**: `Pin("not a key")` → `ErrorDTO.Code == "validation_failed"`, message prefix per §4, zero events.
6. **Corrupt stored pin**: hand-written `pins = ["garbage"]` → List returns one entry, `InRange` false, `ModelName` `"garbage"`, no error.
7. **Unknown model fallback**: pin whose `model_id` is absent from the routes table → `ModelName` = model_id verbatim, `InRange` false.
