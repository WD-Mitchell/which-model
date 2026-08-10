//go:build !nousage

// Package cache implements the per-provider usage snapshot cache:
// one JSON file per provider under the user cache directory, TTL-based
// staleness, atomic replacement writes, explicit invalidation, and a
// read-only offline path (specs/features/F13-usage-cache/SPEC.md).
package cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

// ErrCacheMiss is the wrapped sentinel for "no cache entry".
// Match with errors.Is(err, cache.ErrCacheMiss).
var ErrCacheMiss = errors.New("cache miss")

// Store reads and writes per-provider cache files under Dir.
// Zero value is not usable; construct via New().
type Store struct {
	Dir string
}

// CacheDir returns the absolute cache root:
// os.UserCacheDir()/which-model/usage-cache (SPEC D1). Error only if
// UserCacheDir fails (no HOME/XDG_CACHE_HOME).
func CacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("user cache dir: %w", err)
	}
	return filepath.Join(base, "which-model", "usage-cache"), nil
}

// New creates (if needed, mode 0700) the cache dir and returns a Store.
func New() (*Store, error) {
	dir, err := CacheDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create cache dir: %w", err)
	}
	return &Store{Dir: dir}, nil
}

// cacheFile is the on-disk JSON shape (CONTRACTS §2): the snapshot plus
// the write-time fetched_at used for staleness (SPEC D2). fetched_at is
// set by Write itself, never trusted from the payload.
type cacheFile struct {
	Snapshot  usage.Snapshot `json:"snapshot"`
	FetchedAt time.Time      `json:"fetched_at"`
}

// filePath returns the cache file path for one provider.
func (s *Store) filePath(providerID string) string {
	return filepath.Join(s.Dir, providerID+".json")
}

// Write atomically replaces <providerID>.json (SPEC D4): temp file +
// Sync + Chmod 0600 + Rename; dir created 0700 if missing. Refuses
// snapshots with Failure.Code != "" (never cache failures; SPEC D5).
func (s *Store) Write(providerID string, snap usage.Snapshot) error {
	if snap.Failure != nil && snap.Failure.Code != "" {
		return errors.New("refusing to cache a failed snapshot")
	}
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}
	tmp, err := os.CreateTemp(s.Dir, ".tmp-"+providerID+"-*")
	if err != nil {
		return fmt.Errorf("create temp cache file: %w", err)
	}
	if err := json.NewEncoder(tmp).Encode(cacheFile{Snapshot: snap, FetchedAt: time.Now()}); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("encode cache file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("sync cache file: %w", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("chmod cache file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("close cache file: %w", err)
	}
	if err := os.Rename(tmp.Name(), s.filePath(providerID)); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("rename cache file: %w", err)
	}
	return nil
}

// maxCacheFileSize bounds cache file reads; larger files count as
// corrupt (SPEC D7 — 4 MiB is 100x headroom over real snapshot sizes).
const maxCacheFileSize = 4 << 20

// Read loads one provider's snapshot. stale = now - fetched_at > ttl.
//   - ttl <= 0        → ErrCacheMiss (no cache for this provider)
//   - missing file    → error wrapping ErrCacheMiss
//   - corrupt/oversized (> 4 MiB) file → plain error (F14 refetches)
//   - valid file      → stored snapshot UNTOUCHED (F14 stamps
//     Source/Confidence/Stale afterwards; SPEC D6), stale bool, nil
func (s *Store) Read(providerID string, ttl time.Duration) (usage.Snapshot, bool, error) {
	if ttl <= 0 {
		return usage.Snapshot{}, false, fmt.Errorf("no cache configured for provider %q: %w", providerID, ErrCacheMiss)
	}
	path := s.filePath(providerID)
	stat, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return usage.Snapshot{}, false, fmt.Errorf("cache miss for provider %q: %w", providerID, ErrCacheMiss)
		}
		return usage.Snapshot{}, false, fmt.Errorf("stat cache entry for %q: %w", providerID, err)
	}
	if stat.Size() > maxCacheFileSize {
		return usage.Snapshot{}, false, fmt.Errorf("cache entry for %q exceeds 4 MiB", providerID)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return usage.Snapshot{}, false, fmt.Errorf("read cache entry for %q: %w", providerID, err)
	}
	var cf cacheFile
	if err := json.Unmarshal(data, &cf); err != nil {
		return usage.Snapshot{}, false, fmt.Errorf("decode cache entry for %q: %w", providerID, err)
	}
	stale := time.Since(cf.FetchedAt) > ttl
	return cf.Snapshot, stale, nil
}

// Invalidate removes <providerID>.json. Missing file → nil (idempotent).
func (s *Store) Invalidate(providerID string) error {
	if err := os.Remove(s.filePath(providerID)); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("invalidate cache entry for %q: %w", providerID, err)
	}
	return nil
}

// OfflineRead is the read-only offline path (SPEC D8): never writes,
// never fails. Missing/unreadable → Snapshot{Provider, Failure:
// {fallback_unavailable, "offline and no cached usage"}}. Stale file →
// stored snapshot with Stale=true. Fresh file → stored snapshot as-is.
func (s *Store) OfflineRead(providerID string, ttl time.Duration) usage.Snapshot {
	fallback := usage.Snapshot{
		Provider: providerID,
		Failure:  &usage.Failure{Code: "fallback_unavailable", Message: "offline and no cached usage"},
	}
	if ttl <= 0 {
		return fallback
	}
	stat, err := os.Stat(s.filePath(providerID))
	if err != nil {
		return fallback
	}
	if stat.Size() > maxCacheFileSize {
		return fallback
	}
	data, err := os.ReadFile(s.filePath(providerID))
	if err != nil {
		return fallback
	}
	var cf cacheFile
	if err := json.Unmarshal(data, &cf); err != nil {
		return fallback
	}
	if time.Since(cf.FetchedAt) > ttl {
		cf.Snapshot.Stale = true
	}
	return cf.Snapshot
}
