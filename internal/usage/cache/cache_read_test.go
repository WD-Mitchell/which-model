//go:build !nousage

package cache

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

func writeSnapshot(t *testing.T, s *Store, providerID string, snap usage.Snapshot) {
	t.Helper()
	if err := s.Write(providerID, snap); err != nil {
		t.Fatalf("Write(%q): %v", providerID, err)
	}
}

func TestReadFresh(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	snap := usage.Snapshot{Provider: "p", UsageKnown: true, Windows: []usage.Window{{ID: "w1", Used: f64(5), Limit: f64(10)}}}
	writeSnapshot(t, &s, "p", snap)
	got, stale, err := s.Read("p", time.Hour)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if stale {
		t.Error("stale = true, want false for fresh entry")
	}
	if !reflect.DeepEqual(got, snap) {
		t.Errorf("snapshot = %+v, want %+v", got, snap)
	}
}

func TestReadStale(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	snap := usage.Snapshot{Provider: "p", UsageKnown: true, Windows: []usage.Window{{ID: "w1"}}}
	writeSnapshot(t, &s, "p", snap)
	got, stale, err := s.Read("p", time.Nanosecond)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if !stale {
		t.Error("stale = false, want true for old entry")
	}
	if !reflect.DeepEqual(got, snap) {
		t.Errorf("snapshot = %+v, want %+v", got, snap)
	}
}

func TestReadMissing(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	_, _, err := s.Read("nope", time.Hour)
	if err == nil {
		t.Fatal("Read(missing) = nil, want error")
	}
	if !errors.Is(err, ErrCacheMiss) {
		t.Errorf("error %v does not wrap ErrCacheMiss", err)
	}
}

func TestReadNoCacheTTLZero(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	writeSnapshot(t, &s, "p", usage.Snapshot{Provider: "p"})
	_, _, err := s.Read("p", 0)
	if err == nil {
		t.Fatal("Read(p, 0) = nil, want error")
	}
	if !errors.Is(err, ErrCacheMiss) {
		t.Errorf("error %v does not wrap ErrCacheMiss", err)
	}
}

func TestReadCorrupt(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	path := filepath.Join(s.Dir, "p.json")
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatalf("write corrupt file: %v", err)
	}
	_, _, err := s.Read("p", time.Hour)
	if err == nil {
		t.Fatal("Read(corrupt) = nil, want error")
	}
	if errors.Is(err, ErrCacheMiss) {
		t.Errorf("corrupt file must be a plain error, not ErrCacheMiss: %v", err)
	}
	if _, ok := usage.AsFailure(err); ok {
		t.Errorf("corrupt file must be a plain error, not a *usage.FailureError: %v", err)
	}
}

func TestReadOversized(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	path := filepath.Join(s.Dir, "p.json")
	big := make([]byte, 4.5*1024*1024) // 4.5 MiB > 4 MiB bound (SPEC D7)
	if err := os.WriteFile(path, big, 0o600); err != nil {
		t.Fatalf("write oversized file: %v", err)
	}
	_, _, err := s.Read("p", time.Hour)
	if err == nil {
		t.Fatal("Read(oversized) = nil, want error")
	}
	if !strings.Contains(err.Error(), "4 MiB") {
		t.Errorf("error %q does not mention 4 MiB", err)
	}
}

func TestReadSnapshotUntouched(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	snap := usage.Snapshot{
		Provider:   "p",
		Account:    "acct@example.com",
		Plan:       "pro",
		UsageKnown: true,
		Windows:    []usage.Window{{ID: "w1"}},
		// Source/Confidence/Stale intentionally left zero: F14 stamps them (SPEC D6).
	}
	writeSnapshot(t, &s, "p", snap)
	got, _, err := s.Read("p", time.Hour)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if got.Source != "" {
		t.Errorf("Source = %q, want empty (F14 stamps it)", got.Source)
	}
	if got.Confidence != "" {
		t.Errorf("Confidence = %q, want empty (F14 stamps it)", got.Confidence)
	}
	if got.Stale {
		t.Error("Stale = true, want false (F14 stamps it)")
	}
	if got.Account != "acct@example.com" || got.Plan != "pro" {
		t.Errorf("identity = account %q plan %q, want acct@example.com / pro", got.Account, got.Plan)
	}
	if !reflect.DeepEqual(got, snap) {
		t.Errorf("snapshot = %+v, want stored %+v untouched", got, snap)
	}
}
