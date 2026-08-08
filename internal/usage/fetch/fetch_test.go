//go:build !nousage

package fetch

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/WD-Mitchell/which-model/internal/usage"
	"github.com/WD-Mitchell/which-model/internal/usage/cache"
	"github.com/WD-Mitchell/which-model/internal/usage/credential"
)

// T1: skeleton — Options and the enable gate (SPEC §1–§2, D1).

func TestFetchAllEmpty(t *testing.T) {
	ctx := context.Background()

	// case 1: nil providers → (nil, nil, nil)
	snaps, warns, err := FetchAll(ctx, nil, Options{})
	if snaps != nil || warns != nil || err != nil {
		t.Fatalf("nil providers: got (%v, %v, %v), want (nil, nil, nil)", snaps, warns, err)
	}

	// case 2: empty slice → (nil, nil, nil)
	snaps, warns, err = FetchAll(ctx, []string{}, Options{})
	if snaps != nil || warns != nil || err != nil {
		t.Fatalf("empty providers: got (%v, %v, %v), want (nil, nil, nil)", snaps, warns, err)
	}
}

func TestGateDefaultDeny(t *testing.T) {
	ctx := context.Background()

	// case 3: Enabled nil → default-deny (SPEC D1)
	snaps, _, err := FetchAll(ctx, []string{"fake-ok"}, Options{CacheDir: t.TempDir()})
	if err != nil || len(snaps) != 0 {
		t.Fatalf("default-deny: got %d snapshots, err=%v; want 0, nil", len(snaps), err)
	}

	// case 4: empty Enabled map → default-deny
	snaps, _, err = FetchAll(ctx, []string{"fake-ok"}, Options{CacheDir: t.TempDir(), Enabled: map[string]bool{}})
	if err != nil || len(snaps) != 0 {
		t.Fatalf("empty Enabled map: got %d snapshots, err=%v; want 0, nil", len(snaps), err)
	}

	// case 5: gate applied per provider
	snaps, _, err = FetchAll(ctx, []string{"fake-ok", "fake-fail"}, Options{
		CacheDir: t.TempDir(),
		Enabled:  map[string]bool{"fake-fail": true},
	})
	if err != nil || len(snaps) != 1 {
		t.Fatalf("per-provider gate: got %d snapshots, err=%v; want 1, nil", len(snaps), err)
	}
	if snaps[0].Provider != "fake-fail" {
		t.Fatalf("gate result provider = %q, want %q", snaps[0].Provider, "fake-fail")
	}
}

// T2: test fakes, unknown providers, happy path (SPEC §3–§4, D2, D3).

func TestUnknownProvider(t *testing.T) {
	ctx := context.Background()

	// case 1: unknown provider → provider_status failure whose message
	// contains the ID
	opts := newOptions(t)
	opts.Enabled = map[string]bool{"does-not-exist": true}
	snaps, _, err := FetchAll(ctx, []string{"does-not-exist"}, opts)
	if err != nil || len(snaps) != 1 {
		t.Fatalf("unknown provider: got %d snapshots, err=%v; want 1, nil", len(snaps), err)
	}
	if snaps[0].Failure == nil || snaps[0].Failure.Code != "provider_status" {
		t.Fatalf("unknown provider failure = %+v, want code provider_status", snaps[0].Failure)
	}
	if !strings.Contains(snaps[0].Failure.Message, "does-not-exist") {
		t.Fatalf("unknown provider message %q does not contain the provider ID", snaps[0].Failure.Message)
	}

	// case 2: unknown + fake-ok → 2 snapshots; fake-ok has no failure
	opts = newOptions(t)
	opts.Enabled = map[string]bool{"does-not-exist": true, "fake-ok": true}
	snaps, _, err = FetchAll(ctx, []string{"does-not-exist", "fake-ok"}, opts)
	if err != nil || len(snaps) != 2 {
		t.Fatalf("mixed: got %d snapshots, err=%v; want 2, nil", len(snaps), err)
	}
	for _, s := range snaps {
		if s.Provider == "fake-ok" && s.Failure != nil {
			t.Fatalf("fake-ok snapshot carries failure %+v", s.Failure)
		}
	}
}

