package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/WD-Mitchell/which-model/internal/routing"
)

// fixtureScoresCSV is the B04 CONTRACTS §5 scores fixture. ParseScoresCSV
// requires ALL six tier-1 columns plus model/reasoning; the three extra
// mandatory columns (time, coding, agentic) are set to in-range constants
// that do not feed the 3 ranking axes (pick.Tier1ScoreColumn). The category
// column carries the tier-2 evidence used by the golden ranking.
const fixtureScoresCSV = "model,reasoning,intelligence_index_score,time_per_intelligence_index_task_seconds_score,cost_per_intelligence_index_task_usd_score,median_end_to_end_response_time_seconds_score,artificial_analysis_coding_index_score,artificial_analysis_agentic_index_score,software_engineering_score\n" +
	"alpha,high,90,0,60,70,100,100,80\n" +
	"alpha,low,70,0,90,90,100,100,60\n" +
	"beta,high,85,0,70,75,100,100,\n" +
	"gamma,medium,90,0,60,70,100,100,80\n"

// fixtureConfigTOML is the B04 CONTRACTS §5 config: providers claude
// (priority 1) and codex (priority 2), a custom profile with CoreShare 60,
// tier1 {intelligence:4,cost:3,speed:3}, tier2 {software_engineering:5},
// and [gui].holds 5.
const fixtureConfigTOML = `[providers.claude]
enabled = true
priority = 1

[providers.codex]
enabled = true
priority = 2

[profiles.test_profile]
core_share = 60
tier1 = {intelligence = 4, cost = 3, speed = 3}
tier2 = {software_engineering = 5}

[gui]
holds = 5
`

// fixtureRoutes is the B04 CONTRACTS §5 routes table: providers claude and
// codex, both enabled, covering every catalog identity in fixtureScoresCSV.
func fixtureRoutes() routing.Table {
	lu := routing.ProvenanceUserDeclared
	return routing.Table{
		SchemaVersion: routing.TableSchemaVersion,
		Routes: []routing.Route{
			{Provider: "claude", ModelID: "alpha-1", Model: "alpha", Reasoning: "high", Provenance: lu},
			{Provider: "claude", ModelID: "alpha-1", Model: "alpha", Reasoning: "low", Provenance: lu},
			{Provider: "claude", ModelID: "beta-1", Model: "beta", Reasoning: "high", Provenance: lu},
			{Provider: "codex", ModelID: "alpha-1x", Model: "alpha", Reasoning: "high", Provenance: lu},
			{Provider: "codex", ModelID: "gamma-1", Model: "gamma", Reasoning: "medium", Provenance: lu},
		},
	}
}
func fptr(f float64) *float64 { return &f }

// fixtureServices builds a Services with the B04 §5 fixture tree.
func fixtureServices(t *testing.T, opts ...TestOption) (*Services, *emitRecorder) {
	t.Helper()
	base := []TestOption{
		WithConfigTOML(fixtureConfigTOML),
		WithScoresCSV(fixtureScoresCSV),
		WithRoutes(fixtureRoutes()),
	}
	return newTestServices(t, append(base, opts...)...)
}

