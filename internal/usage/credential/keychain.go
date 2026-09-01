//go:build !nousage

package credential

import (
	"context"
	"errors"
	"fmt"

	"github.com/WD-Mitchell/which-model/internal/security"
	"github.com/WD-Mitchell/which-model/internal/usage"
)

// KeychainStore abstracts the OS keychain so tests use a fake.
type KeychainStore interface {
	Get(service, account string) (string, error)
}

// ManagedKeychainStore extends keychain lookup with the mutations required
// for credentials created and removed by which-model.
type ManagedKeychainStore interface {
	KeychainStore
	Set(service, account, value string) error
	Delete(service, account string) error
}

// keyringNotFound mirrors github.com/zalando/go-keyring's ErrNotFound
// sentinel without importing the package here (go-keyring is referenced
// only from the darwin build adapter, SPEC D2). The concrete value is
// declared per-platform: keychain_darwin.go (keyring.ErrNotFound) and
// keychain_other.go (message-identical mirror); exactly one is compiled.

// UnavailableKeychain is the no-op store used on platforms without a
// keychain adapter: keychain sources degrade to "no candidate", never a
// crash (SPEC §7, D12). Declared here (untagged) so tests exercise it on
// every platform; DefaultKeychain selects it on non-darwin builds.
type UnavailableKeychain struct{}

// Get always reports the item as not found.
func (UnavailableKeychain) Get(service, account string) (string, error) {
	return "", ErrNotFound
}

// Set reports that this platform has no writable keychain.
func (UnavailableKeychain) Set(service, account, value string) error {
	return ErrNotFound
}

// Delete reports that this platform has no writable keychain.
func (UnavailableKeychain) Delete(service, account string) error {
	return ErrNotFound
}

// KeychainResolver resolves a generic-password item. Not-found →
// ErrNotFound; other store errors → keychain_unavailable.
type KeychainResolver struct {
	Store   KeychainStore
	Service string
	Account string
}

// Resolve retrieves the item. A missing item (store ErrNotFound or empty
// value) or a shape-invalid token is "no candidate" → ErrNotFound. Any
// other store error is a hard keychain_unavailable error whose message is
// deliberately store-error-free: deterministic, and it can never leak
// partial secret material (SPEC §7).
func (r *KeychainResolver) Resolve(ctx context.Context) (usage.Credential, error) {
	v, err := r.Store.Get(r.Service, r.Account)
	if err != nil {
		if errors.Is(err, keyringNotFound) {
			return Credential{}, ErrNotFound
		}
		return Credential{}, usage.NewFailureError(
			"keychain_unavailable",
			fmt.Sprintf("keychain lookup failed for service %q", r.Service),
		)
	}
	if v == "" {
		return Credential{}, ErrNotFound
	}
	if err := security.ValidateOpaqueToken(v); err != nil {
		return Credential{}, ErrNotFound
	}
	return Credential{Token: v, Source: usage.AuthKeychainGeneric}, nil
}
