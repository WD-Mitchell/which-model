//go:build !nousage

package credential

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

type fakeManagedKeychain struct {
	value       string
	getErr      error
	setErr      error
	deleteErr   error
	getCalls    int
	setCalls    int
	deleteCalls int
}

func (f *fakeManagedKeychain) Get(service, account string) (string, error) {
	f.getCalls++
	return f.value, f.getErr
}

func (f *fakeManagedKeychain) Set(service, account, value string) error {
	f.setCalls++
	if f.setErr == nil {
		f.value = value
	}
	return f.setErr
}

func (f *fakeManagedKeychain) Delete(service, account string) error {
	f.deleteCalls++
	if f.deleteErr != nil {
		return f.deleteErr
	}
	if f.value == "" {
		return keyringNotFound
	}
	f.value = ""
	return nil
}

func TestManagedStoreUsesKeychainByDefault(t *testing.T) {
	keychain := &fakeManagedKeychain{}
	store := ManagedStore{StateDir: t.TempDir(), Keychain: keychain, UseKeychain: true}
	if err := store.Save("copilot", "token-value"); err != nil {
		t.Fatal(err)
	}
	if keychain.setCalls != 1 || keychain.value != "token-value" {
		t.Fatalf("keychain = %#v", keychain)
	}
	if _, err := os.Stat(store.Path("copilot")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("fallback file exists after keychain save: %v", err)
	}
	cred, warnings, err := store.Resolve(context.Background(), "copilot")
	if err != nil {
		t.Fatal(err)
	}
	if cred.Token != "token-value" || cred.Source != usage.AuthOAuthDeviceFlow || len(warnings) != 0 {
		t.Fatalf("Resolve = %#v, warnings = %v", cred, warnings)
	}
}

func TestManagedStoreFallsBackToFile(t *testing.T) {
	keychain := &fakeManagedKeychain{setErr: errors.New("keychain unavailable"), getErr: errors.New("keychain unavailable")}
	store := ManagedStore{StateDir: t.TempDir(), Keychain: keychain, UseKeychain: true}
	if err := store.Save("copilot", "token-value"); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(store.Path("copilot"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("file mode = %o, want 600", info.Mode().Perm())
	}
	cred, _, err := store.Resolve(context.Background(), "copilot")
	if err != nil {
		t.Fatal(err)
	}
	if cred.Token != "token-value" || cred.Source != usage.AuthOAuthDeviceFlow {
		t.Fatalf("Resolve = %#v", cred)
	}
}

func TestManagedStoreSkipsKeychainWhenDisabled(t *testing.T) {
	keychain := &fakeManagedKeychain{}
	store := ManagedStore{StateDir: t.TempDir(), Keychain: keychain, UseKeychain: false}
	if err := store.Save("copilot", "token-value"); err != nil {
		t.Fatal(err)
	}
	if keychain.setCalls != 0 || keychain.getCalls != 0 {
		t.Fatalf("keychain called while disabled: %#v", keychain)
	}
	cred, _, err := store.Resolve(context.Background(), "copilot")
	if err != nil || cred.Token != "token-value" {
		t.Fatalf("Resolve = %#v, err = %v", cred, err)
	}
	if keychain.getCalls != 0 {
		t.Fatalf("keychain get calls = %d, want 0", keychain.getCalls)
	}
}

func TestManagedStoreRemoveDeletesBothLocations(t *testing.T) {
	keychain := &fakeManagedKeychain{value: "keychain-token"}
	store := ManagedStore{StateDir: t.TempDir(), Keychain: keychain, UseKeychain: false}
	if err := store.Save("copilot", "file-token"); err != nil {
		t.Fatal(err)
	}
	if err := store.Remove("copilot"); err != nil {
		t.Fatal(err)
	}
	if keychain.deleteCalls != 1 || keychain.value != "" {
		t.Fatalf("keychain = %#v", keychain)
	}
	if _, err := os.Stat(store.Path("copilot")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("fallback file remains: %v", err)
	}
	if err := store.Remove("copilot"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second Remove error = %v, want ErrNotFound", err)
	}
}

func TestResolveProviderUsesManagedCredentialAfterDeclaredSources(t *testing.T) {
	store := ManagedStore{StateDir: t.TempDir(), Keychain: &fakeManagedKeychain{}, UseKeychain: false}
	if err := store.Save("copilot", "managed-token"); err != nil {
		t.Fatal(err)
	}
	validated := false
	sources := []usage.AuthSource{
		{Kind: usage.AuthEnvVar, EnvVar: "WM_MANAGED_TEST_TOKEN"},
		{
			Kind: usage.AuthOAuthDeviceFlow,
			Validate: func(_ context.Context, credential usage.Credential, _ *http.Client) error {
				validated = credential.Token == "managed-token"
				return nil
			},
		},
	}
	credential, warnings, err := ResolveProvider(context.Background(), "copilot", sources, &http.Client{}, store)
	if err != nil {
		t.Fatal(err)
	}
	if credential.Token != "managed-token" || credential.Source != usage.AuthOAuthDeviceFlow || !validated || len(warnings) != 0 {
		t.Fatalf("ResolveProvider = %#v, warnings = %v, validated = %v", credential, warnings, validated)
	}
}

func TestResolveProviderPreservesDeclaredSourcePrecedence(t *testing.T) {
	t.Setenv("WM_MANAGED_TEST_TOKEN", "environment-token")
	keychain := &fakeManagedKeychain{value: "managed-token"}
	store := ManagedStore{StateDir: t.TempDir(), Keychain: keychain, UseKeychain: true}
	sources := []usage.AuthSource{
		{Kind: usage.AuthEnvVar, EnvVar: "WM_MANAGED_TEST_TOKEN"},
		{Kind: usage.AuthOAuthDeviceFlow},
	}
	credential, _, err := ResolveProvider(context.Background(), "copilot", sources, &http.Client{}, store)
	if err != nil {
		t.Fatal(err)
	}
	if credential.Token != "environment-token" || credential.Source != usage.AuthEnvVar {
		t.Fatalf("ResolveProvider = %#v", credential)
	}
	if keychain.getCalls != 0 {
		t.Fatalf("managed keychain read before higher-precedence source: %d calls", keychain.getCalls)
	}
}

func TestManagedStoreErrorsRedactKeychainDetails(t *testing.T) {
	const canary = "canary-managed-credential-token"
	store := ManagedStore{
		StateDir: t.TempDir(),
		Keychain: &fakeManagedKeychain{deleteErr: errors.New("delete failed for " + canary)},
	}
	err := store.Remove("copilot")
	if err == nil || strings.Contains(err.Error(), canary) {
		t.Fatalf("Remove error = %v", err)
	}
}
