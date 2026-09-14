//go:build !nousage

package credential

import (
	"errors"

	"github.com/WD-Mitchell/which-model/internal/securestore"
	"github.com/WD-Mitchell/which-model/internal/usage"
)

type refusedKeychain struct{ err error }

func (s refusedKeychain) Get(string, string) (string, error) { return "", s.err }
func (s refusedKeychain) Set(string, string, string) error   { return s.err }
func (s refusedKeychain) Delete(string, string) error        { return s.err }

// KeychainFor preserves personal platform defaults unless explicitly opted in.
// Managed authority chooses the native store independently of that preference.
func KeychainFor(native bool) ManagedKeychainStore {
	p, err := readCompanyPolicy()
	if err != nil {
		return refusedKeychain{err}
	}
	if native || p.Managed {
		return securestore.Native()
	}
	return DefaultKeychain()
}

type nativeFailure struct{ state *securestore.Error }

func (e *nativeFailure) Error() string { return e.state.Error() }
func (e *nativeFailure) Unwrap() []error {
	return []error{e.state, usage.NewFailureError("keychain_unavailable", e.state.Error())}
}

func nativeStoreFailure(err error) error {
	if errors.Is(err, &securestore.Error{Kind: securestore.Missing}) || errors.Is(err, keyringNotFound) || errors.Is(err, ErrNotFound) {
		return ErrNotFound
	}
	var state *securestore.Error
	if !errors.As(err, &state) {
		state = &securestore.Error{Kind: securestore.Unavailable}
	}
	return &nativeFailure{state: state}
}
