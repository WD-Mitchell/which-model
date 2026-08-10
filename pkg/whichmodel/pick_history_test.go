// F26-T9: history log — JSONL append, warning, last/id read
// (specs/features/F26-cmd-pick/TASKS.md T9; SPEC §2.10, §2.11, D-11..D-13;
// CONTRACTS §2, §6).
package whichmodel

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/WD-Mitchell/which-model/internal/config"
	"github.com/WD-Mitchell/which-model/internal/usage"
)

// readHistoryFile parses every line of <stateDir>/pick/history.jsonl into
// HistoryEntry, in file order. A missing file is an empty history (first
// run), not an error — mirroring readHistory. Shared with T4/T10 tests.
func readHistoryFile(t *testing.T, stateDir string) []HistoryEntry {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(stateDir, "pick", "history.jsonl"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatalf("read pick history: %v", err)
	}
	var entries []HistoryEntry
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var e HistoryEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("unmarshal history line %q: %v", line, err)
		}
		entries = append(entries, e)
	}
	return entries
}

// historyRawLine unmarshals history line idx into a raw map so tests can
// assert key presence/absence (degraded mode omits band fields).
func historyRawLine(t *testing.T, stateDir string, idx int) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(stateDir, "pick", "history.jsonl"))
	if err != nil {
		t.Fatalf("read pick history: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if idx >= len(lines) {
		t.Fatalf("history has %d lines, want at least %d", len(lines), idx+1)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(lines[idx]), &m); err != nil {
		t.Fatalf("unmarshal history line %d: %v", idx, err)
	}
	return m
}

// F26-T9 row 1: one successful run appends exactly one JSONL line that
// unmarshals to a full HistoryEntry — 26-char ULID, RFC3339 ts, profile,
// strategy, candidate_id, final_score, excluded_count, and a non-empty
// evidence object. (schema_version "2.0" lives on the document root —
// ExplainResult, annex-c §4.3 — not inside the Evidence object; the explain
// test asserts it there.)
func TestPickHistoryAppend(t *testing.T) {
	cfg, dir := pickPipelineSetup(t, pickTwoRoutes(), pickTwoScores(), nil, nil, nil)

	err, out, _ := runPick(t, PickArgs{Profile: "complex_implementation", Strategy: "priority", ConfigPath: cfg})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	raw := historyRawLine(t, dir, 0)
	if len(raw) == 0 {
		t.Fatal("history line is empty")
	}
	ev, ok := raw["evidence"].(map[string]any)
	if !ok || len(ev) == 0 {
		t.Fatalf("evidence = %v, want non-empty object", raw["evidence"])
	}

	entries := readHistoryFile(t, dir)
	if len(entries) != 1 {
		t.Fatalf("history lines = %d, want 1", len(entries))
	}
	e := entries[0]
	if _, err := ulid.Parse(e.ULID); err != nil {
		t.Errorf("ulid.Parse(%q): %v", e.ULID, err)
	}
	if len(e.ULID) != 26 {
		t.Errorf("ulid = %q (%d chars), want 26", e.ULID, len(e.ULID))
	}
	if _, err := time.Parse(time.RFC3339, e.TS); err != nil {
		t.Errorf("ts %q not RFC3339: %v", e.TS, err)
	}
	if e.Profile != "complex_implementation" {
		t.Errorf("profile = %q, want complex_implementation", e.Profile)
	}
	if e.Strategy != "priority" {
		t.Errorf("strategy = %q, want priority", e.Strategy)
	}
	if e.CandidateID != "claude:claude-sonnet-4-5" {
		t.Errorf("candidate_id = %q, want claude:claude-sonnet-4-5", e.CandidateID)
	}
	if e.FinalScore != 92.0 {
		t.Errorf("final_score = %v, want 92", e.FinalScore)
	}
	if e.ExcludedCount != 0 {
		t.Errorf("excluded_count = %d, want 0", e.ExcludedCount)
	}
	if e.Evidence.Profile != "complex_implementation" {
		t.Errorf("evidence.profile = %q, want complex_implementation", e.Evidence.Profile)
	}
	if len(e.Evidence.ScoreInputs) == 0 {
		t.Error("evidence.score_inputs is empty, want tier1/category inputs")
	}
	first := pickJSON(t, out.String())["candidates"].([]any)[0].(map[string]any)
	if got := first["candidate_id"]; got != e.CandidateID {
		t.Errorf("pick candidate_id = %v, history candidate_id = %q", got, e.CandidateID)
	}
}

