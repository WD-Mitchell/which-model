//go:build !nousage

package credential

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/WD-Mitchell/which-model/internal/company"
	"github.com/WD-Mitchell/which-model/internal/securestore"
	"github.com/WD-Mitchell/which-model/internal/usage"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type migrationKeychain struct {
	value       string
	setErr      error
	badReadback bool
}

func (s *migrationKeychain) Get(string, string) (string, error) {
	if s.value == "" {
		return "", &securestore.Error{Kind: securestore.Missing}
	}
	if s.badReadback {
		return "wrong-token", nil
	}
	return s.value, nil
}
func (s *migrationKeychain) Set(_, _, value string) error {
	if s.setErr != nil {
		return s.setErr
	}
	s.value = value
	return nil
}
func (s *migrationKeychain) Delete(string, string) error { s.value = ""; return nil }

func migrationPolicy(t *testing.T, allowed bool) {
	t.Helper()
	old := readCompanyPolicy
	t.Cleanup(func() { readCompanyPolicy = old })
	p := company.Defaults()
	p.AllowedProviders = []string{"copilot", "codex"}
	p.AllowCredentialMigration = allowed
	readCompanyPolicy = func() (company.Snapshot, error) { return company.Snapshot{Managed: true, Policy: &p}, nil }
}

func TestNativeCredentialMigration(t *testing.T) {
	const canary = "SYNTHETIC_MIGRATION_CANARY"
	for _, name := range []string{"keep", "remove", "denied", "locked", "verify-failed", "transition-failed", "conflict", "replace", "changed-source"} {
		t.Run(name, func(t *testing.T) {
			migrationPolicy(t, name != "denied")
			keychain := &migrationKeychain{}
			store := ManagedStore{StateDir: t.TempDir(), UseKeychain: true, Keychain: keychain}
			path := store.Path("copilot")
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			original := []byte(`{"token":"` + canary + `"}`)
			if err := os.WriteFile(path, original, 0600); err != nil {
				t.Fatal(err)
			}
			opts := MigrationOptions{RemoveSource: name != "keep", Replace: name == "replace"}
			switch name {
			case "locked":
				keychain.setErr = &securestore.Error{Kind: securestore.Locked}
			case "verify-failed":
				keychain.badReadback = true
			case "transition-failed":
				opts.Transition = func() error { return errors.New("SYNTHETIC_CONFIG_ERROR") }
			case "conflict", "replace":
				keychain.value = "different-token"
			case "changed-source":
				opts.Transition = func() error { return os.WriteFile(path, []byte(`{"token":"NEWER_CANARY"}`), 0600) }
			}
			report, err := store.Migrate(context.Background(), "copilot", opts)
			success := name == "keep" || name == "remove" || name == "replace"
			if (err == nil) != success {
				t.Fatalf("success=%t, report=%+v, err=%v", success, report, err)
			}
			encoded, _ := json.Marshal(report)
			if strings.Contains(string(encoded), canary) || (err != nil && strings.Contains(err.Error(), "SYNTHETIC")) {
				t.Fatal("credential/OS text leaked")
			}
			data, readErr := os.ReadFile(path)
			if success && opts.RemoveSource {
				if !errors.Is(readErr, os.ErrNotExist) || report.LegacyCopy != "removed" {
					t.Fatal("source removal not reported")
				}
			} else {
				if readErr != nil || report.LegacyCopy != "retained" {
					t.Fatalf("source not retained: %+v %v", report, readErr)
				}
				expected := string(original)
				if name == "changed-source" {
					expected = `{"token":"NEWER_CANARY"}`
				}
				if string(data) != expected {
					t.Fatal("changed/failed source lost")
				}
			}
			if success && report.SecureStore != "verified" {
				t.Fatal("successful migration lacks verification")
			}
			if name == "denied" && keychain.value != "" {
				t.Fatal("admin denial still wrote secure credential")
			}
			if name == "conflict" && keychain.value != "different-token" {
				t.Fatal("different existing credential was replaced without --replace")
			}
		})
	}
}

func TestNativeMigrationRejectsRedirectedSource(t *testing.T) {
	migrationPolicy(t, true)
	store := ManagedStore{StateDir: t.TempDir(), Keychain: &migrationKeychain{}}
	path := store.Path("copilot")
	os.MkdirAll(filepath.Dir(path), 0700)
	target := filepath.Join(t.TempDir(), "provider-owned.json")
	os.WriteFile(target, []byte(`{"token":"SYNTHETIC_CANARY"}`), 0600)
	if err := os.Symlink(target, path); err != nil {
		t.Skip("symlinks unavailable")
	}
	if _, err := store.Migrate(context.Background(), "copilot", MigrationOptions{RemoveSource: true}); err == nil {
		t.Fatal("redirected legacy source accepted")
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatal("provider-owned target changed")
	}
}

func TestNativeMetadataSurvivesRestore(t *testing.T) {
	migrationPolicy(t, true)
	store := ManagedStore{StateDir: t.TempDir(), UseKeychain: true, Keychain: &migrationKeychain{}}
	before := usage.Credential{Token: "SYNTHETIC_ACCESS", Source: usage.AuthOAuthDeviceFlow, Extra: map[string]string{"account_id": "synthetic-account", "expires_at": "2035-01-01T00:00:00Z"}}
	if err := store.SaveCredential("codex", before); err != nil {
		t.Fatal(err)
	}
	saved, _, err := store.Resolve(context.Background(), "codex")
	if err != nil {
		t.Fatal(err)
	}
	if !sameCredential(before, saved) || saved.Extra["managed_store"] != "keychain" {
		t.Fatal("secure metadata was lost")
	}
	if err := store.Save("codex", "SYNTHETIC_REPLACEMENT"); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveCredential("codex", saved); err != nil {
		t.Fatal(err)
	}
	restored, _, err := store.Resolve(context.Background(), "codex")
	if err != nil || !sameCredential(before, restored) {
		t.Fatal("rollback lost metadata")
	}
}
