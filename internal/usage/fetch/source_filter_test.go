//go:build !nousage

package fetch

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/WD-Mitchell/which-model/internal/usage"
	"github.com/WD-Mitchell/which-model/internal/usage/cache"
	"github.com/WD-Mitchell/which-model/internal/usage/credential"
)

// Issue #28 review P1: a forced canonical source must constrain credential
// resolution. A provider whose chain mixes an env-var link (api) and a CLI
// link must resolve ONLY via the matching links when --source is forced.
func TestForcedSourceFiltersCredentialChain(t *testing.T) {
	chain := []usage.AuthSource{
		{Kind: usage.AuthEnvVar, EnvVar: "WHICH_MODEL_TEST_FORCED_SOURCE_API_TOKEN"},
		{Kind: usage.AuthCLIShellOut, Shell: &usage.ShellSpec{Command: "echo", Args: []string{"cli-token"}, Timeout: 1_000_000_000}},
	}

	// Auto (empty source): first link (env) wins.
	t.Setenv("WHICH_MODEL_TEST_FORCED_SOURCE_API_TOKEN", "api-token-value")
	cred, _, err := credential.ResolveChain(context.Background(), filterChainForSource(chain, ""), http.DefaultClient)
	if err != nil || cred.Token != "api-token-value" {
		t.Fatalf("auto chain resolved %q, err = %v; want the api (env) token", cred.Token, err)
	}

	// Forced cli: the env link is filtered out, so the CLI link resolves.
	cred, _, err = credential.ResolveChain(context.Background(), filterChainForSource(chain, usage.SourceCLI), http.DefaultClient)
	if err != nil || cred.Token != "cli-token" {
		t.Fatalf("forced cli resolved %q, err = %v; want the cli token", cred.Token, err)
	}

	// Forced api: only the env link remains — same as auto here.
	cred, _, err = credential.ResolveChain(context.Background(), filterChainForSource(chain, usage.SourceAPI), http.DefaultClient)
	if err != nil || cred.Token != "api-token-value" {
		t.Fatalf("forced api resolved %q, err = %v; want the api token", cred.Token, err)
	}
}

// Issue #28 review P1: `--source cache` pins the run to the cache — no
// credential resolution and no live fetch, even with a valid credential.
// A cache miss surfaces as the D-7 fallback_unavailable failure.
func TestCacheSourceSkipsCredentialAndFetch(t *testing.T) {
	dir := t.TempDir()
	store := &cache.Store{Dir: dir}
	snap, _ := runProvider(context.Background(), store, http.DefaultClient, "claude", usage.Descriptor{
		ID:       "claude",
		Kind:     usage.KindSubscription,
		Auth:     []usage.AuthSource{{Kind: usage.AuthEnvVar, EnvVar: "WHICH_MODEL_TEST_FORCED_SOURCE_API_TOKEN"}},
		CacheTTL: 60 * time.Second,
	}, Options{Source: usage.SourceCache, CacheDir: dir})
	if snap.Failure == nil || snap.Failure.Code != "fallback_unavailable" {
		t.Fatalf("snapshot failure = %#v, want fallback_unavailable", snap.Failure)
	}
}

func TestNativeDescriptorExpandsFileCredential(t *testing.T) {
	for _, overridePresent := range []bool{true, false} {
		t.Run(fmtBool(overridePresent), func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("USERPROFILE", home)
			if err := os.WriteFile(filepath.Join(home, "auth.json"), []byte(`{"token":"synthetic-descriptor-token"}`), 0o600); err != nil {
				t.Fatal(err)
			}
			t.Setenv("WHICH_MODEL_TEST_DESCRIPTOR_DIR", "")
			if overridePresent {
				t.Setenv("WHICH_MODEL_TEST_DESCRIPTOR_DIR", home)
			}
			calls := 0
			desc := usage.Descriptor{Kind: usage.KindSubscription, Auth: []usage.AuthSource{{Kind: usage.AuthFile, FilePaths: []string{"$WHICH_MODEL_TEST_DESCRIPTOR_DIR/auth.json", "~/auth.json"}, JSONPath: "token"}}, Fetch: func(_ context.Context, cred usage.Credential, _ *http.Client) (usage.Snapshot, error) {
				calls++
				if cred.Token != "synthetic-descriptor-token" {
					t.Error("wrong file credential")
				}
				return usage.Snapshot{Provider: "file-test"}, nil
			}}
			snap, _ := runProvider(context.Background(), &cache.Store{Dir: t.TempDir()}, http.DefaultClient, "file-test", desc, Options{StateDir: t.TempDir(), DisableManagedKeychain: true})
			if snap.Failure != nil || calls != 1 || snap.Source != usage.SourceAPI {
				t.Fatalf("expanded descriptor did not reach Fetch: calls=%d failure=%v", calls, snap.Failure)
			}
		})
	}
}

