//go:build nousage

// Package cache under -tags nousage has no cache: CacheDir returns the
// sentinel directly (specs/features/F21-usage-toggle/SPEC.md §2.2 step 9, D6).
package cache

import "github.com/WD-Mitchell/which-model/internal/usage"

// CacheDir mirrors F13's CacheDir; the compiled-out build has no cache.
func CacheDir() (string, error) { return "", usage.ErrUsageCompiledOut }