// TestPickRankGolden asserts the deterministic golden ranking (CONTRACTS §5
// test 1): exact order, scores, provider priority resolution, and tie-break.
func TestPickRankGolden(t *testing.T) {
	svc, rec := fixtureServices(t)

	first, err := svc.Rank(context.Background(), RankRequest{ProfileSlug: "test_profile", Holds: 5})
	if err != nil {
		t.Fatalf("Rank: %v", err)
	}
	want := RankResponse{
		Total: 4,
		Candidates: []RankedModel{
			{Rank: 1, ModelID: "beta-1", ModelName: "beta", Provider: "claude", Reasoning: "high", Score: 77.50, RouteKey: "claude/beta-1@high", Intelligence: fptr(85), Cost: fptr(70), Speed: fptr(75)},
			{Rank: 2, ModelID: "alpha-1", ModelName: "alpha", Provider: "claude", Reasoning: "high", Score: 77.00, RouteKey: "claude/alpha-1@high", Intelligence: fptr(90), Cost: fptr(60), Speed: fptr(70)},
			{Rank: 3, ModelID: "gamma-1", ModelName: "gamma", Provider: "codex", Reasoning: "medium", Score: 77.00, RouteKey: "codex/gamma-1@medium", Intelligence: fptr(90), Cost: fptr(60), Speed: fptr(70)},
			{Rank: 4, ModelID: "alpha-1", ModelName: "alpha", Provider: "claude", Reasoning: "low", Score: 73.20, RouteKey: "claude/alpha-1@low", Intelligence: fptr(70), Cost: fptr(90), Speed: fptr(90)},
		},
	}
	if !reflect.DeepEqual(first, want) {
		t.Errorf("Rank = %#v\nwant %#v", first, want)
	}
	// SPEC §2.9 determinism: identical request => byte-identical response.
	second, err := svc.Rank(context.Background(), RankRequest{ProfileSlug: "test_profile", Holds: 5})
	if err != nil {
		t.Fatalf("Rank (2nd): %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Errorf("Rank not deterministic:\nfirst  %#v\nsecond %#v", first, second)
	}
	if len(rec.Events()) != 0 {
		t.Errorf("Rank emitted events: %v (want none)", rec.Events())
	}
}

// TestPickRankDisabledRoute tests CONTRACTS §5 test 2: disabling claude's
// alpha@high route re-resolves alpha@high to codex; disabling both providers'
// alpha@high routes drops alpha@high entirely and shrinks Total.
func TestPickRankDisabledRoute(t *testing.T) {
	disabled := `[routes.disabled]
claude = ["alpha-1@high"]
`
	t.Run("codex fallback", func(t *testing.T) {
		svc, _ := fixtureServices(t, WithConfigTOML(fixtureConfigTOML+disabled))
		res, err := svc.Rank(context.Background(), RankRequest{ProfileSlug: "test_profile", Holds: 5})
		if err != nil {
			t.Fatalf("Rank: %v", err)
		}
		if res.Total != 4 {
			t.Fatalf("Total = %d, want 4", res.Total)
		}
		found := false
		for _, c := range res.Candidates {
			if c.ModelName == "alpha" && c.Reasoning == "high" {
				found = true
				if c.Provider != "codex" || c.RouteKey != "codex/alpha-1x@high" {
					t.Errorf("alpha@high -> provider %q route %q, want codex codex/alpha-1x@high", c.Provider, c.RouteKey)
				}
			}
		}
		if !found {
			t.Error("alpha@high not ranked after claude route disabled")
		}
	})

	t.Run("both disabled", func(t *testing.T) {
		both := disabled + "codex = [\"alpha-1x@high\"]\n"
		svc, _ := fixtureServices(t, WithConfigTOML(fixtureConfigTOML+both))
		res, err := svc.Rank(context.Background(), RankRequest{ProfileSlug: "test_profile", Holds: 5})
		if err != nil {
			t.Fatalf("Rank: %v", err)
		}
		if res.Total != 3 {
			t.Fatalf("Total = %d, want 3", res.Total)
		}
		for _, c := range res.Candidates {
			if c.ModelName == "alpha" && c.Reasoning == "high" {
				t.Errorf("alpha@high still ranked after both routes disabled: %#v", c)
			}
		}
	})
}

// TestPickRankEmptyAvailability asserts CONTRACTS §5 test 3: all providers
// disabled => RankResponse{Total: 0}, nil error.
func TestPickRankEmptyAvailability(t *testing.T) {
	cfg := `[providers.claude]
enabled = false
priority = 1

[providers.codex]
enabled = false
priority = 2

[profiles.test_profile]
core_share = 60
tier1 = {intelligence = 4, cost = 3, speed = 3}
tier2 = {software_engineering = 5}

[gui]
holds = 5
`
	svc, rec := fixtureServices(t, WithConfigTOML(cfg))
	res, err := svc.Rank(context.Background(), RankRequest{ProfileSlug: "test_profile", Holds: 5})
	if err != nil {
		t.Fatalf("Rank: %v", err)
	}
	if res.Total != 0 || len(res.Candidates) != 0 {
		t.Errorf("Rank = %#v, want empty candidates Total 0", res)
	}
	if len(rec.Events()) != 0 {
		t.Errorf("emitted events: %v", rec.Events())
	}
}

// TestPickRankOverrides asserts CONTRACTS §5 test 4: overrides rank without
// persisting config, history, or events; invalid overrides fail validation in
// SPEC §2.1 order.
func TestPickRankOverrides(t *testing.T) {
	good := ProfileDetail{
		CoreShare:    60,
		Tier1Weights: map[string]int{"intelligence": 4, "cost": 3, "speed": 3},
		Tier2Weights: map[string]int{"software_engineering": 5},
	}
	svc, rec := fixtureServices(t)

	// Config bytes before the call (no overrides persisted at all).
	cfgBefore := readConfigBytes(t, svc)
	histPath := filepath.Join(svc.paths.StateDir, "pick", "history.jsonl")

	res, err := svc.Rank(context.Background(), RankRequest{Overrides: &good, Holds: 5})
	if err != nil {
		t.Fatalf("Rank (overrides): %v", err)
	}
	if res.Total != 4 {
		t.Fatalf("overrides Total = %d, want 4", res.Total)
	}
	cfgAfter := readConfigBytes(t, svc)
	if !reflect.DeepEqual(cfgBefore, cfgAfter) {
		t.Error("config.toml changed after overrides ranking")
	}
	if _, err := os.Stat(histPath); !os.IsNotExist(err) {
		t.Error("history.jsonl created by ephemeral ranking")
	}
	if len(rec.Events()) != 0 {
		t.Errorf("events after overrides ranking: %v", rec.Events())
	}

	// Invalid overrides hit SPEC §2.1 checks in order.
	badShare := good
	badShare.CoreShare = 57
	if _, err := svc.Rank(context.Background(), RankRequest{Overrides: &badShare, Holds: 5}); !errors.Is(err, errValidation) {
		t.Errorf("CoreShare 57 err = %v, want errValidation", err)
	}
	badWeight := good
	badWeight.Tier1Weights = map[string]int{"intelligence": 6, "cost": 3, "speed": 3}
	if _, err := svc.Rank(context.Background(), RankRequest{Overrides: &badWeight, Holds: 5}); !errors.Is(err, errValidation) {
		t.Errorf("weight 6 err = %v, want errValidation", err)
	}
	badTier2 := good
	badTier2.Tier2Weights = map[string]int{"bogus": 5}
	if _, err := svc.Rank(context.Background(), RankRequest{Overrides: &badTier2, Holds: 5}); !errors.Is(err, errValidation) {
		t.Errorf("tier2 bogus err = %v, want errValidation", err)
	}
	if len(rec.Events()) != 0 {
		t.Errorf("events after invalid overrides: %v", rec.Events())
	}
}

// TestPickRankUnknownProfile asserts an unknown ProfileSlug resolves to
// not_found.
func TestPickRankUnknownProfile(t *testing.T) {
	svc, _ := fixtureServices(t)
	_, err := svc.Rank(context.Background(), RankRequest{ProfileSlug: "nope", Holds: 5})
	if !errors.Is(err, errNotFound) {
		t.Fatalf("err = %v, want errNotFound", err)
	}
	if code := toErrorDTO(err).Code; code != "not_found" {
		t.Fatalf("code = %q, want not_found", code)
	}
}

// TestPickRankHolds asserts CONTRACTS §5 test 5: Holds 0 uses [gui].holds,
// Holds 3 truncates with Total 4, Holds 4 => validation_failed.
func TestPickRankHolds(t *testing.T) {
	svc, _ := fixtureServices(t)
	// Holds 0 => [gui].holds = 5, no truncation (Total 4 < 5).
	res, err := svc.Rank(context.Background(), RankRequest{ProfileSlug: "test_profile", Holds: 0})
	if err != nil {
		t.Fatalf("Rank (holds 0): %v", err)
	}
	if len(res.Candidates) != 4 || res.Total != 4 {
		t.Errorf("holds 0 => %d/%d, want 4/4", len(res.Candidates), res.Total)
	}
	res, err = svc.Rank(context.Background(), RankRequest{ProfileSlug: "test_profile", Holds: 3})
	if err != nil {
		t.Fatalf("Rank (holds 3): %v", err)
	}
	if len(res.Candidates) != 3 || res.Total != 4 {
		t.Errorf("holds 3 => %d/%d, want 3/4", len(res.Candidates), res.Total)
	}
	if _, err := svc.Rank(context.Background(), RankRequest{ProfileSlug: "test_profile", Holds: 4}); !errors.Is(err, errValidation) {
		t.Errorf("holds 4 err = %v, want errValidation", err)
	}
}

// TestPickRecordPick asserts CONTRACTS §5 test 6: two calls append two lines
// with the §4 field names and emit one pick:recorded each; validation and
// write failures behave per SPEC §3.
func TestPickRecordPick(t *testing.T) {
	svc, rec := fixtureServices(t)
	histPath := filepath.Join(svc.paths.StateDir, "pick", "history.jsonl")

	if err := svc.RecordPick(context.Background(), "test_profile", "claude/beta-1@high"); err != nil {
		t.Fatalf("RecordPick 1: %v", err)
	}
	if err := svc.RecordPick(context.Background(), "test_profile", "codex/gamma-1@medium"); err != nil {
		t.Fatalf("RecordPick 2: %v", err)
	}
	data, err := os.ReadFile(histPath)
	if err != nil {
		t.Fatalf("read history: %v", err)
	}
	lines := splitLines(string(data))
	if len(lines) != 2 {
		t.Fatalf("history has %d lines, want 2:\n%s", len(lines), data)
	}
	for i, wantCandidate := range []string{"claude/beta-1@high", "codex/gamma-1@medium"} {
		var entry struct {
			ULID        string          `json:"ulid"`
			TS          string          `json:"ts"`
			Profile     string          `json:"profile"`
			Strategy    string          `json:"strategy"`
			CandidateID string          `json:"candidate_id"`
			FinalScore  float64         `json:"final_score"`
			Excluded    int             `json:"excluded_count"`
			Evidence    json.RawMessage `json:"evidence"`
		}
		if err := json.Unmarshal([]byte(lines[i]), &entry); err != nil {
			t.Fatalf("decode line %d: %v", i, err)
		}
		if len(entry.ULID) != 26 {
			t.Errorf("line %d ulid = %q (%d chars), want 26", i, entry.ULID, len(entry.ULID))
		}
		if _, err := time.Parse(time.RFC3339, entry.TS); err != nil {
			t.Errorf("line %d ts %q not RFC3339: %v", i, entry.TS, err)
		}
		if entry.Profile != "test_profile" || entry.Strategy != "gui" || entry.CandidateID != wantCandidate {
			t.Errorf("line %d = %+v, want test_profile/gui/%s", i, entry, wantCandidate)
		}
		if entry.FinalScore != 0 || entry.Excluded != 0 {
			t.Errorf("line %d final_score/excluded = %v/%d, want 0/0", i, entry.FinalScore, entry.Excluded)
		}
		var ev map[string]json.RawMessage
		if err := json.Unmarshal(entry.Evidence, &ev); err != nil {
			t.Fatalf("decode evidence line %d: %v", i, err)
		}
		for _, field := range []string{"profile", "score_inputs", "route_provenance", "excluded_candidates"} {
			if _, ok := ev[field]; !ok {
				t.Errorf("line %d evidence missing %q", i, field)
			}
		}
	}

	events := rec.Events()
	if len(events) != 2 {
		t.Fatalf("events = %d, want 2: %v", len(events), events)
	}
	for i, wantRoute := range []string{"claude/beta-1@high", "codex/gamma-1@medium"} {
		if events[i].Event != EventPickRecorded {
			t.Errorf("event %d name = %q, want %q", i, events[i].Event, EventPickRecorded)
		}
		payload := events[i].Payload.(map[string]string)
		if payload["profile_slug"] != "test_profile" || payload["route_key"] != wantRoute {
			t.Errorf("event %d payload = %v", i, payload)
		}
	}
}

// TestPickRecordPickBadInputs asserts bad route grammar -> validation_failed
// with zero lines/events, unknown profile -> not_found, and unwritable state
// dir -> io_error with zero events (CONTRACTS §5 test 6).
func TestPickRecordPickBadInputs(t *testing.T) {
	svc, rec := fixtureServices(t)
	histPath := filepath.Join(svc.paths.StateDir, "pick", "history.jsonl")

	for _, key := range []string{"claude/x", "a@b"} {
		if _, _, _, err := ParseRouteKey(key); err == nil {
			t.Fatalf("fixture route key %q parsed; want invalid", key)
		}
		if err := svc.RecordPick(context.Background(), "test_profile", key); !errors.Is(err, errValidation) {
			t.Errorf("RecordPick(%q) err = %v, want errValidation", key, err)
		}
	}
	if _, err := os.Stat(histPath); !os.IsNotExist(err) {
		t.Error("history.jsonl created on validation failure")
	}
	if err := svc.RecordPick(context.Background(), "unknown_profile", "claude/beta-1@high"); !errors.Is(err, errNotFound) {
		t.Errorf("unknown profile err = %v, want errNotFound", err)
	}
	if len(rec.Events()) != 0 {
		t.Errorf("events after bad inputs: %v", rec.Events())
	}

	// Unwritable state dir: point StateDir at a path where pick/ cannot be
	// created as a directory (a regular file occupies the parent's child).
	if err := os.MkdirAll(svc.paths.StateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(svc.paths.StateDir, "pick"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := svc.RecordPick(context.Background(), "test_profile", "claude/beta-1@high"); err == nil {
		t.Error("RecordPick with unwritable state dir returned nil error")
	} else if code := toErrorDTO(err).Code; code != "io_error" {
		t.Errorf("unwritable state dir code = %q, want io_error", code)
	}
	if len(rec.Events()) != 0 {
		t.Errorf("unwritable events: %v", rec.Events())
	}
}

// TestPickCatalogLine asserts CONTRACTS §5 test 7: fixture gives
// {Models: 4, ProvidersOn: 2, Harnesses: 4}; disabling codex => 1;
// config bytes unchanged (read-only).
func TestPickCatalogLine(t *testing.T) {
	svc, rec := fixtureServices(t)
	cfgBefore := readConfigBytes(t, svc)

	line, err := svc.CatalogLine(context.Background())
	if err != nil {
		t.Fatalf("CatalogLine: %v", err)
	}
	want := CatalogSummary{Models: 4, ProvidersOn: 2, Harnesses: 18}
	if line != want {
		t.Errorf("CatalogLine = %v, want %v", line, want)
	}
	if !reflect.DeepEqual(cfgBefore, readConfigBytes(t, svc)) {
		t.Error("config.toml changed by CatalogLine")
	}
	if len(rec.Events()) != 0 {
		t.Errorf("CatalogLine emitted events: %v", rec.Events())
	}

	// Disable codex => ProvidersOn 1.
	cfg := `[providers.claude]
enabled = true
priority = 1

[providers.codex]
enabled = false
priority = 2
`
	svc2, _ := fixtureServices(t, WithConfigTOML(cfg))
	line2, err := svc2.CatalogLine(context.Background())
	if err != nil {
		t.Fatalf("CatalogLine (codex off): %v", err)
	}
	if line2.ProvidersOn != 1 {
		t.Errorf("ProvidersOn = %d, want 1", line2.ProvidersOn)
	}
}

func readConfigBytes(t *testing.T, svc *Services) []byte {
	t.Helper()
	b, err := os.ReadFile(svc.paths.UserConfigFile)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	return b
}

func splitLines(s string) []string {
	s = trimTrailingNewline(s)
	if s == "" {
		return nil
	}
	out := []string{}
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

func trimTrailingNewline(s string) string {
	if len(s) > 0 && s[len(s)-1] == '\n' {
		return s[:len(s)-1]
	}
	return s
}
