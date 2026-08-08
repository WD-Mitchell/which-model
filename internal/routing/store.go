package routing

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// TableSchemaVersion is the current on-disk schema version for routes.json
// (specs/features/F18-routing/CONTRACTS.md §5).
const TableSchemaVersion = "1.0"

// Table is the persisted route table (specs/features/F18-routing/CONTRACTS.md
// §5, on-disk shape §6).
type Table struct {
	SchemaVersion string            `json:"schema_version"`
	ScoresHash    string            `json:"scores_sha256"`
	RefreshedAt   map[string]string `json:"refreshed_at"`
	Routes        []Route           `json:"routes"`
}

// SaveTable writes t to path atomically: a temp file in the same directory,
// fully written, then renamed over path (specs/features/F18-routing/SPEC.md
// §2.11).
func SaveTable(path string, t Table) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".routes-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()

	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return nil
}

// LoadTable reads and parses the route table at path. A read error (e.g. the
// file does not exist) is returned as-is so os.IsNotExist(err) works; a
// decode failure is wrapped.
func LoadTable(path string) (Table, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Table{}, err
	}
	var t Table
	if err := json.Unmarshal(data, &t); err != nil {
		return Table{}, fmt.Errorf("routes: corrupt table %s: %w", path, err)
	}
	return t, nil
}

// Stale reports whether t was built against a scores CSV different from
// currentScoresHash. A stale table is a warning for the caller, never an
// error (specs/features/F18-routing/SPEC.md §2.12).
func (t Table) Stale(currentScoresHash string) bool {
	return t.ScoresHash != currentScoresHash
}

// ProvenanceCounts tallies t.Routes per Provenance, in encounter order; only
// provenance values actually present appear as keys.
func (t Table) ProvenanceCounts() map[Provenance]int {
	counts := make(map[Provenance]int)
	for _, route := range t.Routes {
		counts[route.Provenance]++
	}
	return counts
}
