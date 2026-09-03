//go:build !nousage

package fetch

import (
	"context"
	"strings"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/config"
	"github.com/WD-Mitchell/which-model/internal/usage"
	"github.com/WD-Mitchell/which-model/internal/usage/credential"
	"github.com/WD-Mitchell/which-model/internal/usage/provider/antigravity"
)

func TestFetchAllBackendOffSkipsProviders(t *testing.T) {
	old := codexbarFetch
	t.Cleanup(func() { codexbarFetch = old })
	called := false
	codexbarFetch = func(context.Context, string, usage.Source) (usage.Snapshot, error) {
		called = true
		return usage.Snapshot{}, nil
	}
	got, warnings, err := FetchAll(context.Background(), []string{"claude"}, Options{
		Backend: config.UsageBackendOff,
		Enabled: map[string]bool{"claude": true},
	})
	if err != nil || warnings != nil || got != nil || called {
		t.Fatalf("FetchAll(off) = %#v, %v, %v; called=%v", got, warnings, err, called)
	}
}

func TestFetchAllCodexBarBackendSelected(t *testing.T) {
	old := codexbarFetch
	t.Cleanup(func() { codexbarFetch = old })
	codexbarFetch = func(_ context.Context, provider string, _ usage.Source) (usage.Snapshot, error) {
		return usage.Snapshot{Provider: provider, Account: "secret", Plan: "secret-plan"}, nil
	}
	got, warnings, err := FetchAll(context.Background(), []string{"codex"}, Options{
		Backend:      config.UsageBackendCodexBar,
		Enabled:      map[string]bool{"codex": true},
		CacheDir:     t.TempDir(),
		ShowIdentity: false,
	})
	if err != nil || warnings != nil || len(got) != 1 {
		t.Fatalf("FetchAll(codexbar) = %#v, %v, %v", got, warnings, err)
	}
	if got[0].Provider != "codex" || got[0].Account != "" || got[0].Plan != "" {
		t.Fatalf("FetchAll(codexbar) snapshot = %#v", got[0])
	}
}

func TestFetchAllCodexBarInjectsManagedAntigravityOAuth(t *testing.T) {
	stateDir := t.TempDir()
	managed, err := antigravity.EncodeCredential(antigravity.Credentials{
		AccessToken:  "managed-access",
		RefreshToken: "managed-refresh",
		ExpiryDate:   1_800_000_000_000,
		ClientID:     "client-id",
		ClientSecret: "client-secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	store := credential.ManagedStore{StateDir: stateDir, UseKeychain: false}
	if err := store.Save("antigravity", managed); err != nil {
		t.Fatal(err)
	}

	oldFetch := codexbarFetch
	oldFetchEnvironment := codexbarFetchEnvironment
	t.Cleanup(func() {
		codexbarFetch = oldFetch
		codexbarFetchEnvironment = oldFetchEnvironment
	})
	codexbarFetch = func(context.Context, string, usage.Source) (usage.Snapshot, error) {
		t.Fatal("plain CodexBar fetch called despite a managed Antigravity credential")
		return usage.Snapshot{}, nil
	}
	var injected string
	codexbarFetchEnvironment = func(_ context.Context, provider string, _ usage.Source, environment map[string]string) (usage.Snapshot, error) {
		injected = environment[antigravity.CredentialsEnvironment]
		return usage.Snapshot{Provider: provider, Source: usage.SourceOAuth}, nil
	}

	got, _, err := FetchAll(context.Background(), []string{"antigravity"}, Options{
		Backend:                config.UsageBackendCodexBar,
		Enabled:                map[string]bool{"antigravity": true},
		CacheDir:               t.TempDir(),
		StateDir:               stateDir,
		DisableManagedKeychain: true,
	})
	if err != nil || len(got) != 1 || got[0].Failure != nil {
		t.Fatalf("FetchAll(antigravity) = %#v, %v", got, err)
	}
	if !strings.Contains(injected, `"access_token":"managed-access"`) ||
		!strings.Contains(injected, `"client_secret":"client-secret"`) {
		t.Fatal("CodexBar did not receive the selected managed OAuth credential JSON")
	}
}
