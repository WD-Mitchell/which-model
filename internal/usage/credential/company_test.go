//go:build !nousage

package credential

import (
	"context"
	"errors"
	"io/fs"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/company"
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
	for _, keychainEnabled := range []bool{true, false} {
		keychain := &policyKeychain{failure: errors.New("unavailable")}
		store := ManagedStore{StateDir: t.TempDir(), UseKeychain: keychainEnabled, Keychain: keychain}
		var refused *company.Error
		if err := store.Save("copilot", "SYNTHETIC_CANARY"); !errors.As(err, &refused) {
			t.Fatalf("save did not refuse fallback: %v", err)
		}
		if _, _, err := store.Resolve(context.Background(), "copilot"); !errors.As(err, &refused) {
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
