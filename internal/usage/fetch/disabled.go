//go:build nousage

// Package fetch under -tags nousage has no fetch fan-out: FetchAll always
// returns the sentinel (specs/features/F21-usage-toggle/SPEC.md §2.2 step 9).
package fetch

import (
	"context"
	"time"

	"github.com/WD-Mitchell/which-model/internal/usage"
	"github.com/WD-Mitchell/which-model/internal/usage/credential"
)

// Options configures one FetchAll call. Signature-identical to F14's real
// struct (CONTRACTS §4 — field set pinned; do not rename fields).
type Options struct {
	Refresh      bool
	Offline      bool
	MaxAge       time.Duration
	ShowIdentity bool
	Enabled      map[string]bool
	Timeout      time.Duration
	MaxParallel  int
	CacheDir     string
}

// FetchAll in the compiled-out build always returns the sentinel error.
func FetchAll(ctx context.Context, providers []string, opts Options) ([]usage.Snapshot, []credential.Warning, error) {
	return nil, nil, usage.ErrUsageCompiledOut
}
