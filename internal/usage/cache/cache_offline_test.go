//go:build !nousage

package cache

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

// listDir returns the sorted names in dir (or nil if it does not exist).
func listDir(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
	}
	return names
}

func assertFallback(t *testing.T, got usage.Snapshot) {
	t.Helper()
	if got.Provider != "p" {
		t.Errorf("Provider = %q, want p", got.Provider)
	}
	if got.Failure == nil {
		t.Fatal("Failure = nil, want fallback_unavailable")
	}
	if got.Failure.Code != "fallback_unavailable" {
		t.Errorf("Failure.Code = %q, want fallback_unavailable", got.Failure.Code)
	}
	if got.Failure.Message != "offline and no cached usage" {
		t.Errorf("Failure.Message = %q, want %q", got.Failure.Message, "offline and no cached usage")
	}
}

func TestOfflineReadMissing(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	assertFallback(t, s.OfflineRead("p", time.Hour))
}

func TestOfflineReadCorrupt(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	if err := os.WriteFile(filepath.Join(s.Dir, "p.json"), []byte("{"), 0o600); err != nil {
		t.Fatalf("write corrupt file: %v", err)
	}
	assertFallback(t, s.OfflineRead("p", time.Hour))
}

func TestOfflineReadFresh(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	snap := usage.Snapshot{Provider: "p", UsageKnown: true, Windows: []usage.Window{{ID: "w1", Used: f64(5), Limit: f64(10)}}}
	writeSnapshot(t, &s, "p", snap)
	got := s.OfflineRead("p", time.Hour)
	if got.Stale {
		t.Error("Stale = true, want false for fresh entry")
	}
	if !reflect.DeepEqual(got, snap) {
		t.Errorf("snapshot = %+v, want %+v", got, snap)
	}
}

func TestOfflineReadStale(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	snap := usage.Snapshot{Provider: "p", UsageKnown: true, Windows: []usage.Window{{ID: "w1"}}}
	writeSnapshot(t, &s, "p", snap)
	got := s.OfflineRead("p", time.Nanosecond)
	if !got.Stale {
		t.Error("Stale = false, want true for stale entry")
	}
	want := snap
	want.Stale = true
	if !reflect.DeepEqual(got, want) {
		t.Errorf("snapshot = %+v, want %+v", got, want)
	}
}

func TestOfflineReadNoCacheTTLZero(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	writeSnapshot(t, &s, "p", usage.Snapshot{Provider: "p"})
	assertFallback(t, s.OfflineRead("p", 0))
}

func TestOfflineReadNeverWrites(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	writeSnapshot(t, &s, "p", usage.Snapshot{Provider: "p"})
	before := listDir(t, s.Dir)
	s.OfflineRead("p", time.Hour)
	s.OfflineRead("missing", time.Hour)
	after := listDir(t, s.Dir)
	if !reflect.DeepEqual(before, after) {
		t.Errorf("dir listing changed: before %v, after %v", before, after)
	}

	// Read-only invariant also holds when the dir does not exist: no
	// directory is created.
	absent := filepath.Join(t.TempDir(), "absent")
	s2 := Store{Dir: absent}
	assertFallback(t, s2.OfflineRead("p", time.Hour))
	if _, err := os.Stat(absent); !os.IsNotExist(err) {
		t.Errorf("OfflineRead created missing dir %s", absent)
	}
}
