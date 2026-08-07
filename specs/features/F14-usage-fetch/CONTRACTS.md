---
kind: feature-contracts
version: "1.0"
feature: F14-usage-fetch
project: which-model
---

# F14 — Usage Fetch: Contracts

Package: `internal/usage/fetch` (Layer 1b). Import boundary (global CONTRACTS §8): MAY import `internal/config`, `internal/security`, `internal/httpkit`; MUST NOT import `internal/catalog`, `internal/routing`, `internal/pick`.

Build tags: EVERY file in this package carries `//go:build !nousage` (annex-a §1a.2). F21-usage-toggle mirrors `FetchAll` + `Options` in the `nousage` stub (`internal/usage/fetch/disabled.go`).

---

## 1. API — `internal/usage/fetch/fetch.go`

```go
package fetch

import (
    "context"
    "time"

    "github.com/WD-Mitchell/which-model/internal/usage"
    "github.com/WD-Mitchell/which-model/internal/usage/cache"
    "github.com/WD-Mitchell/which-model/internal/usage/credential"
)

// DefaultTimeoutSec: effective per-provider timeout when neither
// opts.Timeout nor the descriptor's Timeout is set (annex-d --timeout
// default). F04's httpkit DefaultTimeout (10s) is unrelated — F14 enforces
// its own per-provider contexts (SPEC D4).
const DefaultTimeoutSec = 10 * time.Second

// DefaultMaxParallel: fan-out cap when opts.MaxParallel <= 0 (SPEC D6).
const DefaultMaxParallel = 8

// Options configures one FetchAll call. All fields optional.
type Options struct {
    Refresh    bool              // skip cache reads; refetch and rewrite (annex-d --refresh-usage)
    Offline    bool              // read-only: cache only, never credentials/fetch/writes
    MaxAge     time.Duration     // TTL override via cache.EffectiveTTL (annex-d --max-age)
    ShowIdentity bool            // false (default): Account/Plan cleared on RETURNED snapshots
    Enabled    map[string]bool   // L1a gate, default-deny (SPEC D1)
    Timeout    time.Duration     // per-provider timeout; 0 → descriptor.Timeout → DefaultTimeoutSec
    MaxParallel int              // fan-out cap; <= 0 → min(active, DefaultMaxParallel)
    CacheDir   string            // "" → cache.New() (system dir); test seam (SPEC D11)
}

// FetchAll returns one Snapshot per requested AND enabled provider, sorted
// by Provider ID. Partial failures are snapshots with Failure set; err is
// non-nil only on shared-context cancellation (SPEC "Error behaviour").
func FetchAll(ctx context.Context, providers []string, opts Options) ([]usage.Snapshot, []credential.Warning, error)

// MapError converts a provider/resolver error into a canonical Failure:
//   1. usage.AsFailure(err)      → that Failure
//   2. httpkit.AsError(err)      → Failure{Code: e.Code, Message: e.Error()}
//   3. errors.Is(err, credential.ErrNotFound) → login_required
//   4. errors.Is(err, context.DeadlineExceeded) → timeout
//   5. otherwise                → provider_status
// (SPEC §9). F14 additionally scrubs the resolved credential's Token and
// Extra values from the message before it reaches a returned snapshot.
func MapError(err error) usage.Failure

// SourceFor maps a resolved credential's origin (plus a local-tool kind)
// to the canonical Source (SPEC §11). SourceCache is never returned here —
// cached provenance is stamped directly by FetchAll.
func SourceFor(cred usage.Credential, kind usage.Kind) usage.Source
```

## 2. Cross-feature surface consumed (pinned, cited for implementers)

| Symbol | Source | Producer |
|---|---|---|
| `usage.Descriptor`, `usage.AuthSource`, `usage.Credential`, `usage.Snapshot`, `usage.Failure`, `usage.FailureError`, `usage.AsFailure`, `usage.Kind`, `usage.Get`, `usage.IDs` | `internal/usage` — `specs/features/F11-usage-types/CONTRACTS.md` | F11-T2/T3/T4/T5 |
| `credential.ResolveChain`, `credential.Warning`, `credential.ErrNotFound` | `internal/usage/credential` — `specs/features/F12-credentials/CONTRACTS.md §1` | F12-T1/T7 |
| `cache.New`, `cache.Store{Read,Write,OfflineRead}`, `cache.EffectiveTTL`, `cache.ErrCacheMiss` | `internal/usage/cache` — `specs/features/F13-usage-cache/CONTRACTS.md §1` | F13-T1/T2/T3/T4/T6 |
| `httpkit.AsError` (defensive only — F14 never constructs an httpkit client; transport is plain `&http.Client{}` per F11 `FetchFunc`, `specs/DEFERRED.md` D1) | `internal/httpkit` — `specs/features/F04-http/CONTRACTS.md` |

## 3. Ownership summary

| Surface | Value |
|---|---|
| Config keys owned | none |
| Flags owned | none — `Options` mirrors `--refresh-usage`, `--offline`, `--max-age`, `--timeout`, `--show-identity`, `--max-parallel`; cobra wiring is F24 (`docs/plan/annex-d-cli-reference.md` §1) |
| Error codes added | none — all codes canonical (global CONTRACTS §1.6); `MapError` only maps |
| JSON shapes emitted | none (consumes/creates cache files via F13; snapshots flow in memory) |
| Dependencies added | `golang.org/x/sync` (errgroup only) |
| Depends on | F04, F11, F12, F13 (per `specs/DEPENDENCY-GRAPH.md` §2) |
| Blocks | F15, F16, F17, F21, F24 (per `specs/DEPENDENCY-GRAPH.md` §2) |
