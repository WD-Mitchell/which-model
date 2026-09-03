//go:build !nousage

package fetch

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/WD-Mitchell/which-model/internal/config"
	"github.com/WD-Mitchell/which-model/internal/usage"
	"github.com/WD-Mitchell/which-model/internal/usage/cache"
	"github.com/WD-Mitchell/which-model/internal/usage/credential"
	"github.com/WD-Mitchell/which-model/internal/usage/provider/antigravity"
)

// B06's Providers list reads usage from the cache only (OfflineRead, never a
// live fetch), so the codexbar backend must write successful snapshots through
// to the same store the native path uses — otherwise every cache-only consumer
// shows "no usage data" under backend = codexbar.

func stubCodexBar(t *testing.T, fn func(ctx context.Context, provider string, source usage.Source) (usage.Snapshot, error)) {
	t.Helper()
	old := codexbarFetch
	codexbarFetch = fn
	t.Cleanup(func() { codexbarFetch = old })
}

func TestFetchAllCodexBarWritesCache(t *testing.T) {
	stubCodexBar(t, func(_ context.Context, provider string, _ usage.Source) (usage.Snapshot, error) {
		used := 42.0
		minutes := 10080
		return usage.Snapshot{
			Provider: provider,
			Account:  "acct@example.com",
			Plan:     "plus",
			Windows: []usage.Window{{
				ID: "weekly", Label: "seven day", Unit: usage.UnitPercent,
				UsedPercent: &used, WindowMinutes: &minutes, UsageKnown: true,
			}},
		}, nil
	})
	dir := t.TempDir()
	got, warns, err := FetchAll(context.Background(), []string{"codex"}, Options{
		Backend:      config.UsageBackendCodexBar,
		Enabled:      map[string]bool{"codex": true},
		CacheDir:     dir,
		ShowIdentity: false,
	})
	if err != nil || warns != nil || len(got) != 1 {
		t.Fatalf("FetchAll(codexbar) = %#v, %v, %v", got, warns, err)
	}
	// The returned snapshot is redacted…
	if got[0].Account != "" || got[0].Plan != "" {
		t.Fatalf("returned snapshot identity = %q/%q, want redacted", got[0].Account, got[0].Plan)
	}
	// …but the cache keeps full identity (native parity: cache writes
	// precede redaction so identity-showing reads still work).
	store := &cache.Store{Dir: dir}
	cached, stale, rerr := store.Read("codex", time.Hour)
	if rerr != nil || stale {
		t.Fatalf("cache read: %v stale=%v", rerr, stale)
	}
	if cached.Account != "acct@example.com" || cached.Plan != "plus" {
		t.Fatalf("cached identity = %q/%q, want retained", cached.Account, cached.Plan)
	}
	if len(cached.Windows) != 1 || cached.Windows[0].ID != "weekly" ||
		cached.Windows[0].UsedPercent == nil || *cached.Windows[0].UsedPercent != 42 {
		t.Fatalf("cached windows = %#v", cached.Windows)
	}
}

func TestFetchAllCodexBarOfflineNeverLoadsCredentialsOrRunsProvider(t *testing.T) {
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
	credentialStore := credential.ManagedStore{StateDir: stateDir, UseKeychain: false}
	if err := credentialStore.Save("antigravity", managed); err != nil {
		t.Fatal(err)
	}

	cacheDir := t.TempDir()
	cacheStore := cache.Store{Dir: cacheDir}
	if err := cacheStore.Write("antigravity", usage.Snapshot{
		Provider: "antigravity",
		Account:  "private@example.com",
		Plan:     "pro",
	}); err != nil {
		t.Fatal(err)
	}

	oldFetch := codexbarFetch
	oldFetchEnvironment := codexbarFetchEnvironment
	t.Cleanup(func() {
		codexbarFetch = oldFetch
		codexbarFetchEnvironment = oldFetchEnvironment
	})
	codexbarFetch = func(context.Context, string, usage.Source) (usage.Snapshot, error) {
		t.Fatal("offline CodexBar fetch ran the provider subprocess")
		return usage.Snapshot{}, nil
	}
	codexbarFetchEnvironment = func(context.Context, string, usage.Source, map[string]string) (usage.Snapshot, error) {
		t.Fatal("offline CodexBar fetch loaded and injected a managed credential")
		return usage.Snapshot{}, nil
	}

	cases := []struct {
		name string
		opts Options
	}{
		{name: "offline flag", opts: Options{Offline: true}},
		{name: "cache source", opts: Options{Source: usage.SourceCache}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.opts.Backend = config.UsageBackendCodexBar
			tc.opts.Enabled = map[string]bool{"antigravity": true}
			tc.opts.CacheDir = cacheDir
			tc.opts.StateDir = stateDir
			tc.opts.DisableManagedKeychain = true
			tc.opts.MaxAge = time.Hour
			got, warns, err := FetchAll(context.Background(), []string{"antigravity"}, tc.opts)
			if err != nil || len(warns) != 0 || len(got) != 1 {
				t.Fatalf("FetchAll(codexbar offline) = %#v, %v, %v", got, warns, err)
			}
			if got[0].Provider != "antigravity" || got[0].Source != usage.SourceCache ||
				got[0].Confidence != "cached" || got[0].Account != "" || got[0].Plan != "" {
				t.Fatalf("offline snapshot = %#v", got[0])
			}
		})
	}
}

func TestFetchAllCodexBarDoesNotCacheFailures(t *testing.T) {
	stubCodexBar(t, func(_ context.Context, provider string, _ usage.Source) (usage.Snapshot, error) {
		if provider == "claude" {
			return usage.Snapshot{
				Provider: provider,
				Failure:  &usage.Failure{Code: "provider_status", Message: "codexbar returned no matching provider"},
			}, nil
		}
		// Hard error (e.g. binary discovery failure) → the error path
		// synthesizes a failure snapshot.
		return usage.Snapshot{}, errors.New("codexbar CLI not found")
	})
	dir := t.TempDir()
	got, warns, err := FetchAll(context.Background(), []string{"claude", "codex"}, Options{
		Backend:  config.UsageBackendCodexBar,
		Enabled:  map[string]bool{"claude": true, "codex": true},
		CacheDir: dir,
	})
	if err != nil || warns != nil || len(got) != 2 {
		t.Fatalf("FetchAll(codexbar) = %#v, %v, %v", got, warns, err)
	}
	for _, id := range []string{"claude", "codex"} {
		if _, serr := os.Stat(filepath.Join(dir, id+".json")); serr == nil {
			t.Fatalf("%s: failure snapshot was cached", id)
		}
	}
}

func TestFetchAllCodexBarCacheWriteFailureWarns(t *testing.T) {
	stubCodexBar(t, func(_ context.Context, provider string, _ usage.Source) (usage.Snapshot, error) {
		return usage.Snapshot{Provider: provider}, nil
	})
	// A regular FILE at the cache-dir path makes MkdirAll fail: the write is
	// a warning, never a group error (annex-a §6 — the cache is an
	// optimization).
	blocker := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, warns, err := FetchAll(context.Background(), []string{"codex"}, Options{
		Backend:  config.UsageBackendCodexBar,
		Enabled:  map[string]bool{"codex": true},
		CacheDir: blocker,
	})
	if err != nil || len(got) != 1 {
		t.Fatalf("FetchAll(codexbar) = %#v, %v", got, err)
	}
	if len(warns) != 1 || !strings.Contains(warns[0].Message, "failed to cache usage for provider codex") {
		t.Fatalf("warnings = %v, want one containing %q", warns, "failed to cache usage for provider codex")
	}
}
