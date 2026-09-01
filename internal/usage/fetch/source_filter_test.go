//go:build !nousage

package fetch

import (
	"context"
	"net/http"
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
