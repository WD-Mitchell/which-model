//go:build !nousage

// Package fetch orchestrates usage collection: the enable gate, cache
// reads, credential resolution, per-provider fetches with deadlines,
// bounded fan-out, failure mapping, identity redaction, and deterministic
// ordering (specs/features/F14-usage-fetch/SPEC.md).
package fetch

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/WD-Mitchell/which-model/internal/config"
	"github.com/WD-Mitchell/which-model/internal/httpkit"
	"github.com/WD-Mitchell/which-model/internal/usage"
	"github.com/WD-Mitchell/which-model/internal/usage/cache"
	"github.com/WD-Mitchell/which-model/internal/usage/credential"
	"github.com/WD-Mitchell/which-model/internal/usage/provider/antigravity"
	"github.com/WD-Mitchell/which-model/internal/usage/provider/codexbar"
)

const (
	// DefaultTimeoutSec: effective per-provider timeout when neither
	// opts.Timeout nor the descriptor's Timeout is set (annex-d --timeout
	// default). F04's httpkit DefaultTimeout (10s) is unrelated — F14
	// enforces its own per-provider contexts (SPEC D4).
	DefaultTimeoutSec = 10 * time.Second

	// DefaultMaxParallel: fan-out cap when opts.MaxParallel <= 0 (SPEC D6).
	DefaultMaxParallel = 8

	// defaultCodexBarCacheTTL matches the desktop's default usage cache age.
	// CodexBar snapshots have no native descriptor from which to derive one.
	defaultCodexBarCacheTTL = 15 * time.Minute
)

// Options configures one FetchAll call. All fields optional.
type Options struct {
	Backend                config.UsageBackend // off, native, or codexbar; empty preserves native
	Refresh                bool                // skip cache reads; refetch and rewrite (annex-d --refresh-usage)
	Offline                bool                // read-only: cache only, never credentials/fetch/writes
	MaxAge                 time.Duration       // TTL override via cache.EffectiveTTL (annex-d --max-age)
	ShowIdentity           bool                // false (default): Account/Plan cleared on RETURNED snapshots
	Enabled                map[string]bool     // L1a gate, default-deny (SPEC D1)
	Timeout                time.Duration       // per-provider timeout; 0 → descriptor.Timeout → DefaultTimeoutSec
	MaxParallel            int                 // fan-out cap; <= 0 → min(active, DefaultMaxParallel)
	CacheDir               string              // "" → cache.New() (system dir); test seam (SPEC D11)
	Source                 usage.Source        // optional forced credential source; empty preserves auto
	StateDir               string              // "" resolves the platform state directory
	DisableManagedKeychain bool                // false (default) prefers the OS keychain
}

var (
	codexbarFetch            = codexbar.FetchWithSource
	codexbarFetchEnvironment = codexbar.FetchWithSourceEnvironment
)

// FetchAll selects the configured usage backend after applying the common
// enabled-provider gate. An unset backend retains the native implementation
// for direct callers; config.Default selects off.
func FetchAll(ctx context.Context, providers []string, opts Options) ([]usage.Snapshot, []credential.Warning, error) {
	switch opts.Backend {
	case config.UsageBackendOff:
		return nil, nil, nil
	case config.UsageBackendCodexBar:
		return fetchCodexBarAll(ctx, providers, opts)
	case config.UsageBackendNative, "":
		return fetchNativeAll(ctx, providers, opts)
	default:
		return nil, nil, fmt.Errorf("unknown usage backend %q", opts.Backend)
	}
}

