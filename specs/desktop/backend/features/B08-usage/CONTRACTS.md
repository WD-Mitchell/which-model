---
kind: feature-contracts
version: "1.0"
feature: B08-usage
project: which-model-desktop
---

# B08-usage — Contracts

## 1. Package and files

| File | Contents |
|---|---|
| `internal/service/usage.go` | `UsageMode`, `Snapshots`, `Mode`, `SetMode`, `SetBackend`, `StartRefresher`, unexported mapper + helpers |
| `internal/service/usage_test.go` | fixtures + tests (§8); fake descriptor registration |

Only this file (within `internal/service`) may import `internal/usage/fetch`, `internal/usage/cache`, `internal/usage/toggle`. DTOs referenced: D00 `UsageDTO`, `UsageWindow`. Sentinels used: `errValidation`, `errUsageUnavailable` (B00 §3).

## 2. Exported API

```go
package service

// UsageMode is the Mode() return payload.
type UsageMode struct {
    Mode    string `json:"mode"`    // "auto"|"on"|"off"
    Backend string `json:"backend"` // "off"|"native"|"codexbar"
}

// Snapshots fetches usage for every enabled provider, in config priority
// order (B00 §6.1). force tightens the cache TTL to 60s (else 15m); it
// never sets fetch.Options.Refresh. Disabled gate -> errUsageUnavailable
// ("usage disabled: <reason>"). Calls are serialised (SPEC §2.4).
func (s *Services) Snapshots(ctx context.Context, force bool) ([]UsageDTO, error)

// Mode reports the current [usage] section (SPEC §2.9).
func (s *Services) Mode(ctx context.Context) (UsageMode, error)

// SetMode validates mode in {"auto","on","off"}, persists [usage] enabled
// ("on"->"true", "off"->"false"), emits config:changed{section:"usage"}.
func (s *Services) SetMode(ctx context.Context, mode string) error

// SetBackend validates backend in {"off","native","codexbar"}, persists
// [usage] backend, emits config:changed{section:"usage"}.
func (s *Services) SetBackend(ctx context.Context, backend string) error

// StartRefresher spawns the background loop (SPEC §2.10): immediate
// fetch, then ticker at interval (<=0 -> 5m). Emits usage:updated {} after
// every completed fetch; skips ticks while a fetch is in flight; stops on
// ctx.Done(). Idempotent: second call is a no-op.
func (s *Services) StartRefresher(ctx context.Context, interval time.Duration)
```

## 3. fetch.Options per call (SPEC §2.3)

| Field | Value |
|---|---|
| `Backend` | `s.cfg.Usage.Backend` |
| `Enabled` | `map[id]bool` — only providers with config `enabled = true` (default-deny) |
| `Refresh` / `Offline` | `false` / `false` |
| `MaxAge` | `60 * time.Second` when force; `15 * time.Minute` otherwise |
| `ShowIdentity` | `true` |
| `Timeout` | `10 * time.Second` |
| `MaxParallel` | `0` (fetch default) |
| `CacheDir` | `s.usageCacheDir` (§7) |
| `Source` | zero value |

`providers` argument = enabled ids sorted by ascending `priority`, ties id asc.

## 4. Snapshot → UsageDTO mapping

### 4.1 Snapshot fields

| Snapshot | UsageDTO |
|---|---|
| `Provider` | `Provider` |
| `Plan` | `Plan` (may be `""`) |
| `Confidence` | `Confidence` verbatim ("live"/"cached"/"estimated") |
| `Stale` | `Stale` |
| `Source` | `Auth` per §4.3 |
| `Failure != nil` | `Failure` = `Failure.Message`, or `Failure.Code` when Message `""`; windows still mapped if present |
| `Failure == nil` | `Failure` = `""` |
| `Windows` | `Windows` per §4.2, ordered by `usage.Get(Provider).Windows` spec order; ids absent from the descriptor (or unknown descriptor) appended in snapshot order |
| `Account`, `FetchedAt`, `UsageKnown` | not mapped |

Credits/Resets lines: SPEC §2.6–2.7 (duration format §4.4).

### 4.2 Per-window (one row per field combination; first matching row wins)

| Window state | Emitted? | `UsedPercent` | `Unlimited` |
|---|---|---|---|
| `Synthetic == true` (any other fields) | NO — dropped | — | — |
| `Unlimited == true` | yes | nil | true |
| `UsageKnown == false` | yes | nil | false |
| `UsedPercent != nil` | yes | `round(*UsedPercent)`, min 0, no upper clamp | false |
| `Used != nil && Limit != nil && *Limit > 0` | yes | `round(Used/Limit*100)`, min 0 | false |
| `Remaining != nil && Limit != nil && *Limit > 0` | yes | `round((Limit-Remaining)/Limit*100)`, min 0 | false |
| otherwise (balance-only / non-positive Limit) | yes | nil | false |

