package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// fixtureJSONL is the shared aggregation fixture (B11 CONTRACTS §5):
// two profiles with multiple picks each, out-of-order timestamps, differing
// UTC offsets for the same instant (tie kept by later file order), a no-pick
// line, a blank line, a whitespace-only line, a truncated-JSON line, a line
// with empty profile, and a line with an unparseable ts.
const fixtureJSONL = `{"ulid":"01WORK00000000000000000001","ts":"2026-01-02T10:00:00Z","profile":"work","strategy":"balanced","candidate_id":"claude/claude-opus-5@high","final_score":91.25,"excluded_count":0,"evidence":{}}
{"ulid":"01WORK00000000000000000002","ts":"2026-01-01T09:00:00Z","profile":"work","strategy":"balanced","candidate_id":"codex/gpt-6@medium","final_score":84.1,"excluded_count":1,"evidence":{}}
{"ulid":"01WORK00000000000000000003","ts":"2026-01-02T12:00:00+02:00","profile":"work","strategy":"balanced","candidate_id":"claude/claude-opus-5@high","final_score":90.5,"excluded_count":0,"evidence":{}}
{"ulid":"01PERS00000000000000000001","ts":"2026-03-05T08:00:00Z","profile":"personal","strategy":"cheap","candidate_id":"codex/gpt-6@low","final_score":70.02,"excluded_count":0,"evidence":{}}
{"ulid":"01PERS00000000000000000002","ts":"2026-03-06T08:00:00-01:00","profile":"personal","strategy":"cheap","candidate_id":"codex/gpt-6@low","final_score":71.9,"excluded_count":2,"evidence":{}}
{"ulid":"01PERS00000000000000000003","ts":"2026-04-01T00:00:00Z","profile":"personal","strategy":"cheap","candidate_id":"","final_score":0,"excluded_count":3,"evidence":{}}


{"ulid":"01TRUNC000000000000000000","ts":"2026-
{"ulid":"01EMPTY000000000000000001","ts":"2026-05-01T00:00:00Z","profile":"","strategy":"balanced","candidate_id":"claude/claude-opus-5@high","final_score":50,"excluded_count":0,"evidence":{}}
{"ulid":"01BADTS000000000000000001","ts":"yesterday","profile":"work","strategy":"balanced","candidate_id":"claude/claude-opus-5@high","final_score":50,"excluded_count":0,"evidence":{}}
`

// cliLineJSONL mirrors a line the CLI writer in pkg/whichmodel/pick.go emits:
// a full annex-c Evidence object with band, excluded_candidates, etc. It
// guards the comment-sync clause — B11 must decode and count it verbatim.
const cliLineJSONL = `{"ulid":"01JCLI0000000000000000000A","ts":"2026-05-01T12:34:56Z","profile":"cli_profile","strategy":"balanced","candidate_id":"claude/claude-opus-5@high","final_score":88.42,"excluded_count":1,"evidence":{"profile":"cli_profile","score_inputs":{"intelligence":90.1,"cost":70.2},"band":{"name":"medium","used_percent":42.5,"weight":0.9},"snapshot_age_seconds":120,"confidence":"live","route_provenance":"provider_live","excluded_candidates":[{"route":{"provider":"codex","model_id":"gpt-6","model":"GPT-6","reasoning":"high","window_ids":["session"]},"reason_code":"band_gated","reason":"weekly window above threshold"}],"last_verified":"2026-05-01T12:34:00Z"}}
`

func writeHistoryFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pick", "history.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAggregatePicksGolden(t *testing.T) {
	path := writeHistoryFile(t, fixtureJSONL)
	stats, skipped, err := AggregatePicks(path)
	if err != nil {
		t.Fatalf("AggregatePicks: %v", err)
	}
	want := map[string]ProfileStats{
		"work":     {Picks: 3, LastUsed: "2026-01-02T12:00:00+02:00"},
		"personal": {Picks: 2, LastUsed: "2026-03-06T08:00:00-01:00"},
	}
	if !reflect.DeepEqual(stats, want) {
		t.Errorf("stats = %#v, want %#v", stats, want)
	}
	if skipped != 3 {
		t.Errorf("skipped = %d, want 3", skipped)
	}
}

func TestAggregatePicksMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pick", "history.jsonl")
	stats, skipped, err := AggregatePicks(path)
	if err != nil {
		t.Fatalf("AggregatePicks: %v", err)
	}
	if stats == nil {
		t.Fatal("stats map is nil, want empty non-nil map")
	}
	if len(stats) != 0 || skipped != 0 {
		t.Errorf("stats = %v, skipped = %d; want empty map, 0", stats, skipped)
	}
}

func TestAggregatePicksEmptyFile(t *testing.T) {
	path := writeHistoryFile(t, "")
	stats, skipped, err := AggregatePicks(path)
	if err != nil {
		t.Fatalf("AggregatePicks: %v", err)
	}
	if stats == nil {
		t.Fatal("stats map is nil, want empty non-nil map")
	}
	if len(stats) != 0 || skipped != 0 {
		t.Errorf("stats = %v, skipped = %d; want empty map, 0", stats, skipped)
	}
}

func TestAppendPickRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pick", "history.jsonl")
	rawEvidence := `{"profile":"work","score_inputs":{"intelligence":90.1},"route_provenance":"models_dev","excluded_candidates":[]}`
	first := PickHistoryEntry{
		ULID:        "01APPEND000000000000000001",
		TS:          "2026-06-01T10:00:00Z",
		Profile:     "work",
		Strategy:    "balanced",
		CandidateID: "claude/claude-opus-5@high",
		FinalScore:  92.11,
		Evidence:    nil, // must be written as {}
	}
	second := PickHistoryEntry{
		ULID:          "01APPEND000000000000000002",
		TS:            "2026-06-02T10:00:00Z",
		Profile:       "work",
		Strategy:      "balanced",
		CandidateID:   "codex/gpt-6@medium",
		FinalScore:    85.4,
		ExcludedCount: 1,
		Evidence:      json.RawMessage(rawEvidence),
	}
	if err := AppendPick(path, first); err != nil {
		t.Fatalf("AppendPick first: %v", err)
	}
	if err := AppendPick(path, second); err != nil {
		t.Fatalf("AppendPick second: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("file has %d lines, want 2:\n%s", len(lines), data)
	}
	if !strings.Contains(lines[0], `"evidence":{}`) {
		t.Errorf("nil-Evidence line missing \"evidence\":{}: %s", lines[0])
	}

	var got PickHistoryEntry
	if err := json.Unmarshal([]byte(lines[1]), &got); err != nil {
		t.Fatalf("re-decode second line: %v", err)
	}
	if string(got.Evidence) != rawEvidence {
		t.Errorf("evidence not byte-identical:\n got %s\nwant %s", got.Evidence, rawEvidence)
	}

	stats, skipped, err := AggregatePicks(path)
	if err != nil {
		t.Fatalf("AggregatePicks: %v", err)
	}
	if skipped != 0 {
		t.Errorf("skipped = %d, want 0", skipped)
	}
	if stats["work"].Picks != 2 {
		t.Errorf("work picks = %d, want 2", stats["work"].Picks)
	}
	if stats["work"].LastUsed != "2026-06-02T10:00:00Z" {
		t.Errorf("work last_used = %q, want 2026-06-02T10:00:00Z", stats["work"].LastUsed)
	}
}

func TestAppendPickCreatesParents(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "state", "pick", "history.jsonl")
	entry := PickHistoryEntry{
		ULID:        "01APPEND000000000000000003",
		TS:          "2026-06-03T10:00:00Z",
		Profile:     "work",
		Strategy:    "balanced",
		CandidateID: "claude/claude-opus-5@high",
		FinalScore:  90,
	}
	if err := AppendPick(path, entry); err != nil {
		t.Fatalf("AppendPick: %v", err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("history file not created: %v", err)
	}
	if runtime.GOOS != "windows" {
		if got := fi.Mode().Perm(); got != 0o600 {
			t.Errorf("file mode = %o, want 600", got)
		}
		for _, dir := range []string{filepath.Join(root, "state"), filepath.Join(root, "state", "pick")} {
			di, err := os.Stat(dir)
			if err != nil {
				t.Fatalf("parent dir not created: %v", err)
			}
			if got := di.Mode().Perm(); got != 0o700 {
				t.Errorf("dir %s mode = %o, want 700", dir, got)
			}
		}
	}
}

func TestAppendPickValidation(t *testing.T) {
	cases := []struct {
		name    string
		entry   PickHistoryEntry
		wantErr string
	}{
		{
			name:    "empty profile",
			entry:   PickHistoryEntry{ULID: "01X", TS: "2026-06-01T10:00:00Z", Profile: "", CandidateID: "c"},
			wantErr: "history: entry profile must not be empty",
		},
		{
			name:    "bad timestamp",
			entry:   PickHistoryEntry{ULID: "01X", TS: "yesterday", Profile: "work", CandidateID: "c"},
			wantErr: `history: entry ts "yesterday" is not RFC3339`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "pick", "history.jsonl")
			err := AppendPick(path, tc.entry)
			if err == nil {
				t.Fatal("AppendPick returned nil error")
			}
			if err.Error() != tc.wantErr {
				t.Errorf("error = %q, want %q", err.Error(), tc.wantErr)
			}
			if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
				t.Errorf("file was created on validation failure (stat err = %v)", statErr)
			}
		})
	}
}

func TestAggregatePicksCLIInterleaving(t *testing.T) {
	path := writeHistoryFile(t, cliLineJSONL)
	stats, skipped, err := AggregatePicks(path)
	if err != nil {
		t.Fatalf("AggregatePicks: %v", err)
	}
	if skipped != 0 {
		t.Errorf("skipped = %d, want 0", skipped)
	}
	want := ProfileStats{Picks: 1, LastUsed: "2026-05-01T12:34:56Z"}
	if got := stats["cli_profile"]; got != want {
		t.Errorf("cli_profile stats = %#v, want %#v", got, want)
	}
}
