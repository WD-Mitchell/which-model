//go:build nousage

// Package fetch under -tags nousage has no fetch fan-out: FetchAll always
// returns the compiled-out sentinel.
package fetch

import (
	"context"
	"time"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

// Options mirrors the real fetch surface so common callers compile under both
// build variants.
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

// FetchAll in the compiled-out build always returns the sentinel error.
func FetchAll(ctx context.Context, providers []string, opts Options) ([]usage.Snapshot, []usage.Warning, error) {
	return nil, nil, usage.ErrUsageCompiledOut
}
