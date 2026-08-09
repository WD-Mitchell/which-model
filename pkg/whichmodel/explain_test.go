// F26-T10: explain command — flags, lookup, text render
// (specs/features/F26-cmd-pick/TASKS.md T10; SPEC §2.11; CONTRACTS §3, §6, §7).
package whichmodel

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// seedExplainHistory writes entries as JSONL into <state_dir>/pick/history.jsonl
// (the layout RunPick appends; T9 fixture convention).
func seedExplainHistory(t *testing.T, stateDir string, entries ...HistoryEntry) string {
	t.Helper()
	dir := filepath.Join(stateDir, "pick")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir history dir: %v", err)
	}
	var b strings.Builder
	for _, e := range entries {
		line, err := json.Marshal(e)
		if err != nil {
			t.Fatalf("marshal history entry: %v", err)
		}
		b.Write(line)
		b.WriteByte('\n')
	}
	path := filepath.Join(dir, "history.jsonl")
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		t.Fatalf("write history: %v", err)
	}
	return path
}

// explainHistoryEntry builds one history record for explain fixtures.
func explainHistoryEntry(ulid, candidate string, finalScore float64, ev Evidence) HistoryEntry {
	return HistoryEntry{
		ULID:          ulid,
		TS:            "2026-08-07T17:03:11Z",
		Profile:       ev.Profile,
		Strategy:      "score",
		CandidateID:   candidate,
		FinalScore:    finalScore,
		ExcludedCount: len(ev.ExcludedCandidates),
		Evidence:      ev,
	}
}

// f26FullEvidence is the shared full (non-degraded) evidence record
// (CONTRACTS §6; TASKS.md T10 test-4 golden).
func f26FullEvidence() Evidence {
	sec := int64(300)
	return Evidence{
		Profile:            "complex_implementation",
		ScoreInputs:        map[string]float64{"tier1": 84, "category": 8},
		Band:               &BandEvidence{Name: "five hour", UsedPercent: 25, Weight: 0.8},
		SnapshotAgeSeconds: &sec,
		Confidence:         "live",
		RouteProvenance:    "provider_live",
		ExcludedCandidates: []ExcludedCandidate{{
			Route: RouteRef{
				Provider: "codex", ModelID: "gpt-5-codex", Model: "gpt-5-codex",
				Reasoning: "default", WindowIDs: []string{},
			},
			ReasonCode: "band_gated",
			Reason:     "band usage 95% > gate 90%",
		}},
		LastVerified: "2026-08-07T17:03:11Z",
	}
}

// explainEvidenceMap returns the decoded JSON form of an evidence record for
// deep comparison against the document's evidence object.
func explainEvidenceMap(t *testing.T, ev Evidence) map[string]any {
	t.Helper()
	data, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal evidence: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal evidence: %v", err)
	}
	return m
}

// F26-T10 row 1: registeredCommands() contains explain.
func TestExplainCommandRegistered(t *testing.T) {
	for _, cmd := range registeredCommands() {
		if cmd.Name() == "explain" {
			return
		}
	}
	t.Fatal("registeredCommands() does not contain explain")
}

// F26-T10 row 1: NewExplainCmd() — exact name, short, and flag defaults.
func TestExplainCommandShape(t *testing.T) {
	cmd := NewExplainCmd()
	if cmd.Use != "explain" {
		t.Fatalf("Use = %q, want explain", cmd.Use)
	}
	if cmd.Short != "Explain a previous pick" {
		t.Errorf("Short = %q, want %q", cmd.Short, "Explain a previous pick")
	}
	routesCheckFlag(t, cmd, "last", "bool", "false")
	routesCheckFlag(t, cmd, "pick-id", "string", "")
}

