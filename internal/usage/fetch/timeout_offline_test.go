//go:build !nousage

package fetch

import (
	"context"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

// T6: per-provider timeouts and offline mode (SPEC §5, §7, D4, D5).

func TestPerProviderTimeout(t *testing.T) {
	ctx := context.Background()

	// case 1: fake-block (desc.Timeout 0) with opts.Timeout 50ms →
	// timeout Failure, wall time < 2s
	opts := newOptions(t)
	opts.Timeout = 50 * time.Millisecond
	opts.Enabled = map[string]bool{"fake-block": true}
	start := time.Now()
	snaps, _, err := FetchAll(ctx, []string{"fake-block"}, opts)
	elapsed := time.Since(start)
	if err != nil || len(snaps) != 1 {
		t.Fatalf("fake-block: got %d snapshots, err=%v", len(snaps), err)
	}
	if snaps[0].Failure == nil || snaps[0].Failure.Code != "timeout" {
		t.Fatalf("fake-block failure = %+v, want code timeout", snaps[0].Failure)
	}
	if elapsed >= 2*time.Second {
		t.Fatalf("elapsed = %v, want < 2s", elapsed)
	}

	// case 2: fake-block + opts.Timeout 100ms (desc.Timeout 0) → timeout
	opts = newOptions(t)
	opts.Timeout = 100 * time.Millisecond
	opts.Enabled = map[string]bool{"fake-block": true}
	start = time.Now()
	snaps, _, err = FetchAll(ctx, []string{"fake-block"}, opts)
	elapsed = time.Since(start)
	if err != nil || len(snaps) != 1 {
		t.Fatalf("fake-block 100ms: got %d snapshots, err=%v", len(snaps), err)
	}
	if snaps[0].Failure == nil || snaps[0].Failure.Code != "timeout" {
		t.Fatalf("fake-block 100ms failure = %+v, want code timeout", snaps[0].Failure)
	}
	if elapsed >= 2*time.Second {
		t.Fatalf("elapsed = %v, want < 2s", elapsed)
	}
}

func TestMixedSlowAndFast(t *testing.T) {
	// case 3: fake-slow (300ms) + fake-ok (instant) → both snapshots
	// valid; elapsed < 2s proves the slow provider never delays the batch
	// beyond its own deadline
	opts := newOptions(t)
	opts.Enabled = map[string]bool{"fake-slow": true, "fake-ok": true}
	start := time.Now()
	snaps, _, err := FetchAll(context.Background(), []string{"fake-slow", "fake-ok"}, opts)
	elapsed := time.Since(start)
	if err != nil || len(snaps) != 2 {
		t.Fatalf("mixed: got %d snapshots, err=%v; want 2, nil", len(snaps), err)
	}
	for _, s := range snaps {
		if s.Failure != nil {
			t.Fatalf("unexpected failure %+v on %s", s.Failure, s.Provider)
		}
	}
	if elapsed >= 2*time.Second {
		t.Fatalf("elapsed = %v, want < 2s", elapsed)
	}
}

func TestOfflineMode(t *testing.T) {
	ctx := context.Background()

	// case 4: offline + fresh seed → served from cache, no fetch, no
	// writes (dir listing unchanged)
	dir := t.TempDir()
	seedCache(t, dir, "fake-ok", usage.Snapshot{Provider: "fake-ok", Account: "acct", Plan: "pro"})
	before := dirNames(t, dir)
	resetCalls()
	opts := Options{CacheDir: dir, Offline: true, Enabled: map[string]bool{"fake-ok": true}}
	snaps, _, err := FetchAll(ctx, []string{"fake-ok"}, opts)
	if err != nil || len(snaps) != 1 {
		t.Fatalf("offline fresh: got %d snapshots, err=%v", len(snaps), err)
	}
	if snaps[0].Source != usage.SourceCache || snaps[0].Confidence != "cached" {
		t.Fatalf("offline provenance = %q/%q, want cache/cached", snaps[0].Source, snaps[0].Confidence)
	}
	if callCount("fake-ok") != 0 {
		t.Fatalf("offline must not fetch; fake-ok fetched %d times", callCount("fake-ok"))
	}
	if after := dirNames(t, dir); !reflect.DeepEqual(after, before) {
		t.Fatalf("offline FetchAll modified the cache dir: before %v, after %v", before, after)
	}

	// case 5: offline + missing cache → fallback_unavailable
	opts = Options{CacheDir: t.TempDir(), Offline: true, Enabled: map[string]bool{"fake-nocache": true}}
	snaps, _, err = FetchAll(ctx, []string{"fake-nocache"}, opts)
	if err != nil || len(snaps) != 1 {
		t.Fatalf("offline missing: got %d snapshots, err=%v", len(snaps), err)
	}
	if snaps[0].Failure == nil || snaps[0].Failure.Code != "fallback_unavailable" {
		t.Fatalf("offline missing failure = %+v, want fallback_unavailable", snaps[0].Failure)
	}

	// case 6: offline + stale seed (MaxAge 1ns) → Stale true, no failure
	dir = t.TempDir()
	seedCache(t, dir, "fake-ok", usage.Snapshot{Provider: "fake-ok"})
	opts = Options{CacheDir: dir, Offline: true, MaxAge: time.Nanosecond, Enabled: map[string]bool{"fake-ok": true}}
	snaps, _, err = FetchAll(ctx, []string{"fake-ok"}, opts)
	if err != nil || len(snaps) != 1 {
		t.Fatalf("offline stale: got %d snapshots, err=%v", len(snaps), err)
	}
	if !snaps[0].Stale {
		t.Fatalf("offline stale snapshot: Stale = false, want true")
	}
	if snaps[0].Failure != nil {
		t.Fatalf("offline stale snapshot has failure %+v", snaps[0].Failure)
	}
	if snaps[0].Source != usage.SourceCache || snaps[0].Confidence != "cached" {
		t.Fatalf("offline stale provenance = %q/%q, want cache/cached", snaps[0].Source, snaps[0].Confidence)
	}
}

// dirNames returns the sorted file names under dir.
func dirNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir %s: %v", dir, err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}
