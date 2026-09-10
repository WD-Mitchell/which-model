package service

import (
	"errors"
	"github.com/WD-Mitchell/which-model/internal/company"
	"github.com/WD-Mitchell/which-model/internal/securestore"
	"os"
	"strings"
	"testing"
)

type catalogTestStore struct {
	value   string
	failure error
	calls   int
}

func (s *catalogTestStore) Get(string, string) (string, error) { s.calls++; return s.value, s.failure }
func (s *catalogTestStore) Set(_, _, v string) error {
	s.calls++
	if s.failure == nil {
		s.value = v
	}
	return s.failure
}
func (s *catalogTestStore) Delete(string, string) error {
	s.calls++
	if s.failure == nil {
		s.value = ""
	}
	return s.failure
}

func TestCompanyCatalogCredentialHasNoPlaintextFallback(t *testing.T) {
	oldPolicy, oldStore := readCompanyPolicy, catalogSecureStore
	t.Cleanup(func() { readCompanyPolicy, catalogSecureStore = oldPolicy, oldStore })
	p := company.Defaults()
	p.AllowedProviders = []string{securestore.CatalogAccount}
	readCompanyPolicy = func() (company.Snapshot, error) { return company.Snapshot{Managed: true, Policy: &p}, nil }
	store := &catalogTestStore{}
	catalogSecureStore = func() securestore.Store { return store }
	dir := t.TempDir()
	path := aaKeyPath(dir)
	const canary = "LEGACY_CATALOG_CANARY"
	if err := os.WriteFile(path, []byte(canary), 0600); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []securestore.Kind{securestore.Missing, securestore.Locked, securestore.Denied, securestore.Unavailable} {
		store.failure = &securestore.Error{Kind: kind}
		key, err := loadAAKey(dir)
		if key != "" || (kind != securestore.Missing && !errors.Is(err, store.failure)) {
			t.Fatal("native failure used legacy key")
		}
		if err := writeAAKeyFile(dir, "NEW_SYNTHETIC_KEY"); !errors.Is(err, store.failure) {
			t.Fatal("failed native save changed storage")
		}
		if err != nil && strings.Contains(err.Error(), canary) {
			t.Fatal("legacy key leaked")
		}
		data, _ := os.ReadFile(path)
		if string(data) != canary {
			t.Fatal("legacy key was modified")
		}
	}
	store.failure = nil
	if err := writeAAKeyFile(dir, "NEW_SYNTHETIC_KEY"); err != nil {
		t.Fatal(err)
	}
	if err := clearAAKeyFile(dir); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != canary {
		t.Fatal("normal company logout removed uninspected legacy key")
	}
	calls := store.calls
	p.AllowedProviders = nil
	if _, err := loadAAKey(dir); err == nil || store.calls != calls {
		t.Fatal("unapproved provider accessed store")
	}
}