// fetchNativeAll returns native adapter results.
func fetchNativeAll(ctx context.Context, providers []string, opts Options) ([]usage.Snapshot, []credential.Warning, error) {
	// L1a gate: skipped providers are never touched (cache/credential/fetch) — SPEC D1.
	var active []string
	for _, id := range providers {
		if opts.Enabled == nil || !opts.Enabled[id] {
			continue
		}
		active = append(active, id)
	}
	if len(active) == 0 {
		return nil, nil, nil
	}

	// Cache store (SPEC D11): "" CacheDir → the system cache dir; a
	// failure to resolve it fails the whole call.
	dir := opts.CacheDir
	if dir == "" {
		var err error
		dir, err = cache.CacheDir()
		if err != nil {
			return nil, nil, err
		}
	}
	store := &cache.Store{Dir: dir}

	// One shared plain client per call — NO Timeout: per-provider deadlines
	// come from per-provider contexts (SPEC §12 / D4).
	client := &http.Client{}

	var results []usage.Snapshot
	var warnings []credential.Warning

	// Classify requested providers: unknown ones produce failure snapshots
	// synchronously (deterministic), known ones run the pipeline in
	// bounded fan-out (SPEC §8, D6).
	type entry struct {
		id   string
		desc usage.Descriptor
	}
	var known []entry
	for _, id := range active {
		desc, err := usage.Get(id)
		if err != nil {
			// Unknown provider → failure snapshot, never a hard error
			// (SPEC §3, D2).
			results = append(results, usage.Snapshot{
				Provider: id,
				Failure:  &usage.Failure{Code: "provider_status", Message: err.Error()},
			})
			continue
		}
		known = append(known, entry{id: id, desc: desc})
	}

	if len(known) > 0 {
		limit := opts.MaxParallel
		if limit <= 0 {
			limit = min(len(known), DefaultMaxParallel)
		}
		g, gctx := errgroup.WithContext(ctx)
		g.SetLimit(limit)
		ch := make(chan usage.Snapshot, len(known))
		// Per-provider warning slots: each closure writes only its own
		// slot; flattened afterwards in provider-sorted order.
		warnSlots := make([][]credential.Warning, len(known))
		for i, e := range known {
			i, e := i, e
			g.Go(func() error {
				snap, warns := runProvider(gctx, store, client, e.id, e.desc, opts)
				warnSlots[i] = warns
				ch <- snap
				// Per-provider functions never return errors to the group
				// (failures are data); the only error a closure may return
				// is shared-context cancellation (SPEC §8).
				if err := gctx.Err(); err != nil {
					return err
				}
				return nil
			})
		}
		werr := g.Wait()
		close(ch)
		for s := range ch {
			results = append(results, s)
		}
		order := make([]int, len(known))
		for i := range order {
			order[i] = i
		}
		sort.Slice(order, func(a, b int) bool { return known[order[a]].id < known[order[b]].id })
		for _, i := range order {
			warnings = append(warnings, warnSlots[i]...)
		}
		// The group's Wait error can only come from shared-context
		// cancellation (closures never return provider errors). gctx is
		// always cancelled by Wait itself, so check the PARENT ctx.
		if werr != nil {
			return results, warnings, werr
		}
		if err := ctx.Err(); err != nil {
			return results, warnings, err
		}
	}

	// Deterministic output regardless of completion order: sorted by
	// Provider ID ascending (SPEC §8, D7). Unknown-provider snapshots
	// participate in the sort too.
	sort.Slice(results, func(i, j int) bool { return results[i].Provider < results[j].Provider })

	// Identity redaction (SPEC §10, D9): ShowIdentity false (default)
	// clears Account/Plan on every RETURNED snapshot (live, cached,
	// offline alike). Cache writes happened before this point, so the
	// cache files keep full identity for later --show-identity runs.
	if !opts.ShowIdentity {
		for i := range results {
			results[i].Account = ""
			results[i].Plan = ""
		}
	}
	return results, warnings, nil
}

