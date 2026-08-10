//go:build !nousage

package cache

import "time"

// EffectiveTTL returns maxAge when maxAge > 0, else base (SPEC §3;
// the --max-age override semantics). Both 0 → 0 (provider uncached).
func EffectiveTTL(base time.Duration, maxAge time.Duration) time.Duration {
	if maxAge > 0 {
		return maxAge
	}
	return base
}
