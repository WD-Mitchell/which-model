//go:build !nousage

package cache

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// cacheBase points os.UserCacheDir at a temp dir and returns the base it
// resolves to. darwin ignores XDG_CACHE_HOME (it uses ~/Library/Caches),
// so fall back to HOME when the platform does; the dir must always end
// up under the temp root — never the real user cache dir.
func cacheBase(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", root)
	base, err := os.UserCacheDir()
	if err != nil {
		t.Fatalf("os.UserCacheDir() after setting XDG_CACHE_HOME: %v", err)
	}
	if !strings.HasPrefix(base, root) {
		t.Setenv("HOME", root)
		base, err = os.UserCacheDir()
		if err != nil {
			t.Fatalf("os.UserCacheDir() after setting HOME: %v", err)
		}
	}
	if !strings.HasPrefix(base, root) {
		t.Fatalf("os.UserCacheDir() = %q, want under temp root %q", base, root)
	}
	return base
}

func TestCacheDir(t *testing.T) {
	base := cacheBase(t)
	got, err := CacheDir()
	if err != nil {
		t.Fatalf("CacheDir() error: %v", err)
	}
	want := filepath.Join(base, "which-model", "usage-cache")
	if got != want {
		t.Errorf("CacheDir() = %q, want %q", got, want)
	}
}

func TestNew(t *testing.T) {
	base := cacheBase(t)
	s, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	want := filepath.Join(base, "which-model", "usage-cache")
	if s.Dir != want {
		t.Errorf("New().Dir = %q, want %q", s.Dir, want)
	}
}

func TestNewCreatesDirMode0700(t *testing.T) {
	cacheBase(t)
	s, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	info, err := os.Stat(s.Dir)
	if err != nil {
		t.Fatalf("stat %s: %v", s.Dir, err)
	}
	if !info.IsDir() {
		t.Errorf("%s is not a directory", s.Dir)
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Errorf("dir mode = %o, want 700", perm)
	}
}

func TestNewIdempotent(t *testing.T) {
	cacheBase(t)
	if _, err := New(); err != nil {
		t.Fatalf("first New(): %v", err)
	}
	if _, err := New(); err != nil {
		t.Fatalf("second New(): %v", err)
	}
}

func TestStoreValueConstructible(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	if s.Dir == "" {
		t.Fatal("Store{Dir: t.TempDir()} did not retain Dir")
	}
}
