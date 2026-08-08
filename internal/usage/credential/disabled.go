//go:build nousage

// Package credential under -tags nousage is a minimal presence stub:
// only the Warning type the fetch signature needs exists
// (specs/features/F21-usage-toggle/SPEC.md §2.2 step 9, D6). The full real
// surface is F12's credential.go (//go:build !nousage) and is NOT mirrored.
package credential

// Warning is a non-fatal credential resolution notice carried alongside
// fetch results.
type Warning struct{ Message string }