func TestHappyPath(t *testing.T) {
	ctx := context.Background()

	// case 3: fake-ok (no Auth) → live snapshot, SourceAPI (empty chain →
	// zero Credential → fallback), Confidence "", Stale false, err nil
	opts := newOptions(t)
	opts.Enabled = map[string]bool{"fake-ok": true}
	snaps, _, err := FetchAll(ctx, []string{"fake-ok"}, opts)
	if err != nil || len(snaps) != 1 {
		t.Fatalf("fake-ok: got %d snapshots, err=%v", len(snaps), err)
	}
	s := snaps[0]
	if s.Provider != "fake-ok" || len(s.Windows) == 0 {
		t.Fatalf("fake-ok snapshot = %+v, want provider fake-ok with windows populated", s)
	}
	if s.Source != usage.SourceAPI || s.Confidence != "" || s.Stale {
		t.Fatalf("fake-ok provenance = source %q confidence %q stale %v; want api/\"\"/false", s.Source, s.Confidence, s.Stale)
	}

	// case 4: fake-env with token set → SourceAPI, Confidence ""
	t.Setenv("WHICH_MODEL_TEST_TOKEN", "tok-1")
	opts = newOptions(t)
	opts.Enabled = map[string]bool{"fake-env": true}
	snaps, _, err = FetchAll(ctx, []string{"fake-env"}, opts)
	if err != nil || len(snaps) != 1 {
		t.Fatalf("fake-env: got %d snapshots, err=%v", len(snaps), err)
	}
	if snaps[0].Source != usage.SourceAPI || snaps[0].Confidence != "" {
		t.Fatalf("fake-env provenance = %q/%q; want api/\"\"", snaps[0].Source, snaps[0].Confidence)
	}
}

func TestLoginRequired(t *testing.T) {
	// case 5: fake-env with token env unset (empty after trim) →
	// login_required, no fetch
	resetCalls()
	t.Setenv("WHICH_MODEL_TEST_TOKEN", "")
	opts := newOptions(t)
	opts.Enabled = map[string]bool{"fake-env": true}
	snaps, _, err := FetchAll(context.Background(), []string{"fake-env"}, opts)
	if err != nil || len(snaps) != 1 {
		t.Fatalf("fake-env without token: got %d snapshots, err=%v", len(snaps), err)
	}
	if snaps[0].Failure == nil || snaps[0].Failure.Code != "login_required" {
		t.Fatalf("fake-env without token failure = %+v, want login_required", snaps[0].Failure)
	}
	if callCount("fake-env") != 0 {
		t.Fatalf("fake-env fetched %d times; want 0 (login_required short-circuits)", callCount("fake-env"))
	}
}

func TestPartialFailureIsData(t *testing.T) {
	// case 6: fake-fail → provider_status "boom", err == nil
	opts := newOptions(t)
	opts.Enabled = map[string]bool{"fake-fail": true}
	snaps, _, err := FetchAll(context.Background(), []string{"fake-fail"}, opts)
	if err != nil || len(snaps) != 1 {
		t.Fatalf("fake-fail: got %d snapshots, err=%v; want 1, nil", len(snaps), err)
	}
	if snaps[0].Failure == nil || snaps[0].Failure.Code != "provider_status" || snaps[0].Failure.Message != "boom" {
		t.Fatalf("fake-fail failure = %+v, want {provider_status boom}", snaps[0].Failure)
	}
}

func TestMapError(t *testing.T) {
	// case 7: context.DeadlineExceeded → timeout
	if f := MapError(context.DeadlineExceeded); f.Code != "timeout" {
		t.Fatalf("MapError(DeadlineExceeded).Code = %q, want timeout", f.Code)
	}
	// case 8: credential.ErrNotFound → login_required
	if f := MapError(credential.ErrNotFound); f.Code != "login_required" {
		t.Fatalf("MapError(ErrNotFound).Code = %q, want login_required", f.Code)
	}
}

// T3: cache-first reads and writes (SPEC §4, §6, D3, D11).

// seedCache writes a snapshot straight through F13's Store (seeding does
// NOT go through FetchAll).
func seedCache(t *testing.T, dir, id string, snap usage.Snapshot) {
	t.Helper()
	if err := (&cache.Store{Dir: dir}).Write(id, snap); err != nil {
		t.Fatalf("seed %s: %v", id, err)
	}
}

