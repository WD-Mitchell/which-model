//go:build darwin && !nousage

package credential

import (
	"github.com/zalando/go-keyring"
)

// This is the ONLY file in the package that imports
// github.com/zalando/go-keyring (SPEC decision D2).

// keyringNotFound is go-keyring's not-found sentinel, exposed to the
// platform-independent resolver (keychain.go).
var keyringNotFound = keyring.ErrNotFound

// DarwinKeychain wraps go-keyring's generic-password API.
type DarwinKeychain struct{}

// Get retrieves the generic-password item for service/account.
func (DarwinKeychain) Get(service, account string) (string, error) {
	return keyring.Get(service, account)
}

// Set stores a generic-password item for service/account.
func (DarwinKeychain) Set(service, account, value string) error {
	return keyring.Set(service, account, value)
}

// Delete removes a generic-password item for service/account.
func (DarwinKeychain) Delete(service, account string) error {
	return keyring.Delete(service, account)
}

// DefaultKeychain returns the macOS keychain adapter.
func DefaultKeychain() ManagedKeychainStore { return DarwinKeychain{} }
