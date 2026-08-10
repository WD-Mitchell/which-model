//go:build !nousage

// Package fetch orchestrates CodexBar usage collection: the enable gate,
// bounded fan-out, failure mapping, identity redaction, and deterministic
// ordering.
package fetch

import (
	"context"
	"errors"
	"sort"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/WD-Mitchell/which-model/internal/usage"
	"github.com/WD-Mitchell/which-model/internal/usage/provider/codexbar"
)

const (
	// DefaultTimeoutSec is retained for callers that use the fetch package's
	// public tuning constants. CodexBar itself applies its 30-second deadline.
	DefaultTimeoutSec  = 10 * time.Second
	DefaultMaxParallel = 8
)

// Options configures one FetchAll call. All fields optional.
type Options struct {
	Refresh      bool
	Offline      bool
	MaxAge       time.Duration
	ShowIdentity bool
	Enabled      map[string]bool
	Timeout      time.Duration
	MaxParallel  int
	CacheDir     string
	Source       usage.Source
}

// fetchProvider is the default two-argument seam used by ordinary fetches.
var fetchProvider = codexbar.Fetch

// fetchProviderWithSource is used only when the command explicitly selects a
// CodexBar source. Keeping both seams makes provider fetching deterministic in
// tests without changing the canonical Fetch signature.
var fetchProviderWithSource = codexbar.FetchWithSource

// FetchAll returns one Snapshot per requested AND enabled provider, sorted by
// Provider ID. Partial failures are snapshots with Failure set; err is
// non-nil only on shared-context cancellation.
func FetchAll(ctx context.Context, providers []string, opts Options) ([]usage.Snapshot, []usage.Warning, error) {
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

	limit := opts.MaxParallel
	if limit <= 0 {
		limit = min(len(active), DefaultMaxParallel)
	}
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(limit)
	results := make(chan usage.Snapshot, len(active))
	for _, id := range active {
		id := id
		g.Go(func() error {
			providerCtx := gctx
			cancel := func() {}
			if opts.Timeout > 0 {
				providerCtx, cancel = context.WithTimeout(gctx, opts.Timeout)
			}
			defer cancel()
			var snap usage.Snapshot
			var err error
			if opts.Offline || opts.Source == usage.SourceCache {
				snap = usage.Snapshot{
					Provider: id,
					Source:   usage.SourceCache,
					Failure:  &usage.Failure{Code: "fallback_unavailable", Message: "offline usage cache is unavailable"},
				}
			} else if opts.Source != "" {
				snap, err = fetchProviderWithSource(providerCtx, id, opts.Source)
			} else {
				snap, err = fetchProvider(providerCtx, id)
			}
			if err != nil {
				var notFound *codexbar.BinaryNotFoundError
				if errors.As(err, &notFound) {
					snap = usage.Snapshot{
						Provider: id,
						Failure: &usage.Failure{
							Code:    "provider_status",
							Message: "codexbar CLI not found; install from https://github.com/steipete/CodexBar",
						},
					}
				} else {
					snap = usage.Snapshot{
						Provider: id,
						Failure:  &usage.Failure{Code: "provider_status", Message: "codexbar usage fetch failed"},
					}
				}
			}
			if snap.Provider == "" {
				snap.Provider = id
			}
			if !opts.ShowIdentity {
				snap.Account = ""
				snap.Plan = ""
			}
			results <- snap
			if err := gctx.Err(); err != nil {
				return err
			}
			return nil
		})
	}
	waitErr := g.Wait()
	close(results)
	out := make([]usage.Snapshot, 0, len(active))
	for snap := range results {
		out = append(out, snap)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Provider < out[j].Provider })
	if waitErr != nil {
		return out, nil, waitErr
	}
	if err := ctx.Err(); err != nil {
		return out, nil, err
	}
	return out, nil, nil
}