func TestCacheReadsAndWrites(t *testing.T) {
	ctx := context.Background()

	// case 1: fresh seeded entry served from cache; fake-never's Fetch
	// panics, so no panic proves no fetch happened
	resetCalls()
	dir := t.TempDir()
	seedCache(t, dir, "fake-never", usage.Snapshot{Provider: "fake-never", Account: "acct", Plan: "pro", Windows: []usage.Window{{ID: "w1"}}})
	opts := Options{CacheDir: dir, Enabled: map[string]bool{"fake-never": true}}
	snaps, _, err := FetchAll(ctx, []string{"fake-never"}, opts)
	if err != nil || len(snaps) != 1 {
		t.Fatalf("cached fake-never: got %d snapshots, err=%v; want 1, nil", len(snaps), err)
	}
	s := snaps[0]
	if s.Source != usage.SourceCache || s.Confidence != "cached" || s.Stale {
		t.Fatalf("cached provenance = source %q confidence %q stale %v; want cache/cached/false", s.Source, s.Confidence, s.Stale)
	}
	if callCount("fake-never") != 0 {
		t.Fatalf("fake-never fetched %d times; want 0 (cache hit must not fetch)", callCount("fake-never"))
	}

	// case 2: MaxAge 1ns makes a fresh seed effectively stale → refetch,
	// cache file rewritten with the live snapshot
	dir = t.TempDir()
	seedCache(t, dir, "fake-ok", usage.Snapshot{Provider: "fake-ok"})
	resetCalls()
	opts = Options{CacheDir: dir, MaxAge: time.Nanosecond, Enabled: map[string]bool{"fake-ok": true}}
	snaps, _, err = FetchAll(ctx, []string{"fake-ok"}, opts)
	if err != nil || len(snaps) != 1 {
		t.Fatalf("stale refetch: got %d snapshots, err=%v", len(snaps), err)
	}
	if callCount("fake-ok") != 1 {
		t.Fatalf("fake-ok fetched %d times; want 1 (stale entry must refetch)", callCount("fake-ok"))
	}
	if snaps[0].Source != usage.SourceAPI {
		t.Fatalf("live source = %q, want api", snaps[0].Source)
	}
	acct, plan := decodeCacheIdentity(t, dir, "fake-ok")
	if acct != "acct" || plan != "pro" {
		t.Fatalf("cache file not rewritten with live snapshot: account/plan = %q/%q, want acct/pro", acct, plan)
	}

	// case 3: empty cache → fetch happened, cache file created
	dir = t.TempDir()
	resetCalls()
	opts = Options{CacheDir: dir, Enabled: map[string]bool{"fake-ok": true}}
	if _, _, err = FetchAll(ctx, []string{"fake-ok"}, opts); err != nil {
		t.Fatalf("empty cache: err=%v", err)
	}
	if callCount("fake-ok") != 1 {
		t.Fatalf("fake-ok fetched %d times; want 1", callCount("fake-ok"))
	}
	if _, serr := os.Stat(filepath.Join(dir, "fake-ok.json")); serr != nil {
		t.Fatalf("fake-ok.json not created after fetch: %v", serr)
	}

	// case 4: second call served from cache (total fetch count stays 1)
	snaps, _, err = FetchAll(ctx, []string{"fake-ok"}, opts)
	if err != nil || len(snaps) != 1 {
		t.Fatalf("cached second call: got %d snapshots, err=%v", len(snaps), err)
	}
	if callCount("fake-ok") != 1 {
		t.Fatalf("fake-ok fetched %d times total; want 1 (second call from cache)", callCount("fake-ok"))
	}
	if snaps[0].Source != usage.SourceCache || snaps[0].Confidence != "cached" {
		t.Fatalf("second call provenance = %q/%q; want cache/cached", snaps[0].Source, snaps[0].Confidence)
	}

	// case 5: fake-nocache (CacheTTL 0) → fetch on every call, never
	// cached (no cache file created by FetchAll)
	dir = t.TempDir()
	resetCalls()
	opts = Options{CacheDir: dir, Enabled: map[string]bool{"fake-nocache": true}}
	if _, _, err = FetchAll(ctx, []string{"fake-nocache"}, opts); err != nil {
		t.Fatalf("fake-nocache: err=%v", err)
	}
	if callCount("fake-nocache") != 1 {
		t.Fatalf("fake-nocache fetched %d times; want 1", callCount("fake-nocache"))
	}
	if _, serr := os.Stat(filepath.Join(dir, "fake-nocache.json")); !errors.Is(serr, os.ErrNotExist) {
		t.Fatalf("fake-nocache.json must not exist (CacheTTL 0), stat err = %v", serr)
	}

	// case 6: corrupt cache file → refetch, no error
	dir = t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fake-ok.json"), []byte("{"), 0o600); err != nil {
		t.Fatalf("corrupt seed: %v", err)
	}
	resetCalls()
	opts = Options{CacheDir: dir, Enabled: map[string]bool{"fake-ok": true}}
	if _, _, err = FetchAll(ctx, []string{"fake-ok"}, opts); err != nil {
		t.Fatalf("corrupt cache: err=%v, want nil (refetch)", err)
	}
	if callCount("fake-ok") != 1 {
		t.Fatalf("fake-ok fetched %d times; want 1 (corrupt cache must refetch)", callCount("fake-ok"))
	}
}

// decodeCacheIdentity reads <dir>/<id>.json (F13's cache-file shape)
// directly and returns the stored snapshot's account/plan.
func decodeCacheIdentity(t *testing.T, dir, id string) (account, plan string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, id+".json"))
	if err != nil {
		t.Fatalf("read cache file %s.json: %v", id, err)
	}
	var cf struct {
		Snapshot struct {
			Account string `json:"account"`
			Plan    string `json:"plan"`
		} `json:"snapshot"`
	}
	if err := json.Unmarshal(data, &cf); err != nil {
		t.Fatalf("decode cache file %s.json: %v", id, err)
	}
	return cf.Snapshot.Account, cf.Snapshot.Plan
}

