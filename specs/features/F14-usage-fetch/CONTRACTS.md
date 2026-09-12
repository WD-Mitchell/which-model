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

    "github.com/WD-Mitchell/which-model/internal/config"
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
    Backend    config.UsageBackend // off, native, codexbar; empty retains native
    Source     usage.Source     // forced credential source; empty preserves auto precedence
    Refresh    bool              // skip cache reads; refetch and rewrite (annex-d --refresh-usage)
    Offline    bool              // read-only: cache only, never credentials/fetch/writes
    MaxAge     time.Duration     // TTL override via cache.EffectiveTTL (annex-d --max-age)
    ShowIdentity bool            // false (default): Account/Plan cleared on RETURNED snapshots
    Enabled    map[string]bool   // L1a gate, default-deny (SPEC D1)
    Timeout    time.Duration     // per-provider timeout; 0 → descriptor.Timeout → DefaultTimeoutSec
    MaxParallel int              // fan-out cap; <= 0 → min(active, DefaultMaxParallel)
    CacheDir   string            // "" → cache.New() (system dir); test seam (SPEC D11)
    StateDir   string            // "" → platform state dir; managed credential fallback
    DisableManagedKeychain bool  // false (default) prefers OS keychain
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
| `credential.ResolveProvider`, `credential.ManagedStore`, `credential.Warning`, `credential.ErrNotFound` | `internal/usage/credential` — `specs/features/F12-credentials/CONTRACTS.md §5.1` | F12 |
| `cache.New`, `cache.Store{Read,Write,OfflineRead}`, `cache.EffectiveTTL`, `cache.ErrCacheMiss` | `internal/usage/cache` — `specs/features/F13-usage-cache/CONTRACTS.md §1` | F13-T1/T2/T3/T4/T6 |
| `httpkit.AsError` (defensive only — F14 never constructs an httpkit client; transport is plain `&http.Client{}` per F11 `FetchFunc`, `specs/DEFERRED.md` D1) | `internal/httpkit` — `specs/features/F04-http/CONTRACTS.md` | F04 |

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

## Review regression contract (#180)

| Scenario | Required result |
|---|---|
| Fresh CodexBar cache / two sequential online calls | No credential/subprocess/write on hit; one total live fetch |
| Missing, stale, corrupt, or Refresh cache | One live fetch; successful result cached before identity redaction |
| Two-minute cache, MaxAge 60s / 15m | Fetch / hit respectively |

## Review regression contract (#181)

| Scenario | Required result |
|---|---|
| Explicit CodexBar timeout / absent timeout | Provider context uses requested budget / 10s default |
| Earlier parent deadline or cancellation | Parent remains the upper bound and returns batch error |
| Blocked provider and successful sibling | `timeout` data for blocked provider; sibling retained |

## Review regression contract (#184)

| Scenario | Required result |
|---|---|
| Forced api with managed OAuth or forced oauth with managed API key | `login_required`; zero native fetches; no credential material in error |
| Matching managed credential or empty source | Fetch succeeds with resolved source |
| Forced online source with matching cache provenance | Cache hit stamped cache/cached after original source check |
| Forced online source with mismatched/unknown cache provenance | Matching live path; no incompatible cache reuse |
| Source cache or Offline, including Refresh | Cache-only behavior remains unchanged |

## Managed lookup deadline correction — #181 review

The provider budget bounds managed keychain lookup as well as the CodexBar
process. Because the OS keychain interface cannot cancel an active prompt,
return on context cancellation and discard the late result; permit at most four
outstanding such calls process-wide. Do not launch CodexBar after the lookup
exhausts the budget. Pin blocked-keychain deadline and earlier-parent tests.


## Cache source correction — #180 review

Before reusing an online CodexBar cache entry for an explicitly requested source,
compare its original source with the request. Only then stamp the returned
snapshot as cached. A mismatching cache entry causes live collection with the
requested source. Explicit cache-only reads retain their existing policy.


## Company-policy extension (#282)

FetchAll checks protected policy before enabled-provider cache, credential or network activity, including direct callers and explicit backend/storage options. Disallowed providers and unapproved CodexBar delegation are refused. Disabled providers remain untouched. The guard is independent of ordinary configuration; per-provider API boundaries also recheck authorization.

This intentionally supersedes unrestricted operation for enrolled installations only;
see the [F01 managed-policy contract](../F01-config/MANAGED-POLICY.md). Pinned evidence:
`TestManagedConfigurationPrecedence`, `TestCompanyCredentialFallbackHasNoFileSideEffects`,
and native `TestNativeManagedOperationBoundaries` on macOS, Windows and Linux.


## Company privacy correction (#284)

Options/API/usage DTOs remain unchanged. Personal cache identity and error rendering remain as before. Company tests pin raw native/delegated error-canary removal and lock-remediation preservation. The #287 approved-installation consumer governs delegation; shared cache/output minimization does not itself grant approval.

Governing shared contract: `specs/features/F13-usage-cache/MANAGED-RETENTION.md`.
Decision: requester-approved optional company defaults and advisory audit handling;
this supersedes conflicting personal-only persistence statements for company mode.


## Approved CodexBar correction (#287)

The requester requires company CodexBar to remain disabled until an administrator approves a specific installation. Personal behavior stays unchanged. [APPROVED-CODEXBAR.md](APPROVED-CODEXBAR.md) is normative. Unapproved delegation is refused before cache/credential effects. Approved offline and fresh-cache paths execute nothing; a cache miss preflights the image/config before credential inputs, then the adapter re-verifies before invocation. Per-provider errors stay partial results. Existing cache/source/timeout tests remain binding. New pinned tests: TestCompanyCodexBarPreflightPrecedesCredentialsAndProcess, TestCompanyCodexBarCacheOnlyNeverPreflightsOrDelegates, TestCompanyCodexBarOutputAndTimeoutMatrix, TestNativeCompanyCodexBarApproval.

The requester-approved PR #314 review correction and pinned regression matrix are
in [APPROVED-CODEXBAR.md, Source and cache correction](APPROVED-CODEXBAR.md#source-and-cache-correction-pr-314-review).
The adapter and delegated cache consumer share this pure internal-package helper:

```go
func CompanySourceMatches(provider string, actual, requested usage.Source) bool
```

It accepts auto, equal canonical sources, and the reviewed Antigravity/Windsurf
local result for CLI selection. It performs no authorization or effects. Live
normalization rejects unrecognized provider/label pairs before this comparison.
Canonical Snapshot/Source/Options types remain unchanged. Personal fetching and
native credential-source matching retain their existing rules.