// F26-T10 row 2: neither nor both selectors is a UsageError (exit 2) with
// the exact message, before any config or history access.
func TestExplainSelectorValidation(t *testing.T) {
	cfg := pickTestConfig(t, t.TempDir(), "")
	cases := []struct {
		name string
		args ExplainArgs
	}{
		{"neither", ExplainArgs{ConfigPath: cfg}},
		{"both", ExplainArgs{Last: true, PickID: "01J2X7K4Q5A8B9C0D1E2F3G4H5", ConfigPath: cfg}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out, errOut strings.Builder
			err := RunExplain(tc.args, &out, &errOut)
			var usage *UsageError
			if !errors.As(err, &usage) {
				t.Fatalf("err = %v, want *UsageError", err)
			}
			if usage.Message != "exactly one of --last or --pick-id is required" {
				t.Errorf("message = %q, want %q", usage.Message, "exactly one of --last or --pick-id is required")
			}
			if got := ExitCodeFor(err); got != 2 {
				t.Errorf("ExitCodeFor(err) = %d, want 2", got)
			}
			if out.Len() != 0 {
				t.Errorf("stdout = %q, want empty", out.String())
			}
		})
	}
}

// F26-T10 row 3: --pick-id selects the record whose ULID matches exactly
// (not the last line); the JSON document is the annex-c §4.3 root —
// schema_version "2.0", candidate echoed back, evidence verbatim (indent 2 +
// trailing newline).
func TestExplainPickIDFound(t *testing.T) {
	cfg := pickTestConfig(t, t.TempDir(), "")
	stateDir := t.TempDir()
	first := explainHistoryEntry("01J2X7K4Q5A8B9C0D1E2F3G4H5", "claude:claude-sonnet-4-5", 92.5, f26FullEvidence())
	second := explainHistoryEntry("01J2X7K4Q5A8B9C0D1E2F3G4H6", "codex:gpt-5-codex", 88, f26FullEvidence())
	seedExplainHistory(t, stateDir, first, second)
	setStateDir(t, func() string { return stateDir })

	var out, errOut strings.Builder
	err := RunExplain(ExplainArgs{PickID: first.ULID, JSON: true, ConfigPath: cfg}, &out, &errOut)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.HasSuffix(out.String(), "\n") {
		t.Error("JSON output must end with a newline")
	}
	doc := pickJSON(t, out.String())
	if doc["schema_version"] != "2.0" {
		t.Errorf("schema_version = %v, want 2.0", doc["schema_version"])
	}
	if got := doc["candidate"]; got != first.CandidateID {
		t.Errorf("candidate = %v, want %q", got, first.CandidateID)
	}
	if got := doc["evidence"]; !reflect.DeepEqual(got, explainEvidenceMap(t, first.Evidence)) {
		t.Errorf("evidence = %v, want the recorded evidence %v", got, first.Evidence)
	}
	if errOut.Len() != 0 {
		t.Errorf("stderr = %q, want empty", errOut.String())
	}
}

// F26-T10 row 3: an unknown ULID is CodedError no_record (exit 1) with
// message "no record <ulid>".
func TestExplainPickIDUnknown(t *testing.T) {
	cfg := pickTestConfig(t, t.TempDir(), "")
	stateDir := t.TempDir()
	seedExplainHistory(t, stateDir, explainHistoryEntry("01J2X7K4Q5A8B9C0D1E2F3G4H5", "claude:claude-sonnet-4-5", 92.5, f26FullEvidence()))
	setStateDir(t, func() string { return stateDir })

	unknown := "01ZZZZZZZZZZZZZZZZZZZZZZZZZZ"
	var out, errOut strings.Builder
	err := RunExplain(ExplainArgs{PickID: unknown, JSON: true, ConfigPath: cfg}, &out, &errOut)
	var coded *CodedError
	if !errors.As(err, &coded) {
		t.Fatalf("err = %v, want *CodedError", err)
	}
	if coded.Code != "no_record" {
		t.Errorf("code = %q, want no_record", coded.Code)
	}
	if want := "no record " + unknown; coded.Message != want {
		t.Errorf("message = %q, want %q", coded.Message, want)
	}
	if got := ExitCodeFor(err); got != 1 {
		t.Errorf("ExitCodeFor(err) = %d, want 1", got)
	}
	if out.Len() != 0 {
		t.Errorf("stdout = %q, want empty", out.String())
	}
}

