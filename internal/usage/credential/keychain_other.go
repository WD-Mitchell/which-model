//go:build !darwin && !nousage

package credential

import "errors"

// This file holds the non-macOS keychain adapter. The UnavailableKeychain
// store itself is declared untagged in keychain.go so tests can exercise
// it on every platform (SPEC §7, D12).

// keyringNotFound mirrors github.com/zalando/go-keyring's ErrNotFound
// (same message, same semantics) so the resolver's not-found detection is
// platform-independent; on non-darwin builds go-keyring is never linked.
var keyringNotFound = errors.New("secret not found in keyring")

// DefaultKeychain returns the unavailable keychain adapter: keychain
// sources are macOS-specific, so a missing capability degrades to "no
// candidate", never a crash.
func DefaultKeychain() KeychainStore { return UnavailableKeychain{} }
