---
kind: feature-contracts
version: "1.0"
feature: F13-usage-cache
project: which-model
---

# F13 — Usage Cache: Contracts

Package: `internal/usage/cache` (Layer 1b). Import boundary (global CONTRACTS §8): MAY import `internal/config`, `internal/security`, `internal/httpkit`; MUST NOT import `internal/catalog`, `internal/routing`, `internal/pick`.

Build tags: EVERY file in this package carries `//go:build !nousage` (annex-a §1a.2). The `nousage`-tagged package-presence stub is owned by F21-usage-toggle.

---

## 1. API — `internal/usage/cache/cache.go`

```go
package cache

import (
    "time"

    "github.com/WD-Mitchell/which-model/internal/usage"
)

// ErrCacheMiss is the wrapped sentinel for "no cache entry".
// Match with errors.Is(err, cache.ErrCacheMiss).
var ErrCacheMiss = errors.New("cache miss")

// Store reads and writes per-provider cache files under Dir.
// Zero value is not usable; construct via New().
type Store struct {
    Dir string
}

// CacheDir returns the absolute cache root:
// os.UserCacheDir()/which-model/usage-cache (SPEC D1). Error only if
// UserCacheDir fails (no HOME/XDG_CACHE_HOME).
func CacheDir() (string, error)

// New creates (if needed, mode 0700) the cache dir and returns a Store.
func New() (*Store, error)

// Read loads one provider's snapshot. stale = now - fetched_at > ttl.
//   - ttl <= 0        → ErrCacheMiss (no cache for this provider)
//   - missing file    → error wrapping ErrCacheMiss
//   - corrupt/oversized (> 4 MiB) file → plain error (F14 refetches)
//   - valid file      → stored snapshot UNTOUCHED (F14 stamps
//     Source/Confidence/Stale afterwards; SPEC D6), stale bool, nil
func (s *Store) Read(providerID string, ttl time.Duration) (usage.Snapshot, bool, error)

// Write atomically replaces <providerID>.json (SPEC D4): temp file +
// Sync + Chmod 0600 + Rename; dir created 0700 if missing. Refuses
// snapshots with Failure.Code != "" (never cache failures; SPEC D5).
func (s *Store) Write(providerID string, snap usage.Snapshot) error

// Invalidate removes <providerID>.json. Missing file → nil (idempotent).
func (s *Store) Invalidate(providerID string) error

// OfflineRead is the read-only offline path (SPEC D8): never writes,
// never fails. Missing/unreadable → Snapshot{Provider, Failure:
// {fallback_unavailable, "offline and no cached usage"}}. Stale file →
// stored snapshot with Stale=true. Fresh file → stored snapshot as-is.
func (s *Store) OfflineRead(providerID string, ttl time.Duration) usage.Snapshot

// EffectiveTTL returns maxAge when maxAge > 0, else base (SPEC D3;
// --max-age override semantics). Both 0 → 0.
func EffectiveTTL(base time.Duration, maxAge time.Duration) time.Duration
```

## 2. Cache file JSON shape — `internal/usage/cache/cache.go` (writer) / `cache_read.go` (reader)

```json
{
  "snapshot":   { "provider": "codex", "windows": [ ... ], "fetched_at": "...", "source": "", "confidence": "", "usage_known": false, "account": "...", "plan": "...", "stale": false, "error": null },
  "fetched_at": "2026-08-07T12:00:00Z"
}
```

- `snapshot` is a full `usage.Snapshot` per `specs/global/CONTRACTS.md` §1.5 (JSON tags as defined there).
- `fetched_at` is written by `Write` (time.Now, RFC3339); `Read` uses it for the staleness computation and does NOT trust any time inside `snapshot`.
- Internal type: `type cacheFile struct { Snapshot usage.Snapshot `json:"snapshot"`; FetchedAt time.Time `json:"fetched_at"` }` (unexported).

## 3. Ownership summary

| Surface | Value |
|---|---|
| Config keys owned | none |
| Flags owned | usage-domain semantics only: `--refresh-usage` (skip cache reads), `--max-age <duration>` (TTL override via `EffectiveTTL`), `--offline` (use `OfflineRead`) — cobra wiring is F24 (`specs/features/F24-cli-usage/`); see `docs/plan/annex-d-cli-reference.md` §1 |
| Error codes added | none — `fallback_unavailable` is canonical (global CONTRACTS §1.6); cache problems are plain errors (SPEC "Error behaviour") |
| JSON shapes emitted | cache file above (the only JSON this package writes) |
| Dependencies added | none (stdlib) |
| Depends on | F11 (per `specs/DEPENDENCY-GRAPH.md` §2) |
| Blocks | F14 (per `specs/DEPENDENCY-GRAPH.md` §2) |