// F26-T10 selector: --last explains the most recent record (the final line),
// not an earlier one.
func TestExplainLastSelector(t *testing.T) {
	cfg := pickTestConfig(t, t.TempDir(), "")
	stateDir := t.TempDir()
	first := explainHistoryEntry("01J2X7K4Q5A8B9C0D1E2F3G4H5", "claude:claude-sonnet-4-5", 92.5, f26FullEvidence())
	second := explainHistoryEntry("01J2X7K4Q5A8B9C0D1E2F3G4H6", "codex:gpt-5-codex", 88, f26FullEvidence())
	seedExplainHistory(t, stateDir, first, second)
	setStateDir(t, func() string { return stateDir })

	var out, errOut strings.Builder
	err := RunExplain(ExplainArgs{Last: true, JSON: true, ConfigPath: cfg}, &out, &errOut)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	doc := pickJSON(t, out.String())
	if got := doc["candidate"]; got != second.CandidateID {
		t.Errorf("candidate = %v, want %q", got, second.CandidateID)
	}
}

// F26-T10 error: a missing/empty history is CodedError no_record (exit 1)
// for both selectors.
func TestExplainEmptyHistory(t *testing.T) {
	cfg := pickTestConfig(t, t.TempDir(), "")
	stateDir := t.TempDir()
	setStateDir(t, func() string { return stateDir })

	cases := []struct {
		name string
		args ExplainArgs
		want string
	}{
		{"last", ExplainArgs{Last: true, ConfigPath: cfg}, "no record in pick history"},
		{"pick-id", ExplainArgs{PickID: "01J2X7K4Q5A8B9C0D1E2F3G4H5", ConfigPath: cfg}, "no record 01J2X7K4Q5A8B9C0D1E2F3G4H5"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out, errOut strings.Builder
			err := RunExplain(tc.args, &out, &errOut)
			var coded *CodedError
			if !errors.As(err, &coded) {
				t.Fatalf("err = %v, want *CodedError", err)
			}
			if coded.Code != "no_record" || coded.Message != tc.want {
				t.Errorf("err = {code %q, message %q}, want {code %q, message %q}", coded.Code, coded.Message, "no_record", tc.want)
			}
			if got := ExitCodeFor(err); got != 1 {
				t.Errorf("ExitCodeFor(err) = %d, want 1", got)
			}
			if out.Len() != 0 {
				t.Errorf("stdout = %q, want empty", out.String())
			}
		})
	}
}

// F26-T10 error: a config load failure is a UsageError (exit 2) carrying the
// load error text.
func TestExplainConfigError(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.toml")
	var out, errOut strings.Builder
	err := RunExplain(ExplainArgs{Last: true, ConfigPath: missing}, &out, &errOut)
	var usage *UsageError
	if !errors.As(err, &usage) {
		t.Fatalf("err = %v, want *UsageError", err)
	}
	if usage.Message == "" {
		t.Error("message empty, want config load error text")
	}
	if got := ExitCodeFor(err); got != 2 {
		t.Errorf("ExitCodeFor(err) = %d, want 2", got)
	}
}