// F26-T9 row 2: append-only — two runs produce two lines and the first
// line is byte-identical to the first run's line.
func TestPickHistoryAppendOnly(t *testing.T) {
	cfg, dir := pickPipelineSetup(t, pickTwoRoutes(), pickTwoScores(), nil, nil, nil)
	path := filepath.Join(dir, "pick", "history.jsonl")

	err, _, _ := runPick(t, PickArgs{Profile: "complex_implementation", Strategy: "priority", ConfigPath: cfg})
	if err != nil {
		t.Fatalf("run 1 err = %v", err)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read history after run 1: %v", err)
	}
	firstLine := strings.TrimSpace(string(first))

	err, _, _ = runPick(t, PickArgs{Profile: "complex_implementation", Strategy: "priority", ConfigPath: cfg})
	if err != nil {
		t.Fatalf("run 2 err = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read history after run 2: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("history lines = %d, want 2", len(lines))
	}
	if lines[0] != firstLine {
		t.Errorf("line 1 changed between runs:\n got %q\nwant %q", lines[0], firstLine)
	}
	if lines[1] == lines[0] {
		t.Error("line 2 is byte-identical to line 1; want a distinct run record (new ulid/ts)")
	}
}

// F26-T9 row 3: an unwritable history path (state dir is a file) yields
// the stderr warning `warning: could not write pick history: <err>` and
// the pick still succeeds — history must never fail a run (D-12).
func TestPickHistoryWriteFailure(t *testing.T) {
	cfg, _ := pickPipelineSetup(t, pickTwoRoutes(), pickTwoScores(), nil, nil, nil)
	stateFile := filepath.Join(t.TempDir(), "state-file")
	if err := os.WriteFile(stateFile, []byte("occupied"), 0o600); err != nil {
		t.Fatalf("write state file: %v", err)
	}
	setStateDir(t, func() string { return stateFile })

	err, out, errOut := runPick(t, PickArgs{Profile: "complex_implementation", Strategy: "priority", ConfigPath: cfg})
	if err != nil {
		t.Fatalf("err = %v, want nil (history failure must not fail the pick)", err)
	}
	if ExitCodeFor(err) != 0 {
		t.Errorf("exit code = %d, want 0", ExitCodeFor(err))
	}
	if !strings.Contains(errOut.String(), "warning: could not write pick history:") {
		t.Errorf("stderr = %q, want history write warning", errOut.String())
	}
	if got := pickJSON(t, out.String())["schema_version"]; got != "2.0" {
		t.Errorf("pick schema_version = %v, want 2.0", got)
	}
}

// F26-T9 row 4: evidence content for the T8 fixture (claude + codex,
// codex band-gated, usage enabled) — full annex-c §4.3 record — and the
// degraded-mode record with band/snapshot_age_seconds/confidence/
// last_verified absent and route_provenance never provider_live (§5.1).
func TestPickHistoryEvidence(t *testing.T) {
	verifiedAt := time.Date(2026, 8, 7, 17, 3, 11, 0, time.UTC)

	t.Run("live", func(t *testing.T) {
		cfg, dir := pickPipelineSetup(t, pickTwoRoutes(), pickTwoScores(), nil,
			func(_ context.Context, _ []string, _ pickFetchOptions) (map[string]*usageSnapshot, map[string]timeValue, error) {
				fetchedAt := time.Now().Add(-2 * time.Minute)
				return map[string]*usageSnapshot{
					"claude": {Provider: "claude", FetchedAt: fetchedAt, UsageKnown: true},
					"codex":  {Provider: "codex", FetchedAt: fetchedAt, UsageKnown: true},
				}, map[string]timeValue{"claude": verifiedAt}, nil
			},
			func(snap *usage.Snapshot, _ string, _ *config.Config) (bandResult, error) {
				if snap.Provider == "codex" {
					return bandResult{Name: "five hour", UsedPercent: 95, Gated: true}, nil
				}
				return bandResult{Name: "five hour", UsedPercent: 25, Weight: 0.8}, nil
			})

		err, _, _ := runPick(t, PickArgs{Profile: "complex_implementation", Strategy: "priority", ConfigPath: cfg})
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		e := readHistoryFile(t, dir)[0]
		if e.CandidateID != "claude:claude-sonnet-4-5" {
			t.Errorf("candidate_id = %q, want claude:claude-sonnet-4-5", e.CandidateID)
		}
		if e.FinalScore != 73.6 {
			t.Errorf("final_score = %v, want 73.6 (92 * 0.8 * 1.0)", e.FinalScore)
		}
		if e.ExcludedCount != 1 {
			t.Errorf("excluded_count = %d, want 1", e.ExcludedCount)
		}
		ev := e.Evidence
		if ev.Profile != "complex_implementation" {
			t.Errorf("evidence.profile = %q, want complex_implementation", ev.Profile)
		}
		if ev.ScoreInputs["tier1"] != 92.0 || ev.ScoreInputs["category"] != 0.0 {
			t.Errorf("score_inputs = %v, want {tier1: 92, category: 0}", ev.ScoreInputs)
		}
		if ev.Band == nil || ev.Band.Name != "five hour" || ev.Band.UsedPercent != 25 || ev.Band.Weight != 0.8 {
			t.Errorf("band = %+v, want five hour 25%% weight 0.8", ev.Band)
		}
		if ev.SnapshotAgeSeconds == nil || *ev.SnapshotAgeSeconds <= 0 {
			t.Errorf("snapshot_age_seconds = %v, want > 0", ev.SnapshotAgeSeconds)
		}
		if ev.Confidence != "live" {
			t.Errorf("confidence = %q, want live", ev.Confidence)
		}
		if ev.RouteProvenance != "provider_live" {
			t.Errorf("route_provenance = %q, want provider_live", ev.RouteProvenance)
		}
		if len(ev.ExcludedCandidates) != 1 || ev.ExcludedCandidates[0].ReasonCode != "band_gated" {
			t.Errorf("excluded_candidates = %+v, want [band_gated]", ev.ExcludedCandidates)
		}
		if got := ev.ExcludedCandidates[0].Route.ModelID; got != "gpt-5-codex" {
			t.Errorf("excluded route model_id = %q, want gpt-5-codex", got)
		}
		if ev.LastVerified != verifiedAt.UTC().Format(time.RFC3339) {
			t.Errorf("last_verified = %q, want %q", ev.LastVerified, verifiedAt.UTC().Format(time.RFC3339))
		}
	})

	t.Run("degraded", func(t *testing.T) {
		cfg, dir := pickPipelineSetup(t, pickTwoRoutes(), pickTwoScores(),
			func(_ bool, _ *config.Config) (bool, string) { return false, "flag" }, nil, nil)

		err, _, _ := runPick(t, PickArgs{Profile: "complex_implementation", Strategy: "priority", ConfigPath: cfg})
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		raw := historyRawLine(t, dir, 0)
		ev := raw["evidence"].(map[string]any)
		for _, absent := range []string{"band", "snapshot_age_seconds", "confidence", "last_verified"} {
			if _, ok := ev[absent]; ok {
				t.Errorf("degraded evidence carries %q key: %v", absent, ev[absent])
			}
		}
		for _, present := range []string{"score_inputs", "route_provenance", "excluded_candidates"} {
			if _, ok := ev[present]; !ok {
				t.Errorf("degraded evidence missing %q key", present)
			}
		}
		e := readHistoryFile(t, dir)[0]
		dev := e.Evidence
		if dev.RouteProvenance != "models_dev" {
			t.Errorf("route_provenance = %q, want models_dev (provider_live mapped in degraded mode)", dev.RouteProvenance)
		}
		if dev.ScoreInputs["tier1"] != 92.0 {
			t.Errorf("score_inputs = %v, want tier1 92", dev.ScoreInputs)
		}
		if dev.Band != nil || dev.SnapshotAgeSeconds != nil || dev.Confidence != "" || dev.LastVerified != "" {
			t.Errorf("degraded evidence carries live-only fields: %+v", dev)
		}
	})
}

// F26-T9 row 5: RunExplain(Last: true) echoes the recorded candidate and
// reproduces the history line's evidence exactly; an empty/missing history
// is CodedError no_record with exit 1.
func TestPickHistoryExplainLast(t *testing.T) {
	t.Run("last", func(t *testing.T) {
		cfg, dir := pickPipelineSetup(t, pickTwoRoutes(), pickTwoScores(), nil, nil, nil)

		err, _, _ := runPick(t, PickArgs{Profile: "complex_implementation", Strategy: "priority", ConfigPath: cfg})
		if err != nil {
			t.Fatalf("pick err = %v", err)
		}
		entry := readHistoryFile(t, dir)[0]

		var out, errOut strings.Builder
		err = RunExplain(ExplainArgs{Last: true, JSON: true, ConfigPath: cfg}, &out, &errOut)
		if err != nil {
			t.Fatalf("RunExplain err = %v", err)
		}
		root := pickJSON(t, out.String())
		if root["schema_version"] != "2.0" {
			t.Errorf("schema_version = %v, want 2.0", root["schema_version"])
		}
		if root["candidate"] != entry.CandidateID {
			t.Errorf("candidate = %v, want %q", root["candidate"], entry.CandidateID)
		}
		var res ExplainResult
		if err := json.Unmarshal([]byte(out.String()), &res); err != nil {
			t.Fatalf("unmarshal explain JSON: %v", err)
		}
		if res.SchemaVersion != "2.0" {
			t.Errorf("ExplainResult.SchemaVersion = %q, want 2.0", res.SchemaVersion)
		}
		if res.Candidate != entry.CandidateID {
			t.Errorf("ExplainResult.Candidate = %q, want %q", res.Candidate, entry.CandidateID)
		}
		if !reflect.DeepEqual(res.Evidence, entry.Evidence) {
			t.Errorf("explain evidence != history evidence:\n got %+v\nwant %+v", res.Evidence, entry.Evidence)
		}
		if errOut.Len() != 0 {
			t.Errorf("stderr = %q, want empty", errOut.String())
		}
	})

	t.Run("empty history", func(t *testing.T) {
		cfg := pickTestConfig(t, t.TempDir(), "")
		setStateDir(t, func() string { return t.TempDir() })

		var out, errOut strings.Builder
		err := RunExplain(ExplainArgs{Last: true, JSON: true, ConfigPath: cfg}, &out, &errOut)
		var ce *CodedError
		if !errors.As(err, &ce) {
			t.Fatalf("err = %v, want *CodedError", err)
		}
		if ce.Code != "no_record" {
			t.Errorf("code = %q, want no_record", ce.Code)
		}
		if ce.Message != "no record in pick history" {
			t.Errorf("message = %q, want %q", ce.Message, "no record in pick history")
		}
		if ExitCodeFor(err) != 1 {
			t.Errorf("exit code = %d, want 1", ExitCodeFor(err))
		}
		if out.Len() != 0 {
			t.Errorf("stdout = %q, want empty", out.String())
		}
	})
}