// runProvider executes one provider's pipeline and returns its snapshot
// plus any warnings. It NEVER returns an error: provider failures are
// data (failure snapshots), never group errors (SPEC §8).
func runProvider(gctx context.Context, store *cache.Store, client *http.Client, id string, desc usage.Descriptor, opts Options) (usage.Snapshot, []credential.Warning) {
	var warns []credential.Warning

	// Forced `--source cache` (issue #28 review P1): cache-only read, no
	// credentials and no live fetch — mirrors Offline, but never falls
	// back to a fetch. A miss stays a failure snapshot (D-7 semantics).
	if cacheOnlySource(opts.Source) {
		ttl := cache.EffectiveTTL(desc.CacheTTL, opts.MaxAge)
		snap := store.OfflineRead(id, ttl)
		if snap.Failure != nil && snap.Failure.Code != "" {
			return snap, warns
		}
		snap.Source = usage.SourceCache
		snap.Confidence = "cached"
		return snap, warns
	}

	// Offline mode (SPEC §7): read-only — no credentials, no fetch, no
	// writes. Refresh is ignored (offline wins, SPEC D5).
	if opts.Offline {
		ttl := cache.EffectiveTTL(desc.CacheTTL, opts.MaxAge)
		snap := store.OfflineRead(id, ttl)
		if snap.Failure != nil && snap.Failure.Code != "" {
			return snap, warns // fallback_unavailable passes through
		}
		snap.Source = usage.SourceCache
		snap.Confidence = "cached"
		return snap, warns
	}

	// Cache-first read (SPEC §4a): skipped when refreshing or when the
	// provider never caches. desc.CacheTTL == 0 skips BOTH read and write.
	if !opts.Refresh && desc.CacheTTL > 0 {
		ttl := cache.EffectiveTTL(desc.CacheTTL, opts.MaxAge)
		snap, stale, rerr := store.Read(id, ttl)
		if rerr == nil && !stale && snap.Failure == nil && matchesRequestedSource(snap.Source, opts.Source) {
			snap.Source = usage.SourceCache
			snap.Confidence = "cached"
			snap.Stale = false
			return snap, warns
		}
		// miss / stale / corrupt → fall through to a live fetch
	}

	// Per-provider deadline (SPEC §5): opts.Timeout > desc.Timeout >
	// DefaultTimeoutSec. A slow provider never delays siblings beyond its
	// own deadline (D4).
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = desc.Timeout
	}
	if timeout <= 0 {
		timeout = DefaultTimeoutSec
	}
	pctx, cancel := context.WithTimeout(gctx, timeout)
	defer cancel()

	// Credential resolution (SPEC §4b): an empty Auth chain means the
	// provider needs no credential (local-tool/presence providers). A
	// forced canonical source filters the chain first (issue #28 review
	// P1): only links that can produce the requested source are walked.
	var cred usage.Credential
	if len(desc.Auth) > 0 {
		var rerr error
		var rwarns []credential.Warning
		cred, rwarns, rerr = credential.ResolveProvider(pctx, id, filterChainForSource(desc.Auth, opts.Source), client, credential.ManagedStore{
			StateDir:    opts.StateDir,
			Keychain:    credential.DefaultKeychain(),
			UseKeychain: !opts.DisableManagedKeychain,
		})
		warns = append(warns, rwarns...)
		if rerr != nil {
			if errors.Is(rerr, credential.ErrNotFound) {
				return failureSnapshot(id, usage.Failure{
					Code:    "login_required",
					Message: "no credential found for provider " + id,
				}, cred), warns
			}
			return failureSnapshot(id, MapError(rerr), cred), warns
		}
	}

	if !matchesRequestedSource(SourceFor(cred, desc.Kind), opts.Source) {
		return failureSnapshot(id, usage.Failure{
			Code:    "login_required",
			Message: "no credential found for requested source",
		}, cred), warns
	}

	// Fetch (SPEC §4c); the provider leaves Source unset — F14 alone
	// stamps it (F11 CONTRACTS §5).
	snap, ferr := desc.Fetch(pctx, cred, client)
	if ferr != nil {
		return failureSnapshot(id, MapError(ferr), cred), warns
	}
	snap.Source = SourceFor(cred, desc.Kind)

	// Cache write (SPEC §4d): failures are warnings, never errors
	// (annex-a §6 — the cache is an optimization).
	if desc.CacheTTL > 0 {
		if werr := store.Write(id, snap); werr != nil {
			warns = append(warns, credential.Warning{
				Message: "failed to cache usage for provider " + id + ": " + werr.Error(),
			})
		}
	}
	return snap, warns
}

// failureSnapshot builds a failure-carrying snapshot, scrubbing any
// credential material out of the message (SPEC §9, D8 — invariant 5).
func failureSnapshot(id string, f usage.Failure, cred usage.Credential) usage.Snapshot {
	if cred.Token != "" {
		f.Message = strings.ReplaceAll(f.Message, cred.Token, "<redacted>")
	}
	for _, v := range cred.Extra {
		if v != "" {
			f.Message = strings.ReplaceAll(f.Message, v, "<redacted>")
		}
	}
	return usage.Snapshot{Provider: id, Failure: &f}
}