// F26-T10 row 4: FormatExplainText renders the full record exactly per
// CONTRACTS §7 — header plus confidence/band/route_provenance/excluded/
// last_verified lines.
func TestExplainTextGolden(t *testing.T) {
	entry := explainHistoryEntry("01J2X7K4Q5A8B9C0D1E2F3G4H5", "claude:claude-sonnet-4-5", 92.5, f26FullEvidence())
	want := "" +
		"explain complex_implementation (01J2X7K4Q5A8B9C0D1E2F3G4H5): picked claude:claude-sonnet-4-5 (score 92.5)\n" +
		"  confidence: live\n" +
		"  band: five hour (25% used, weight 0.8)\n" +
		"  route_provenance: provider_live\n" +
		"  excluded: codex:gpt-5-codex (band_gated)\n" +
		"  last_verified: 2026-08-07T17:03:11Z\n"
	if got := FormatExplainText(entry); got != want {
		t.Errorf("FormatExplainText =\n%q\nwant:\n%q", got, want)
	}
}

// F26-T10 row 4: absent degraded fields omit their lines; an empty
// candidate_id renders "-" (no-pick record).
func TestExplainTextDegraded(t *testing.T) {
	ev := Evidence{
		Profile:         "complex_implementation",
		ScoreInputs:     map[string]float64{"tier1": 84, "category": 8},
		RouteProvenance: "models_dev",
	}
	want := "" +
		"explain complex_implementation (01J2X7K4Q5A8B9C0D1E2F3G4H6): picked claude:claude-sonnet-4-5 (score 92.5)\n" +
		"  route_provenance: models_dev\n"
	if got := FormatExplainText(explainHistoryEntry("01J2X7K4Q5A8B9C0D1E2F3G4H6", "claude:claude-sonnet-4-5", 92.5, ev)); got != want {
		t.Errorf("FormatExplainText =\n%q\nwant:\n%q", got, want)
	}

	wantNoPick := "" +
		"explain complex_implementation (01J2X7K4Q5A8B9C0D1E2F3G4H7): picked - (score 0)\n" +
		"  route_provenance: models_dev\n"
	if got := FormatExplainText(explainHistoryEntry("01J2X7K4Q5A8B9C0D1E2F3G4H7", "", 0, ev)); got != wantNoPick {
		t.Errorf("FormatExplainText (no pick) =\n%q\nwant:\n%q", got, wantNoPick)
	}
}

// F26-T10: text mode (JSON false) writes FormatExplainText to stdout.
func TestExplainTextMode(t *testing.T) {
	cfg := pickTestConfig(t, t.TempDir(), "")
	stateDir := t.TempDir()
	entry := explainHistoryEntry("01J2X7K4Q5A8B9C0D1E2F3G4H5", "claude:claude-sonnet-4-5", 92.5, f26FullEvidence())
	seedExplainHistory(t, stateDir, entry)
	setStateDir(t, func() string { return stateDir })

	var out, errOut strings.Builder
	err := RunExplain(ExplainArgs{Last: true, JSON: false, ConfigPath: cfg}, &out, &errOut)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if want := FormatExplainText(entry); out.String() != want {
		t.Errorf("stdout =\n%q\nwant:\n%q", out.String(), want)
	}
	if errOut.Len() != 0 {
		t.Errorf("stderr = %q, want empty", errOut.String())
	}
}

// F26-T10: runExplainE wires --last/--pick-id into RunExplain and renders
// the document on the command's stdout.
func TestExplainCommandRunE(t *testing.T) {
	cfg := pickTestConfig(t, t.TempDir(), "")
	stateDir := t.TempDir()
	entry := explainHistoryEntry("01J2X7K4Q5A8B9C0D1E2F3G4H5", "claude:claude-sonnet-4-5", 92.5, f26FullEvidence())
	seedExplainHistory(t, stateDir, entry)
	setStateDir(t, func() string { return stateDir })

	oldGlobal := Global
	Global = GlobalFlags{ConfigPath: cfg, JSON: true}
	t.Cleanup(func() { Global = oldGlobal })

	cmd := NewExplainCmd()
	var out, errOut strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"--pick-id", entry.ULID})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	doc := pickJSON(t, out.String())
	if got := doc["candidate"]; got != entry.CandidateID {
		t.Errorf("candidate = %v, want %q", got, entry.CandidateID)
	}
}
