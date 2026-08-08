//go:build !nousage

package cache

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

func TestInvalidateRemovesFile(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	writeSnapshot(t, &s, "p", usage.Snapshot{Provider: "p"})
	if err := s.Invalidate("p"); err != nil {
		t.Fatalf("Invalidate() error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(s.Dir, "p.json")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("p.json still exists (stat err %v)", err)
	}
	if _, _, err := s.Read("p", time.Hour); !errors.Is(err, ErrCacheMiss) {
		t.Errorf("Read after Invalidate = %v, want ErrCacheMiss", err)
	}
}

func TestInvalidateMissingIsNil(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	if err := s.Invalidate("p"); err != nil {
		t.Errorf("Invalidate(missing) = %v, want nil (idempotent)", err)
	}
}

func TestInvalidateMissingDirIsNil(t *testing.T) {
	s := Store{Dir: filepath.Join(t.TempDir(), "absent")}
	if err := s.Invalidate("p"); err != nil {
		t.Errorf("Invalidate on missing dir = %v, want nil", err)
	}
}

func TestInvalidateLeavesOtherProviders(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	writeSnapshot(t, &s, "p", usage.Snapshot{Provider: "p"})
	writeSnapshot(t, &s, "q", usage.Snapshot{Provider: "q"})
	if err := s.Invalidate("p"); err != nil {
		t.Fatalf("Invalidate() error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(s.Dir, "q.json")); err != nil {
		t.Errorf("q.json removed or unreadable: %v", err)
	}
	if _, err := os.Stat(filepath.Join(s.Dir, "p.json")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("p.json still exists (stat err %v)", err)
	}
}
