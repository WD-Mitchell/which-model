// Package service is the desktop app's Wails-free programmatic surface over
// the which-model engine (B00 SPEC). This file (B11-history) holds the pure
// pick-history functions: no Services receiver, no lock, no events, no config.
package service

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// PickHistoryEntry is one line of <StateDir>/pick/history.jsonl.
// KEEP IN SYNC (by convention, field-for-field, identical JSON tags) with
// HistoryEntry in pkg/whichmodel/pick.go — the CLI owns the on-disk shape;
// B00 SPEC §2.3 forbids importing it. Evidence is opaque here (SPEC §2.1).
type PickHistoryEntry struct {
	ULID          string          `json:"ulid"` // 26-char ULID
	TS            string          `json:"ts"`   // RFC3339
	Profile       string          `json:"profile"`
	Strategy      string          `json:"strategy"`
	CandidateID   string          `json:"candidate_id"` // "" when no pick
	FinalScore    float64         `json:"final_score"`  // 0 when no pick
	ExcludedCount int             `json:"excluded_count"`
	Evidence      json.RawMessage `json:"evidence"` // round-tripped verbatim
}

// AggregatePicks streams the JSONL file at path and returns per-profile
// stats (SPEC §2.2–2.5): Picks counts lines with a non-empty candidate_id;
// LastUsed is the original ts string of the max-time counting line.
// skipped counts corrupt lines (bad JSON, empty profile, bad ts). Missing
// file -> empty non-nil map, 0, nil. Only real I/O errors are returned.
func AggregatePicks(path string) (stats map[string]ProfileStats, skipped int, err error) {
	stats = make(map[string]ProfileStats)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return stats, 0, nil
		}
		return nil, 0, fmt.Errorf("history: %w", err)
	}
	defer f.Close()

	lastParsed := make(map[string]time.Time)
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue // blank/whitespace-only: ignored, not skipped (SPEC §2.2)
		}
		var e PickHistoryEntry
		if json.Unmarshal([]byte(line), &e) != nil {
			skipped++
			continue
		}
		if e.Profile == "" {
			skipped++
			continue
		}
		ts, perr := time.Parse(time.RFC3339, e.TS)
		if perr != nil {
			skipped++
			continue
		}
		if e.CandidateID == "" {
			continue // valid no-pick run: ignored, not skipped (SPEC §2.3)
		}
		s := stats[e.Profile]
		s.Picks++
		// Ties keep the later line in file order (SPEC §2.3).
		if prev, ok := lastParsed[e.Profile]; !ok || !ts.Before(prev) {
			lastParsed[e.Profile] = ts
			s.LastUsed = e.TS
		}
		stats[e.Profile] = s
	}
	if serr := sc.Err(); serr != nil {
		return nil, 0, fmt.Errorf("history: %w", serr)
	}
	return stats, skipped, nil
}

// AppendPick validates entry (profile non-empty, ts RFC3339; messages
// CONTRACTS §4), creates parent dirs (0700), and appends one compact JSON
// line + "\n" via O_APPEND|O_CREATE|O_WRONLY, 0600. Nil Evidence is written
// as {}. No event; the caller emits pick:recorded (SPEC §2.6).
func AppendPick(path string, entry PickHistoryEntry) error {
	if entry.Profile == "" {
		return errors.New("history: entry profile must not be empty")
	}
	if _, err := time.Parse(time.RFC3339, entry.TS); err != nil {
		return fmt.Errorf("history: entry ts %q is not RFC3339", entry.TS)
	}
	if entry.Evidence == nil {
		entry.Evidence = json.RawMessage("{}")
	}
	line, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("history: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("history: %w", err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("history: %w", err)
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		f.Close()
		return fmt.Errorf("history: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("history: %w", err)
	}
	return nil
}