// T4: Refresh, MaxAge, uncached providers (SPEC §4a–§4c).

func TestRefreshAndMaxAge(t *testing.T) {
	ctx := context.Background()
	seed := func(dir string) {
		t.Helper()
		seedCache(t, dir, "fake-ok", usage.Snapshot{Provider: "fake-ok", Account: "seeded"})
	}

	// case 1: Refresh bypasses a fresh cache entry → refetch + rewrite
	dir := t.TempDir()
	seed(dir)
	resetCalls()
	opts := Options{CacheDir: dir, Refresh: true, Enabled: map[string]bool{"fake-ok": true}}
	snaps, _, err := FetchAll(ctx, []string{"fake-ok"}, opts)
	if err != nil || len(snaps) != 1 {
		t.Fatalf("refresh: got %d snapshots, err=%v", len(snaps), err)
	}
	if callCount("fake-ok") != 1 {
		t.Fatalf("refresh must bypass fresh cache; fake-ok fetched %d times, want 1", callCount("fake-ok"))
	}
	acct, _ := decodeCacheIdentity(t, dir, "fake-ok")
	if acct != "acct" {
		t.Fatalf("cache not rewritten after refresh: account = %q, want acct (live)", acct)
	}

	// case 2: MaxAge 1ns → entry effectively stale → refetch
	dir = t.TempDir()
	seed(dir)
	resetCalls()
	opts = Options{CacheDir: dir, MaxAge: time.Nanosecond, Enabled: map[string]bool{"fake-ok": true}}
	if _, _, err = FetchAll(ctx, []string{"fake-ok"}, opts); err != nil {
		t.Fatalf("max-age stale: err=%v", err)
	}
	if callCount("fake-ok") != 1 {
		t.Fatalf("max-age must make the entry stale; fake-ok fetched %d times, want 1", callCount("fake-ok"))
	}

	// case 3: MaxAge 1h → served from cache
	dir = t.TempDir()
	seed(dir)
	resetCalls()
	opts = Options{CacheDir: dir, MaxAge: time.Hour, Enabled: map[string]bool{"fake-ok": true}}
	snaps, _, err = FetchAll(ctx, []string{"fake-ok"}, opts)
	if err != nil || len(snaps) != 1 {
		t.Fatalf("max-age fresh: got %d snapshots, err=%v", len(snaps), err)
	}
	if callCount("fake-ok") != 0 {
		t.Fatalf("max-age fresh must serve from cache; fake-ok fetched %d times, want 0", callCount("fake-ok"))
	}
	if snaps[0].Confidence != "cached" {
		t.Fatalf("confidence = %q, want cached", snaps[0].Confidence)
	}

	// case 4: control — fresh seed, no Refresh/MaxAge → served from cache
	dir = t.TempDir()
	seed(dir)
	resetCalls()
	opts = Options{CacheDir: dir, Enabled: map[string]bool{"fake-ok": true}}
	snaps, _, err = FetchAll(ctx, []string{"fake-ok"}, opts)
	if err != nil || len(snaps) != 1 {
		t.Fatalf("control: got %d snapshots, err=%v", len(snaps), err)
	}
	if callCount("fake-ok") != 0 {
		t.Fatalf("control must serve from cache; fake-ok fetched %d times, want 0", callCount("fake-ok"))
	}
	if snaps[0].Confidence != "cached" {
		t.Fatalf("control confidence = %q, want cached", snaps[0].Confidence)
	}

	// case 5: fake-nocache (CacheTTL 0) + manual fresh seed → refetched;
	// FetchAll creates no fake-nocache.json (the dir holds only the seed)
	dir = t.TempDir()
	seedCache(t, dir, "fake-nocache", usage.Snapshot{Provider: "fake-nocache"})
	resetCalls()
	opts = Options{CacheDir: dir, Enabled: map[string]bool{"fake-nocache": true}}
	if _, _, err = FetchAll(ctx, []string{"fake-nocache"}, opts); err != nil {
		t.Fatalf("uncached: err=%v", err)
	}
	if callCount("fake-nocache") != 1 {
		t.Fatalf("CacheTTL 0 must refetch every call; fetched %d times, want 1", callCount("fake-nocache"))
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read cache dir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "fake-nocache.json" {
		t.Fatalf("cache dir = %v; want only the manual seed fake-nocache.json (FetchAll wrote nothing)", dirEntryNames(entries))
	}
}

func dirEntryNames(entries []os.DirEntry) []string {
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}
