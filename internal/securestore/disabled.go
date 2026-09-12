//go:build nousage || (!darwin && !windows && !linux)

package securestore

// Native has no linked credential adapter in compiled-out builds.
func Native() Store { return unavailableStore{} }
