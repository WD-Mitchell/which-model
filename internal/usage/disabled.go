//go:build nousage

// Package usage under -tags nousage: canonical tag-free types (types.go) plus
// this stub surface (SPEC §2.2 step 9, D5/D6). No credential resolver, cookie
// reader, keychain call, or provider endpoint constant exists in the binary.
package usage

import "context"

// Descriptor is a nousage-only minimal stub shape. F11's real Descriptor lives
// in descriptor.go (//go:build !nousage); nothing dereferences this stub's
// fields under nousage, so only the identity fields exist. MUST NOT be declared
// if F11 ever makes Descriptor tag-free (R1 guards types.go).
type Descriptor struct {
	ID          string
	DisplayName string
}

// Options is the nousage-only minimal stub for the fetch options. F11's real
// Options type lives in a !nousage file; the stub needs only the type identity.
type Options struct{}

// Registry mirrors F11's Registry: returns nil in the compiled-out build —
// the binary contains no provider adapters at all (SPEC §2.2 step 6).
func Registry() []Descriptor { return nil }

// Lookup mirrors F11's Lookup: always false in the compiled-out build.
func Lookup(id string) (Descriptor, bool) { return Descriptor{}, false }

// Fetch mirrors F11's Fetch (context, []string providers, Options): always the
// sentinel error. Callers compare with errors.Is (SPEC §2.2 step 8).
func Fetch(context.Context, []string, Options) ([]Snapshot, error) {
	return nil, ErrUsageCompiledOut
}

// CacheDir returns the usage cache directory; the compiled-out build has no
// cache, so it returns the sentinel directly (annex §1a.2 form — no
// usage↔cache delegation, decision D6).
func CacheDir() (string, error) { return "", ErrUsageCompiledOut }
