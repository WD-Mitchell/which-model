//go:build !nousage

package fetch

import (
	"context"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

// T8: identity redaction and Source mapping (SPEC §10–§11, D9, D10).

func TestIdentityRedaction(t *testing.T) {
	ctx := context.Background()

	// case 1: ShowIdentity false (default) → returned Account/Plan
	// cleared; the cache file keeps full identity (write happened before
	// redaction)
	opts := newOptions(t)
	opts.Enabled = map[string]bool{"fake-ok": true}
	snaps, _, err := FetchAll(ctx, []string{"fake-ok"}, opts)
	if err != nil || len(snaps) != 1 {
		t.Fatalf("redaction: got %d snapshots, err=%v", len(snaps), err)
	}
	if snaps[0].Account != "" || snaps[0].Plan != "" {
		t.Fatalf("returned identity = %q/%q, want cleared", snaps[0].Account, snaps[0].Plan)
	}
	if acct, plan := decodeCacheIdentity(t, opts.CacheDir, "fake-ok"); acct != "acct" || plan != "pro" {
		t.Fatalf("cache identity = %q/%q, want acct/pro (cache retains identity)", acct, plan)
	}

	// case 2: ShowIdentity true → identity returned
	opts = newOptions(t)
	opts.ShowIdentity = true
	opts.Enabled = map[string]bool{"fake-ok": true}
	snaps, _, err = FetchAll(ctx, []string{"fake-ok"}, opts)
	if err != nil || len(snaps) != 1 {
		t.Fatalf("show-identity: got %d snapshots, err=%v", len(snaps), err)
	}
	if snaps[0].Account != "acct" || snaps[0].Plan != "pro" {
		t.Fatalf("returned identity = %q/%q, want acct/pro", snaps[0].Account, snaps[0].Plan)
	}

	// case 3: fresh cached seed (cache holds acct/pro), ShowIdentity
	// false → returned cleared, cache intact
	dir := t.TempDir()
	seedCache(t, dir, "fake-ok", usage.Snapshot{Provider: "fake-ok", Account: "acct", Plan: "pro"})
	opts = Options{CacheDir: dir, Enabled: map[string]bool{"fake-ok": true}}
	snaps, _, err = FetchAll(ctx, []string{"fake-ok"}, opts)
	if err != nil || len(snaps) != 1 {
		t.Fatalf("cached redaction: got %d snapshots, err=%v", len(snaps), err)
	}
	if snaps[0].Account != "" || snaps[0].Plan != "" {
		t.Fatalf("cached returned identity = %q/%q, want cleared", snaps[0].Account, snaps[0].Plan)
	}
	if acct, plan := decodeCacheIdentity(t, dir, "fake-ok"); acct != "acct" || plan != "pro" {
		t.Fatalf("cached file identity = %q/%q, want intact acct/pro", acct, plan)
	}

	// case 4: offline + fresh seed, ShowIdentity false → returned cleared
	dir = t.TempDir()
	seedCache(t, dir, "fake-ok", usage.Snapshot{Provider: "fake-ok", Account: "acct", Plan: "pro"})
	opts = Options{CacheDir: dir, Offline: true, Enabled: map[string]bool{"fake-ok": true}}
	snaps, _, err = FetchAll(ctx, []string{"fake-ok"}, opts)
	if err != nil || len(snaps) != 1 {
		t.Fatalf("offline redaction: got %d snapshots, err=%v", len(snaps), err)
	}
	if snaps[0].Account != "" || snaps[0].Plan != "" {
		t.Fatalf("offline returned identity = %q/%q, want cleared", snaps[0].Account, snaps[0].Plan)
	}
}

func TestSourceFor(t *testing.T) {
	cases := []struct {
		cred usage.Credential
		kind usage.Kind
		want usage.Source
	}{
		// case 5
		{usage.Credential{Source: usage.AuthEnvVar}, usage.KindSubscription, usage.SourceAPI},
		// case 6
		{usage.Credential{Source: usage.AuthFile}, usage.KindSubscription, usage.SourceAPI},
		// case 7
		{usage.Credential{Source: usage.AuthKeychainGeneric}, usage.KindSubscription, usage.SourceAPI},
		// case 8
		{usage.Credential{Source: usage.AuthOAuthDeviceFlow}, usage.KindSubscription, usage.SourceOAuth},
		// case 9
		{usage.Credential{Source: usage.AuthCLIShellOut}, usage.KindSubscription, usage.SourceCLI},
		// case 10
		{usage.Credential{Source: usage.AuthBrowserCookie}, usage.KindSubscription, usage.SourceWeb},
		// case 11
		{usage.Credential{}, usage.KindLocalTool, usage.SourceLocal},
		// case 12
		{usage.Credential{}, usage.KindGateway, usage.SourceAPI},
	}
	for i, c := range cases {
		if got := SourceFor(c.cred, c.kind); got != c.want {
			t.Fatalf("case %d: SourceFor(source=%v, kind=%v) = %q, want %q", i+1, c.cred.Source, c.kind, got, c.want)
		}
	}
}
