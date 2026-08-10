//go:build !nousage

package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

func f64(v float64) *float64   { return &v }
func i32(v int) *int           { return &v }
func tm(v time.Time) *time.Time { return &v }

// readFileJSON decodes a cache file into its on-disk shape (CONTRACTS §2).
func readFileJSON(t *testing.T, path string) cacheFile {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var cf cacheFile
	if err := json.Unmarshal(data, &cf); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return cf
}

func TestWriteCreatesFile(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	snap := usage.Snapshot{
		Provider:   "p1",
		UsageKnown: true,
		Windows:    []usage.Window{{ID: "w1", Used: f64(5), Limit: f64(10)}},
	}
	if err := s.Write("p1", snap); err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	path := filepath.Join(s.Dir, "p1.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
}

func TestWriteFileContent(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	before := time.Now()
	snap := usage.Snapshot{Provider: "p1", UsageKnown: true, Windows: []usage.Window{{ID: "w1"}}}
	if err := s.Write("p1", snap); err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	cf := readFileJSON(t, filepath.Join(s.Dir, "p1.json"))
	if cf.Snapshot.Provider != "p1" {
		t.Errorf("snapshot.provider = %q, want p1", cf.Snapshot.Provider)
	}
	if !cf.Snapshot.UsageKnown {
		t.Error("snapshot.usage_known = false, want true")
	}
	if cf.FetchedAt.IsZero() {
		t.Fatal("fetched_at is zero")
	}
	if since := time.Since(cf.FetchedAt); since < 0 || since > 5*time.Second {
		t.Errorf("fetched_at = %v, want within 5s of now (before %v)", cf.FetchedAt, before)
	}
}

func TestWriteFileMode0600(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	if err := s.Write("p1", usage.Snapshot{Provider: "p1"}); err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	info, err := os.Stat(filepath.Join(s.Dir, "p1.json"))
	if err != nil {
		t.Fatalf("stat p1.json: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("file mode = %o, want 600", perm)
	}
}

func TestWriteReplacesAndLeavesNoTempFiles(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	if err := s.Write("p1", usage.Snapshot{Provider: "p1", Windows: []usage.Window{{ID: "w1"}}}); err != nil {
		t.Fatalf("first Write() error: %v", err)
	}
	if err := s.Write("p1", usage.Snapshot{Provider: "p1", Windows: []usage.Window{{ID: "w2"}}}); err != nil {
		t.Fatalf("second Write() error: %v", err)
	}
	cf := readFileJSON(t, filepath.Join(s.Dir, "p1.json"))
	if len(cf.Snapshot.Windows) != 1 || cf.Snapshot.Windows[0].ID != "w2" {
		t.Errorf("windows = %+v, want [w2]", cf.Snapshot.Windows)
	}
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "p1.json" {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		t.Errorf("dir entries = %v, want exactly [p1.json]", names)
	}
}

func TestWriteRefusesFailedSnapshot(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	good := usage.Snapshot{Provider: "p1", Windows: []usage.Window{{ID: "w1"}}}
	if err := s.Write("p1", good); err != nil {
		t.Fatalf("Write(good) error: %v", err)
	}
	failed := usage.Snapshot{Provider: "p1", Failure: &usage.Failure{Code: "timeout", Message: "x"}}
	err := s.Write("p1", failed)
	if err == nil {
		t.Fatal("Write(failed) = nil, want error")
	}
	if !strings.Contains(err.Error(), "refusing to cache a failed snapshot") {
		t.Errorf("error = %q, want containing %q", err, "refusing to cache a failed snapshot")
	}
	cf := readFileJSON(t, filepath.Join(s.Dir, "p1.json"))
	if len(cf.Snapshot.Windows) != 1 || cf.Snapshot.Windows[0].ID != "w1" {
		t.Errorf("existing file changed: windows = %+v, want [w1]", cf.Snapshot.Windows)
	}
}

func TestWriteCreatesMissingNestedDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "missing", "nested")
	s := Store{Dir: dir}
	if err := s.Write("p1", usage.Snapshot{Provider: "p1"}); err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat %s: %v", dir, err)
	}
	if !info.IsDir() {
		t.Errorf("%s is not a directory", dir)
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Errorf("dir mode = %o, want 700", perm)
	}
}

func TestWriteWindowRoundTrip(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	resetsAt := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	want := usage.Window{
		ID:            "monthly",
		Label:         "Monthly quota",
		Unit:          usage.UnitTokens,
		UsedPercent:   f64(42.5),
		Used:          f64(4250),
		Limit:         f64(10000),
		Remaining:     f64(5750),
		Unlimited:     false,
		WindowMinutes: i32(43200),
		ResetsAt:      tm(resetsAt),
		ResetHint:     "resets on the first of the month",
		ModelScope:    []string{"claude-3-7-sonnet", "claude-3-5-haiku"},
		Synthetic:     true,
		UsageKnown:    true,
	}
	snap := usage.Snapshot{
		Provider:   "p1",
		Account:    "acct@example.com",
		Plan:       "pro",
		UsageKnown: true,
		Windows:    []usage.Window{want},
	}
	if err := s.Write("p1", snap); err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	cf := readFileJSON(t, filepath.Join(s.Dir, "p1.json"))
	if cf.Snapshot.Account != "acct@example.com" || cf.Snapshot.Plan != "pro" {
		t.Errorf("identity round-trip = account %q plan %q, want acct@example.com / pro", cf.Snapshot.Account, cf.Snapshot.Plan)
	}
	if len(cf.Snapshot.Windows) != 1 {
		t.Fatalf("windows = %+v, want exactly one", cf.Snapshot.Windows)
	}
	if !reflect.DeepEqual(cf.Snapshot.Windows[0], want) {
		t.Errorf("window round-trip:\n got %+v\nwant %+v", cf.Snapshot.Windows[0], want)
	}
}