func fmtBool(value bool) string {
	if value {
		return "override present"
	}
	return "home fallback"
}

func TestForcedSourceRejectsMismatchedManagedCredential(t *testing.T) {
	const token = "synthetic-managed-source-canary"
	for _, tc := range []struct {
		name       string
		forced     usage.Source
		apiKey     bool
		wantCall   bool
		wantSource usage.Source
	}{
		{"api rejects oauth", usage.SourceAPI, false, false, ""},
		{"oauth rejects api", usage.SourceOAuth, true, false, ""},
		{"matching oauth", usage.SourceOAuth, false, true, usage.SourceOAuth},
		{"matching api", usage.SourceAPI, true, true, usage.SourceAPI},
		{"auto oauth", "", false, true, usage.SourceOAuth},
		{"auto api", "", true, true, usage.SourceAPI},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := credential.ManagedStore{StateDir: t.TempDir(), UseKeychain: false}
			var err error
			if tc.apiKey {
				err = store.SaveAPIKey("managed-test", token)
			} else {
				err = store.Save("managed-test", token)
			}
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			desc := usage.Descriptor{Kind: usage.KindSubscription, Auth: []usage.AuthSource{{Kind: usage.AuthFile, FilePaths: []string{filepath.Join(t.TempDir(), "missing.json")}, JSONPath: "token"}, {Kind: usage.AuthOAuthDeviceFlow}}, Fetch: func(context.Context, usage.Credential, *http.Client) (usage.Snapshot, error) {
				calls++
				return usage.Snapshot{Provider: "managed-test"}, nil
			}}
			snap, _ := runProvider(context.Background(), &cache.Store{Dir: t.TempDir()}, http.DefaultClient, "managed-test", desc, Options{Source: tc.forced, StateDir: store.StateDir, DisableManagedKeychain: true})
			if tc.wantCall {
				if calls != 1 || snap.Failure != nil || snap.Source != tc.wantSource {
					t.Fatalf("matching/auto failed: calls=%d failure=%v source=%s", calls, snap.Failure, snap.Source)
				}
			} else {
				if calls != 0 || snap.Failure == nil || snap.Failure.Code != "login_required" {
					t.Fatalf("source mismatch reached fetch: calls=%d failure=%v", calls, snap.Failure)
				}
				if snap.Failure != nil && strings.Contains(snap.Failure.Message, token) {
					t.Error("failure leaks credential")
				}
			}
		})
	}
}

func TestNativeForcedSourceChecksCachedProvenance(t *testing.T) {
	for _, tc := range []struct {
		name           string
		stored, forced usage.Source
		wantFetch      bool
	}{
		{"matching", usage.SourceAPI, usage.SourceAPI, false},
		{"mismatch", usage.SourceOAuth, usage.SourceAPI, true},
		{"unknown", "", usage.SourceAPI, true},
		{"auto accepts unknown", "", "", false},
		{"cache source accepts oauth", usage.SourceOAuth, usage.SourceCache, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &cache.Store{Dir: t.TempDir()}
			if err := store.Write("cache-source-test", usage.Snapshot{Provider: "cache-source-test", Source: tc.stored}); err != nil {
				t.Fatal(err)
			}
			calls := 0
			desc := usage.Descriptor{CacheTTL: time.Hour, Fetch: func(context.Context, usage.Credential, *http.Client) (usage.Snapshot, error) {
				calls++
				return usage.Snapshot{Provider: "cache-source-test"}, nil
			}}
			snap, _ := runProvider(context.Background(), store, http.DefaultClient, "cache-source-test", desc, Options{Source: tc.forced})
			if (calls == 1) != tc.wantFetch || snap.Failure != nil {
				t.Fatalf("source predicate calls=%d failure=%v", calls, snap.Failure)
			}
			if !tc.wantFetch && snap.Source != usage.SourceCache {
				t.Errorf("expected cache provenance, got %s", snap.Source)
			}
		})
	}
}
