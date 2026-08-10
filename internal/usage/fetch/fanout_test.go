//go:build !nousage

package fetch

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

// T5: fan-out, partial failure, concurrency cap, scrubbing (SPEC §8–§9).

func TestFanoutPartialFailure(t *testing.T) {
	// case 1: fake-ok + fake-fail + fake-slow → all three snapshots
	// present, err == nil, the failed one has provider_status
	opts := newOptions(t)
	opts.Enabled = map[string]bool{"fake-ok": true, "fake-fail": true, "fake-slow": true}
	snaps, _, err := FetchAll(context.Background(), []string{"fake-ok", "fake-fail", "fake-slow"}, opts)
	if err != nil {
		t.Fatalf("err = %v, want nil (partial failure is data)", err)
	}
	if len(snaps) != 3 {
		t.Fatalf("got %d snapshots, want 3", len(snaps))
	}
	byID := map[string]bool{}
	for _, s := range snaps {
		byID[s.Provider] = true
		if s.Provider == "fake-fail" {
			if s.Failure == nil || s.Failure.Code != "provider_status" {
				t.Fatalf("fake-fail failure = %+v, want provider_status", s.Failure)
			}
		} else if s.Failure != nil {
			t.Fatalf("%s carries failure %+v", s.Provider, s.Failure)
		}
	}
	for _, id := range []string{"fake-ok", "fake-fail", "fake-slow"} {
		if !byID[id] {
			t.Fatalf("missing snapshot for %s", id)
		}
	}
}

func TestConcurrencyCap(t *testing.T) {
	slowIDs := []string{"fake-slow", "fake-slow2", "fake-slow3", "fake-slow4"}

	// case 2: MaxParallel 2 → peak concurrency ≤ 2, elapsed ≥ 600ms
	// (4 × 300ms / 2)
	slowMax.Store(0)
	opts := newOptions(t)
	opts.MaxParallel = 2
	opts.Enabled = map[string]bool{"fake-slow": true, "fake-slow2": true, "fake-slow3": true, "fake-slow4": true}
	start := time.Now()
	_, _, err := FetchAll(context.Background(), slowIDs, opts)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if peak := slowPeak(); peak > 2 {
		t.Fatalf("peak concurrency = %d, want ≤ 2", peak)
	}
	if elapsed < 600*time.Millisecond {
		t.Fatalf("elapsed = %v, want ≥ 600ms (4 × 300ms / 2)", elapsed)
	}

	// case 3: MaxParallel 0 → default cap (8) → all 4 run concurrently
	slowMax.Store(0)
	opts = newOptions(t)
	opts.Enabled = map[string]bool{"fake-slow": true, "fake-slow2": true, "fake-slow3": true, "fake-slow4": true}
	start = time.Now()
	_, _, err = FetchAll(context.Background(), slowIDs, opts)
	elapsed = time.Since(start)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if peak := slowPeak(); peak > 4 {
		t.Fatalf("peak concurrency = %d, want ≤ 4", peak)
	}
	if elapsed >= 600*time.Millisecond {
		t.Fatalf("elapsed = %v, want < 600ms (all 4 concurrent)", elapsed)
	}
}

func TestScrubbing(t *testing.T) {
	// case 4: fake-leak embeds the resolved token in its error message;
	// the returned Failure.Message must carry <redacted>, never the canary
	const canary = "canary-9f3a2b1c4d5e6f78"
	t.Setenv("WHICH_MODEL_TEST_TOKEN", canary)
	opts := newOptions(t)
	opts.Enabled = map[string]bool{"fake-leak": true}
	snaps, _, err := FetchAll(context.Background(), []string{"fake-leak"}, opts)
	if err != nil || len(snaps) != 1 {
		t.Fatalf("fake-leak: got %d snapshots, err=%v", len(snaps), err)
	}
	if snaps[0].Failure == nil {
		t.Fatalf("fake-leak snapshot has no failure")
	}
	msg := snaps[0].Failure.Message
	if !strings.Contains(msg, "<redacted>") {
		t.Fatalf("failure message %q does not contain <redacted>", msg)
	}
	if strings.Contains(msg, canary) {
		t.Fatalf("failure message leaks the credential canary: %q", msg)
	}
}

func TestCacheWriteFailureWarning(t *testing.T) {
	// case 5: read-only cache dir → snapshot still returned, one warning
	// containing "failed to cache usage", err == nil
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	opts := Options{CacheDir: dir, Enabled: map[string]bool{"fake-ok": true}}
	snaps, warns, err := FetchAll(context.Background(), []string{"fake-ok"}, opts)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if len(snaps) != 1 {
		t.Fatalf("got %d snapshots, want 1", len(snaps))
	}
	found := false
	for _, w := range warns {
		if strings.Contains(w.Message, "failed to cache usage") {
			found = true
		}
	}
	if !found {
		t.Fatalf("warnings = %v, want one containing %q", warns, "failed to cache usage")
	}
}

func TestWarningsSortedByProvider(t *testing.T) {
	// Warnings appear in provider-sorted order: fake-ok's warning must
	// precede fake-warn2's regardless of request order.
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	opts := Options{CacheDir: dir, Enabled: map[string]bool{"fake-ok": true, "fake-warn2": true}}
	_, warns, err := FetchAll(context.Background(), []string{"fake-warn2", "fake-ok"}, opts)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	joined := ""
	for _, w := range warns {
		joined += w.Message + "\n"
	}
	okIdx := strings.Index(joined, "provider fake-ok:")
	w2Idx := strings.Index(joined, "provider fake-warn2:")
	if okIdx == -1 || w2Idx == -1 {
		t.Fatalf("warnings = %q, want one per provider (fake-ok and fake-warn2)", joined)
	}
	if okIdx > w2Idx {
		t.Fatalf("warnings out of provider-sorted order: %q", joined)
	}
}

func TestSharedCancellation(t *testing.T) {
	// case 6: shared ctx cancelled → err == context.Canceled (partial
	// results are allowed)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	opts := newOptions(t)
	opts.Enabled = map[string]bool{"fake-ok": true}
	_, _, err := FetchAll(ctx, []string{"fake-ok"}, opts)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}
