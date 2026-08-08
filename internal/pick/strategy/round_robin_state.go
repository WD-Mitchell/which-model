package strategy

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/gofrs/flock"
)

// stateFilePath returns the round-robin cursor file path under dataDir
// (specs/features/F20-strategies/SPEC.md §2.4).
func stateFilePath(dataDir string) string {
	return filepath.Join(dataDir, "pick", "round_robin.json")
}

// scopeKey derives a stable, order-independent key for one round-robin scope
// (a profile + its route-key set).
func scopeKey(profile string, routeKeys []string) string {
	keys := make([]string, len(routeKeys))
	copy(keys, routeKeys)
	sort.Strings(keys)
	joined := ""
	for i, k := range keys {
		if i > 0 {
			joined += "|"
		}
		joined += k
	}
	sum := sha256.Sum256([]byte(profile + "|" + joined))
	return hex.EncodeToString(sum[:])[:16]
}

// cursorDoc is one scope's persisted round-robin cursor.
type cursorDoc struct {
	Index     int    `json:"index"`
	UpdatedAt string `json:"updated_at"`
}

// roundRobinFile is the on-disk cursor document: scope key -> cursorDoc.
type roundRobinFile map[string]cursorDoc

// loadCursor returns the persisted cursor index for key, or 0 when the file
// is absent, unreadable, or corrupt (corruption is never an error — SPEC
// §2.4 step 9).
func loadCursor(dataDir, key string) (int, error) {
	data, err := os.ReadFile(stateFilePath(dataDir))
	if err != nil {
		return 0, nil
	}
	var m roundRobinFile
	if err := json.Unmarshal(data, &m); err != nil {
		return 0, nil
	}
	doc, ok := m[key]
	if !ok {
		return 0, nil
	}
	return doc.Index, nil
}

// saveCursor persists index for key under an exclusive flock, read-modifying
// the whole file so concurrent scopes never clobber each other
// (specs/features/F20-strategies/SPEC.md §2.4 step 8).
func saveCursor(dataDir, key string, index int) error {
	if err := os.MkdirAll(filepath.Join(dataDir, "pick"), 0o700); err != nil {
		return err
	}
	path := stateFilePath(dataDir)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()

	fl := flock.New(path)
	if err := fl.Lock(); err != nil {
		return err
	}
	defer fl.Unlock()

	m := make(roundRobinFile)
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &m) // corrupt/empty content -> start fresh
	}
	m[key] = cursorDoc{Index: index, UpdatedAt: time.Now().UTC().Format(time.RFC3339)}

	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	if err := f.Truncate(0); err != nil {
		return err
	}
	if _, err := f.WriteAt(out, 0); err != nil {
		return err
	}
	return f.Sync()
}

// nextCursor atomically reads key's current index and, unless dryRun,
// advances it by one — all under a single flock so concurrent callers never
// observe (or clobber) each other's read-modify-write (SPEC §2.4 step 8).
// loadCursor/saveCursor alone cannot provide this: a separate unlocked read
// followed by a locked write leaves a race window between them.
func nextCursor(dataDir, key string, dryRun bool) (int, error) {
	if err := os.MkdirAll(filepath.Join(dataDir, "pick"), 0o700); err != nil {
		return 0, err
	}
	path := stateFilePath(dataDir)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	fl := flock.New(path)
	if err := fl.Lock(); err != nil {
		return 0, err
	}
	defer fl.Unlock()

	m := make(roundRobinFile)
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &m) // corrupt/empty content -> start fresh
	}
	index := m[key].Index
	if dryRun {
		return index, nil
	}

	m[key] = cursorDoc{Index: index + 1, UpdatedAt: time.Now().UTC().Format(time.RFC3339)}
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return 0, err
	}
	if err := f.Truncate(0); err != nil {
		return 0, err
	}
	if _, err := f.WriteAt(out, 0); err != nil {
		return 0, err
	}
	if err := f.Sync(); err != nil {
		return 0, err
	}
	return index, nil
}
