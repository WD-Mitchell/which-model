---
kind: feature-spec
version: "1.0"
feature: F13-usage-cache
project: which-model
---

# F13 — Usage Cache: per-provider snapshots, TTL, atomic writes, offline reads

## Purpose

F13 gives the usage subsystem a per-provider on-disk cache: one JSON file per provider ID under the user cache directory, TTL-based staleness, atomic replacement writes, explicit invalidation, and a read-only offline path that degrades to the `fallback_unavailable` failure code. It sits between F11's `Snapshot` type and F14's fetch orchestration; providers never touch the cache directly.

## Behaviour

1. **Layout (decision D1).** Cache root = `os.UserCacheDir()/which-model/usage-cache/`; per-provider file `<provider-id>.json` (`os.UserCacheDir()` resolves `$XDG_CACHE_HOME` on Linux, `~/Library/Caches` on macOS). Annex-a §6 names the `usage/` subdirectory; the assignment pins `usage-cache/` — the assignment wins. `CacheDir() (string, error)` returns the absolute root; `New()` creates it with mode 0700 (`os.MkdirAll(dir, 0o700)`) and returns `*Store`.
2. **File schema (decision D2).** Each cache file is the JSON object `{"snapshot": <usage.Snapshot>, "fetched_at": <RFC3339 time.Time>}` (annex-a §6: store the snapshot plus its fetch time). `fetched_at` is the wall-clock time the snapshot was WRITTEN, set by `Write` itself — never trusted from the provider. Corrupt or schema-invalid files yield an error from `Read`; F14 treats that as a miss and refetches (SPEC F14 §7, decision D3). `snapshot` is stored untouched (provider field values round-trip verbatim, including `Account`/`Plan` identity — the cache is private data with 0600 files).
3. **TTL semantics.** `Read(providerID, ttl)` reports `stale = now - fetched_at > ttl`. `ttl <= 0` means "no cache": `Read` returns `ErrCacheMiss` without touching the file (SPEC §6; codex/openrouter/sakana use `CacheTTL 0` — annex-a §6). `EffectiveTTL(base, maxAge)` picks `maxAge` when `maxAge > 0`, else `base` (the `--max-age` override; annex-d `--max-age <duration>`).
4. **Write is atomic (decision D4).** `Write` serializes `{snapshot, fetched_at: now}`, then: `os.CreateTemp(dir, ".tmp-"+providerID+"-*")`, write, `Sync`, `Chmod(0o600)`, `Close`, `Rename` over the target. A crash leaves either the old file or the new file, never a partial one. Directory is created if missing (0700). Returned errors are plain `error` (no Failure code — the cache is not a credential/endpoint path); F14 converts write failures into warnings (SPEC F14 §7).
5. **Never cache failures.** `Write` rejects snapshots whose `Failure.Code != ""` with a plain error (`errors.New("refusing to cache a failed snapshot")`) — annex-a §6: failures are never cached; F14 maps the fetch error to a Failure instead. Rate-limit backoff caching (Retry-After / TTL×4) is explicitly deferred to the provider features (F15+) and out of scope here (decision D5).
6. **Read semantics.** `Read(providerID, ttl) (usage.Snapshot, bool, error)`: missing file → `ErrCacheMiss` (a `fmt.Errorf("cache miss for provider %q: %w", id, ErrCacheMiss)` wrapping the sentinel — `errors.Is` works). Corrupt/oversized/undecodable file → plain error (never a Failure code; F14 refetches). Valid file → the stored snapshot, `stale` bool, nil error. The snapshot is returned **untouched**: `Source`, `Confidence`, `Stale` are stamped by F14 after read (global CONTRACTS §1.5 semantics; decision D6). Files larger than 4 MiB are rejected as corrupt (decision D7).
7. **Invalidate.** `Invalidate(providerID)` removes the file; a missing file → nil (idempotent). Used by `--refresh-usage`-adjacent flows at F24's discretion (annex-d §1) — F13 provides the primitive, not the flag.
8. **OfflineRead.** `OfflineRead(providerID, ttl) usage.Snapshot`: missing/unreadable file → `Snapshot{Provider: providerID, Failure: Failure{Code: "fallback_unavailable", Message: "offline and no cached usage"}}` (annex-d: offline + missing cache → exit 1 `fallback_unavailable`; the Failure carries it). Fresh file → stored snapshot. Stale file → stored snapshot with `Stale: true` (consumer decides; F14 shows it with the stale marker — global CONTRACTS §1.5). NEVER writes or creates anything (read-only path; decision D8).
9. **Flag ownership.** F13 owns the usage-domain semantics of `--refresh-usage` (skip cache reads), `--max-age <duration>` (override TTL), and `--offline` (use `OfflineRead`); the cobra flag wiring itself is F24's (annex-d §1). Default TTLs per provider are set by the provider descriptors (F15+), not here (annex-a §6: claude 1800s/60s, codex 0, openrouter/sakana 0, other HTTP providers 60s — informative only).
10. **Build tags.** Every file in `internal/usage/cache/` carries `//go:build !nousage` (annex-a §1a.2). F21 provides the `nousage` package-presence stub; nothing under `nousage` imports this package's real symbols.

## Error behaviour

- `Read`/`Write`/`Invalidate` return plain Go errors (no `usage.FailureError`): cache problems never masquerade as provider failures. `ErrCacheMiss` sentinel (wrapped) for absent entries.
- `OfflineRead` cannot fail — absence is expressed as `Snapshot.Failure = fallback_unavailable`.
- `Write` refuses failed snapshots with a plain error; F14 turns that into a warning, never an error exit.

## Decisions

| # | Decision | Value | Rationale |
|---|---|---|---|
| D1 | Cache subdirectory | `which-model/usage-cache/` | Assignment pins `usage-cache/`; overrides annex-a §6's `usage/`. Recorded so F24's `--cache-dir`-adjacent decisions cite the right path. |
| D2 | File schema | `{"snapshot": …, "fetched_at": …}` | Annex-a §6. `fetched_at` is write-time only, set by the cache, never read from the payload. |
| D3 | Corrupt file | plain error; F14 refetches | A torn/hand-edited file must not fail the command; cache is an optimization, never a correctness input. |
| D4 | Atomic write | CreateTemp + Sync + Chmod 0600 + Rename; 0700 dirs | Crash-safe, no partial files; 0600 keeps identity data (Account/Plan) private (global SPEC §6 invariant 8). |
| D5 | Failure caching | never; rate-limit backoff deferred to F15+ | Annex-a §6: failures are never cached; Retry-After/TTL×4 is provider-policy, owned with the adapters. |
| D6 | Stamped fields | `Source`/`Confidence`/`Stale` set by F14 after read, never stored by F13 | Global CONTRACTS §1.5: `Confidence=cached` + `Stale` are fetch-layer presentation; the cache stores raw provider data. |
| D7 | Oversized file bound | 4 MiB → corrupt | Bounded reads everywhere (global SPEC §6 invariant 3); snapshots are small (≤ tens of KiB), 4 MiB is 100× headroom. |
| D8 | OfflineRead | never writes, never fails | Offline path must be side-effect-free and total; absence → `fallback_unavailable` (annex-d exit 1). |

## Out of scope

- Fetch orchestration, cache-first/refresh logic, `--refresh-usage`/`--max-age`/`--offline` cobra wiring — F14 (orchestration), F24 (flags).
- Provider default TTLs and rate-limit backoff caching — F15–F17.
- `nousage` stub file for this package — F21.
- Cache cleanup/pruning (single-file-per-provider; no retention policy needed).