// MapError converts a provider/resolver error into a canonical Failure
// (CONTRACTS §1, SPEC §9):
//
//  1. usage.AsFailure(err)             → that Failure
//  2. httpkit.AsError(err)             → Failure{Code: e.Code, Message: e.Error()}
//  3. errors.Is(err, credential.ErrNotFound)  → login_required
//  4. errors.Is(err, context.DeadlineExceeded) → timeout
//  5. otherwise                        → provider_status
func MapError(err error) usage.Failure {
	if f, ok := usage.AsFailure(err); ok {
		return f
	}
	if e, ok := httpkit.AsError(err); ok {
		return usage.Failure{Code: e.Code, Message: e.Error()}
	}
	if errors.Is(err, credential.ErrNotFound) {
		return usage.Failure{Code: "login_required", Message: err.Error()}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return usage.Failure{Code: "timeout", Message: err.Error()}
	}
	return usage.Failure{Code: "provider_status", Message: err.Error()}
}

// SourceFor maps a resolved credential's origin (plus a local-tool kind)
// to the canonical Source (SPEC §11, D10). SourceCache is never returned
// here — cached provenance is stamped directly by FetchAll.
//
// Canonical mapping:
//
//	AuthEnvVar / AuthFile / AuthKeychainGeneric / AuthKeychainInternet → SourceAPI
//	AuthOAuthDeviceFlow / AuthOAuthRefreshGrant                       → SourceOAuth
//	AuthCLIShellOut / AuthSubprocessRPC                               → SourceCLI
//	AuthBrowserCookie                                                 → SourceWeb
//	kind == KindLocalTool                                             → SourceLocal
//	anything else (incl. the zero Credential)                         → SourceAPI
func SourceFor(cred usage.Credential, kind usage.Kind) usage.Source {
	if kind == usage.KindLocalTool {
		return usage.SourceLocal
	}
	switch cred.Source {
	case usage.AuthEnvVar, usage.AuthFile, usage.AuthKeychainGeneric, usage.AuthKeychainInternet:
		return usage.SourceAPI
	case usage.AuthOAuthDeviceFlow, usage.AuthOAuthRefreshGrant:
		return usage.SourceOAuth
	case usage.AuthCLIShellOut, usage.AuthSubprocessRPC:
		return usage.SourceCLI
	case usage.AuthBrowserCookie:
		return usage.SourceWeb
	default:
		return usage.SourceAPI
	}
}
func fetchCodexBarAll(ctx context.Context, providers []string, opts Options) ([]usage.Snapshot, []credential.Warning, error) {
	active := make([]string, 0, len(providers))
	for _, id := range providers {
		if opts.Enabled == nil || !opts.Enabled[id] {
			continue
		}
		active = append(active, id)
	}
	if len(active) == 0 {
		return nil, nil, nil
	}

	// Cache store (SPEC D11, native-path parity): "" CacheDir → the system
	// cache dir; a failure to resolve it fails the whole call. Successful
	// snapshots are written through so cache-only consumers (B06 Providers
	// list, offline reads) see codexbar data too.
	dir := opts.CacheDir
	if dir == "" {
		var err error
		dir, err = cache.CacheDir()
		if err != nil {
			return nil, nil, err
		}
	}
	store := &cache.Store{Dir: dir}

	if opts.Offline || cacheOnlySource(opts.Source) {
		ttl := cache.EffectiveTTL(defaultCodexBarCacheTTL, opts.MaxAge)
		results := make([]usage.Snapshot, 0, len(active))
		for _, id := range active {
			snap := store.OfflineRead(id, ttl)
			if snap.Failure == nil {
				snap.Source = usage.SourceCache
				snap.Confidence = "cached"
			}
			if !opts.ShowIdentity {
				snap.Account = ""
				snap.Plan = ""
			}
			results = append(results, snap)
		}
		sort.Slice(results, func(i, j int) bool { return results[i].Provider < results[j].Provider })
		return results, nil, nil
	}

	limit := opts.MaxParallel
	if limit <= 0 || limit > len(active) {
		limit = min(len(active), DefaultMaxParallel)
	}
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(limit)
	results := make([]usage.Snapshot, len(active))
	// Per-provider warning slots: each closure writes only its own slot;
	// flattened afterwards in provider-sorted order.
	warnSlots := make([][]credential.Warning, len(active))
	for i, id := range active {
		i, id := i, id
		g.Go(func() error {
			if !opts.Refresh {
				ttl := cache.EffectiveTTL(defaultCodexBarCacheTTL, opts.MaxAge)
				snap, stale, err := store.Read(id, ttl)
				if err == nil && !stale && snap.Failure == nil && matchesRequestedSource(snap.Source, opts.Source) {
					snap.Source = usage.SourceCache
					snap.Confidence = "cached"
					snap.Stale = false
					if !opts.ShowIdentity {
						snap.Account = ""
						snap.Plan = ""
					}
					results[i] = snap
					return nil
				}
			}
			timeout := opts.Timeout
			if timeout <= 0 {
				timeout = DefaultTimeoutSec
			}
			pctx, cancel := context.WithTimeout(gctx, timeout)
			defer cancel()
			environment := codexbarCredentialEnvironment(pctx, id, opts)
			var snap usage.Snapshot
			var err error
			if len(environment) == 0 {
				snap, err = codexbarFetch(pctx, id, opts.Source)
			} else {
				snap, err = codexbarFetchEnvironment(pctx, id, opts.Source, environment)
			}
			if errors.Is(pctx.Err(), context.DeadlineExceeded) {
				snap = usage.Snapshot{Provider: id, Source: usage.SourceCLI, Failure: &usage.Failure{Code: "timeout", Message: "codexbar usage request timed out"}}
			} else if err != nil {
				var notFound *codexbar.BinaryNotFoundError
				message := "codexbar usage fetch failed"
				if errors.As(err, &notFound) {
					message = "codexbar CLI not found; install from https://github.com/steipete/CodexBar"
				}
				snap = usage.Snapshot{
					Provider: id,
					Source:   usage.SourceCLI,
					Failure:  &usage.Failure{Code: "provider_status", Message: message},
				}
			}
			if snap.Provider == "" {
				snap.Provider = id
			}
			// Cache write (SPEC §4d parity): failures are never cached
			// (F13 D5); write errors are warnings, never errors (annex-a
			// §6 — the cache is an optimization). Writes precede identity
			// redaction so cache files keep full identity.
			if snap.Failure == nil {
				if werr := store.Write(id, snap); werr != nil {
					warnSlots[i] = append(warnSlots[i], credential.Warning{
						Message: "failed to cache usage for provider " + id + ": " + werr.Error(),
					})
				}
			}
			if !opts.ShowIdentity {
				snap.Account = ""
				snap.Plan = ""
			}
			results[i] = snap
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return results, nil, err
	}
	if err := ctx.Err(); err != nil {
		return results, nil, err
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Provider < results[j].Provider })
	order := make([]int, len(active))
	for i := range order {
		order[i] = i
	}
	sort.Slice(order, func(a, b int) bool { return active[order[a]] < active[order[b]] })
	var warnings []credential.Warning
	for _, i := range order {
		warnings = append(warnings, warnSlots[i]...)
	}
	return results, warnings, nil
}

func codexbarCredentialEnvironment(ctx context.Context, provider string, opts Options) map[string]string {
	if provider != "antigravity" {
		return nil
	}
	store := credential.ManagedStore{
		StateDir:    opts.StateDir,
		Keychain:    credential.DefaultKeychain(),
		UseKeychain: !opts.DisableManagedKeychain,
	}
	managed, _, err := store.Resolve(ctx, provider)
	if err != nil {
		return nil
	}
	if !matchesRequestedSource(SourceFor(managed, usage.KindSubscription), opts.Source) {
		return nil
	}
	credentialsJSON, ok := antigravity.CredentialsJSON(managed.Token)
	if !ok {
		return nil
	}
	return map[string]string{antigravity.CredentialsEnvironment: credentialsJSON}
}

// matchesRequestedSource checks original provenance before it is stamped as
// cache. Cache-only reads are handled before this online eligibility check.
func matchesRequestedSource(actual, requested usage.Source) bool {
	return requested == "" || actual == requested
}
