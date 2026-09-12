//go:build !nousage

package credential

import (
	"context"
	"errors"
	"io/fs"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/company"
	"github.com/WD-Mitchell/which-model/internal/securestore"
)

type policyKeychain struct {
	gets, sets int
	failure    error
}

func (k *policyKeychain) Get(string, string) (string, error) { k.gets++; return "", k.failure }
func (k *policyKeychain) Set(string, string, string) error   { k.sets++; return k.failure }
func (k *policyKeychain) Delete(string, string) error        { return nil }

func TestCompanyCredentialFallbackHasNoFileSideEffects(t *testing.T) {
	oldPolicy, oldRead, oldStat, oldWrite := readCompanyPolicy, managedFileRead, managedFileStat, managedFileWrite
	t.Cleanup(func() {
		readCompanyPolicy, managedFileRead, managedFileStat, managedFileWrite = oldPolicy, oldRead, oldStat, oldWrite
	})
	p := company.Defaults()
	p.AllowedProviders = []string{"copilot"}
	readCompanyPolicy = func() (company.Snapshot, error) { return company.Snapshot{Managed: true, Policy: &p}, nil }
	files := 0
	managedFileRead = func(string) ([]byte, error) { files++; return nil, errors.New("unexpected read") }
	managedFileStat = func(string) (fs.FileInfo, error) { files++; return nil, errors.New("unexpected stat") }
	managedFileWrite = func(string, []byte) error { files++; return errors.New("unexpected write") }
	for _, kind := range []securestore.Kind{securestore.Missing, securestore.Locked, securestore.Denied, securestore.Unavailable} {
		for _, keychainEnabled := range []bool{true, false} {
			keychain := &policyKeychain{failure: &securestore.Error{Kind: kind}}
			store := ManagedStore{StateDir: t.TempDir(), UseKeychain: keychainEnabled, Keychain: keychain}
			refused := func(err error) bool {
				if keychainEnabled {
					var state *securestore.Error
					return errors.As(err, &state) && state.Kind == kind
				}
				var denied *company.Error
				return errors.As(err, &denied)
			}
			if err := store.Save("copilot", "SYNTHETIC_CANARY"); !refused(err) && !(keychainEnabled && kind == securestore.Missing && errors.Is(err, ErrNotFound)) {
				t.Fatalf("save did not refuse fallback: %v", err)
			}
			if _, _, err := store.Resolve(context.Background(), "copilot"); !refused(err) && !(keychainEnabled && kind == securestore.Missing && errors.Is(err, ErrNotFound)) {
				t.Fatalf("resolve did not refuse fallback: %v", err)
			}
			if files != 0 {
				t.Fatal("prohibited credential-file I/O occurred")
			}
			if !keychainEnabled && (keychain.gets != 0 || keychain.sets != 0) {
				t.Fatal("disabled keychain used")
			}
		}
	}
}