`round` = half away from zero (`math.Round`). Always: `ID` = `Window.ID`; `Label` = `Window.Label`, or `Window.ID` when empty; `ResetHint` = `Window.ResetHint` if non-empty, else `"resets in <dur>"` from `ResetsAt` (§4.4), else `""`.

### 4.3 Source → Auth strings (closed)

| `usage.Source` | `Auth` |
|---|---|
| `oauth` | `"oauth"` |
| `api` | `"api key"` |
| `cli` | `"cli"` |
| `web` | `"browser"` |
| `local` | `"local"` |
| `cache` | `"cached"` |
| other / `""` | `""` |

### 4.4 Duration format (`<dur>` for "resets in …")

`d = time.Until(*ResetsAt)`. `d <= 1m` → the whole hint is `"resets soon"`. Else hours `h = floor(d/1h)`, minutes `m = floor((d-h)/1m)`: `h > 0` → `"%dh %dm"`; `h == 0` → `"%dm"`. Examples: `"resets in 2h 40m"`, `"resets in 35m"`.

## 5. Events

| Method | Event | Payload |
|---|---|---|
| `SetMode`, `SetBackend` | `config:changed` | `{"section":"usage"}` |
| refresher completed fetch | `usage:updated` | `{}` |
| `Snapshots`, `Mode`, disabled-gate refresher cycle | none | — |

## 6. Validation error messages (exact, checked first in each setter)

| Condition | Message (wraps `errValidation`) |
|---|---|
| `SetMode` bad value | `usage: mode %q must be one of "auto", "on", "off"` |
| `SetBackend` bad value | `usage: backend %q must be one of "off", "native", "codexbar"` |
| gate disabled | `usage disabled: <reason>` (wraps `errUsageUnavailable`; reason = toggle constant) |

## 7. Unexported seams

```go
// on Services (fields set by New; usage_test.go may override):
usageCacheDir string      // default: usage/cache.CacheDir() result (SPEC §5 Deviations)
usageFetchMu  sync.Mutex  // serialises FetchAll; refresher uses TryLock
refresherOnce sync.Once   // StartRefresher idempotence

// pure mapper, unit-tested directly:
func snapshotToDTO(snap usage.Snapshot, spec []usage.WindowSpec, now time.Time) UsageDTO
```

`snapshotToDTO` takes `now` explicitly so duration strings are golden-testable.

## 8. Test fixtures (`usage_test.go`; TDD first)

1. **Fake descriptor** — id `svc_fake_usage` registered ONCE (package-level `sync.Once` or `TestMain`) via `usage.Register`, `CacheTTL` 0, `Fetch` returning a canned snapshot from a package-level variable tests swap per-case. Config fixture enables it (`enabled = true`, `priority = 1`) with `[usage] enabled = "true"`, `backend = "native"`; `usageCacheDir` → `t.TempDir()`.
2. **Golden DTO mapping** — table over `snapshotToDTO` with fixed `now` covering every §4.2 row: synthetic dropped; unlimited; unknown-usage window keeping a ResetHint; reported UsedPercent 42.4→42 and 103.6→104; Used/Limit; Remaining/Limit; balance-only credits window driving Credits `"340 credits left"`; Resets from soonest ResetsAt (`"session resets in 2h 40m"`) and the ResetHint fallback (`"weekly on Mon"`); descriptor-order reordering; failure snapshot → `Failure` set, empty windows.
3. **Disabled gate** — each of `[usage] enabled="false"`, `backend="off"`, no providers enabled ⇒ `Snapshots` error maps to `usage_unavailable` with the matching reason; recorder shows zero events.
4. **Force TTL** — spy on options via the fake fetch path (cache-miss both ways); assert 60s vs 15m by pre-seeding a 5m-old cache file: `force=false` serves cache (`Confidence "cached"`), `force=true` refetches live.
5. **Setters** — `SetMode("on")` round-trips config to `enabled = "true"` and emits exactly one `config:changed{section:"usage"}`; invalid enum → `validation_failed`, no write, no event (golden messages §6). Same shape for `SetBackend`.
6. **Refresher** — fake clock not required: interval 10ms, cancellable ctx; assert ≥2 `usage:updated` emissions, then cancel and assert emissions stop; slow fake fetch (blocks 50ms) proves tick-skipping (no overlap: max one fetch in flight, counted via atomic in the fake); second `StartRefresher` call starts no second loop.
