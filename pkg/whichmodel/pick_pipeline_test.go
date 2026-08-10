// F26-T3: pipeline core — score, filter, rank, routes join
// (specs/features/F26-cmd-pick/TASKS.md T3; SPEC §2.2a–d, §3).
package whichmodel

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/WD-Mitchell/which-model/internal/config"
	"github.com/WD-Mitchell/which-model/internal/routing"
	"github.com/WD-Mitchell/which-model/internal/usage"
)

// pickTestConfig writes a temp config.toml (body may be empty) and returns
// its path. Fixture convention shared by F26-T3..T10.
func pickTestConfig(t *testing.T, dir, body string) string {
	t.Helper()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

// pickScoresConfigBody returns a minimal config body pointing
// catalog.scores_csv_path at path (plus optional provider lines).
func pickScoresConfigBody(scoresPath, providers string) string {
	return "[catalog]\nscores_csv_path = \"" + scoresPath + "\"\n" + providers
}

// f26ClaudeRoute is the shared claude route fixture (provider_live).
func f26ClaudeRoute() routing.Route {
	return routing.Route{
		Provider:   "claude",
		ModelID:    "claude-sonnet-4-5",
		Model:      "claude-sonnet-4-5",
		Reasoning:  "default",
		WindowIDs:  []string{"5h", "7d"},
		Provenance: routing.ProvenanceProviderLive,
	}
}

// f26CodexRoute is the shared codex route fixture (provider_live).
func f26CodexRoute() routing.Route {
	return routing.Route{
		Provider:   "codex",
		ModelID:    "gpt-5-codex",
		Model:      "gpt-5-codex",
		Reasoning:  "default",
		WindowIDs:  []string{"5h", "7d"},
		Provenance: routing.ProvenanceProviderLive,
	}
}

// setScoreFunc swaps the F10 scoring seam and restores it on cleanup
// (SPEC §2.2a; CONTRACTS §8.7).
func setScoreFunc(t *testing.T, fn func(profile, model, reasoning string) (decimal.Decimal, bool, map[string]float64)) {
	t.Helper()
	old := scoreFunc
	scoreFunc = fn
	t.Cleanup(func() { scoreFunc = old })
}

// setStateDir swaps the state-directory seam (history lives under
// <state_dir>/pick; T9).
func setStateDir(t *testing.T, fn func() string) {
	t.Helper()
	old := stateDirFunc
	stateDirFunc = fn
	t.Cleanup(func() { stateDirFunc = old })
}

// setPickFetchAll swaps the F14 fetch seam (T4; CONTRACTS §8.3).
func setPickFetchAll(t *testing.T, fn func(ctx context.Context, providers []string, opts pickFetchOptions) (map[string]*usageSnapshot, map[string]timeValue, error)) {
	t.Helper()
	old := pickFetchAllFunc
	pickFetchAllFunc = fn
	t.Cleanup(func() { pickFetchAllFunc = old })
}

// setBandEvaluate swaps the F19 band seam (T4; CONTRACTS §8.4).
func setBandEvaluate(t *testing.T, fn func(snap *usage.Snapshot, route string, cfg *config.Config) (bandResult, error)) {
	t.Helper()
	old := bandEvaluateFunc
	bandEvaluateFunc = fn
	t.Cleanup(func() { bandEvaluateFunc = old })
}

func setSuccessfulUsage(t *testing.T) {
	t.Helper()
	setPickFetchAll(t, func(_ context.Context, providers []string, _ pickFetchOptions) (map[string]*usageSnapshot, map[string]timeValue, error) {
		snapshots := make(map[string]*usageSnapshot, len(providers))
		for _, provider := range providers {
			snapshots[provider] = &usage.Snapshot{Provider: provider}
		}
		return snapshots, map[string]timeValue{}, nil
	})
	setBandEvaluate(t, func(_ *usage.Snapshot, _ string, _ *config.Config) (bandResult, error) {
		return bandResult{Name: "available", Weight: 1}, nil
	})
}

// setStrategyApply swaps the F20 strategy seam (T6; CONTRACTS §8.5).
func setStrategyApply(t *testing.T, fn func(name string, cands []Candidate, opts strategyOptions) ([]Candidate, error)) {
	t.Helper()
	old := strategyApplyFunc
	strategyApplyFunc = fn
	t.Cleanup(func() { strategyApplyFunc = old })
}

// pickFakeScores is the default T3 fake: every routed model scores via the
// overrides map; absent models return ok=false.
func pickFakeScores(overrides map[string]decimal.Decimal) func(string, string, string) (decimal.Decimal, bool, map[string]float64) {
	return func(profile, model, reasoning string) (decimal.Decimal, bool, map[string]float64) {
		if v, ok := overrides[model]; ok {
			return v, true, map[string]float64{"tier1": v.InexactFloat64(), "category": 0}
		}
		return decimal.Zero, false, nil
	}
}

// runPick runs RunPick with JSON forced on, returning err, stdout, stderr.
func runPick(t *testing.T, args PickArgs) (err error, out, errOut *strings.Builder) {
	t.Helper()
	args.JSON = true
	var o, e strings.Builder
	return RunPick(args, &o, &e), &o, &e
}

// pickJSON decodes a pick document for assertions.
func pickJSON(t *testing.T, data string) map[string]any {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal([]byte(data), &doc); err != nil {
		t.Fatalf("unmarshal pick JSON: %v\n%s", err, data)
	}
	return doc
}

// F26-T3 row 1: the allowlist filter excludes non-listed models with
// not_in_availability_list and keeps listed ones.
func TestPickPipelineAllowlistFilter(t *testing.T) {
	cfg := pickTestConfig(t, t.TempDir(), "")
	allow := filepath.Join(t.TempDir(), "available.txt")
	if err := os.WriteFile(allow, []byte("# one per line\nclaude-sonnet-4-5\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	setLoadRoutes(t, func(string) (routing.Table, error) {
		return routesTable(f26ClaudeRoute(), f26CodexRoute()), nil
	})
	setScoreFunc(t, pickFakeScores(map[string]decimal.Decimal{
		"claude-sonnet-4-5": decimal.NewFromInt(90),
		"gpt-5-codex":       decimal.NewFromInt(80),
	}))

	err, out, _ := runPick(t, PickArgs{Profile: "complex_implementation", Strategy: "priority", Allowlists: []string{allow}, ConfigPath: cfg})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	doc := pickJSON(t, out.String())
	cands := doc["candidates"].([]any)
	if len(cands) != 1 {
		t.Fatalf("candidates = %d, want 1", len(cands))
	}
	if got := cands[0].(map[string]any)["candidate_id"]; got != "claude:claude-sonnet-4-5" {
		t.Errorf("candidate_id = %v, want claude:claude-sonnet-4-5", got)
	}
	ex := doc["excluded_candidates"].([]any)
	if len(ex) != 1 {
		t.Fatalf("excluded = %d, want 1", len(ex))
	}
	first := ex[0].(map[string]any)
	if first["reason_code"] != "not_in_availability_list" || first["reason"] != "model not in allowlist" {
		t.Errorf("excluded = %v", first)
	}
	if route := first["route"].(map[string]any); route["provider"] != "codex" || route["model_id"] != "gpt-5-codex" {
		t.Errorf("excluded route = %v", route)
	}
}

// F26-T3 row 2: a route whose model has no score row is excluded with
// no_score_row and warned on stderr.
func TestPickPipelineNoScoreExclusion(t *testing.T) {
	cfg := pickTestConfig(t, t.TempDir(), "")
	setLoadRoutes(t, func(string) (routing.Table, error) {
		return routesTable(f26ClaudeRoute(), routing.Route{
			Provider: "codex", ModelID: "gpt-5-codex", Model: "noscore-model",
			Reasoning: "default", WindowIDs: []string{"5h"}, Provenance: routing.ProvenanceProviderLive,
		}), nil
	})
	setScoreFunc(t, pickFakeScores(map[string]decimal.Decimal{
		"claude-sonnet-4-5": decimal.NewFromInt(90),
	}))

	err, out, errOut := runPick(t, PickArgs{Profile: "complex_implementation", Strategy: "priority", ConfigPath: cfg})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	doc := pickJSON(t, out.String())
	if got := doc["candidates"].([]any)[0].(map[string]any)["candidate_id"]; got != "claude:claude-sonnet-4-5" {
		t.Errorf("candidate_id = %v", got)
	}
	ex := doc["excluded_candidates"].([]any)
	if len(ex) != 1 {
		t.Fatalf("excluded = %d, want 1", len(ex))
	}
	first := ex[0].(map[string]any)
	if first["reason_code"] != "no_score_row" || first["reason"] != "no score row for noscore-model/default" {
		t.Errorf("excluded = %v", first)
	}
	if !strings.Contains(errOut.String(), "warning: no score row for noscore-model/default; excluded") {
		t.Errorf("stderr = %q, want no-score warning", errOut.String())
	}
}

// F26-T3 row 3: a score row with no route is a stderr warning only — never
// an excluded_candidates entry, never a candidate (SPEC §2.2d; D-6).
func TestPickPipelineUnroutedWarning(t *testing.T) {
	cfg := pickTestConfig(t, t.TempDir(), "")
	setLoadRoutes(t, func(string) (routing.Table, error) {
		return routesTable(f26ClaudeRoute()), nil
	})
	setReadScores(t, func(string) ([]ScoreRow, error) {
		return []ScoreRow{{Model: "unrouted-model", Reasoning: "default"}}, nil
	})
	setScoreFunc(t, func(profile, model, reasoning string) (decimal.Decimal, bool, map[string]float64) {
		if model == "unrouted-model" {
			return decimal.NewFromInt(42), true, map[string]float64{"tier1": 42, "category": 0}
		}
		if model == "claude-sonnet-4-5" {
			return decimal.NewFromInt(90), true, map[string]float64{"tier1": 90, "category": 0}
		}
		return decimal.Zero, false, nil
	})

	err, out, errOut := runPick(t, PickArgs{Profile: "complex_implementation", Strategy: "priority", ConfigPath: cfg})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(errOut.String(), "warning: no route for score row unrouted-model/default; ignored") {
		t.Errorf("stderr = %q, want unrouted warning", errOut.String())
	}
	doc := pickJSON(t, out.String())
	for _, c := range doc["candidates"].([]any) {
		if c.(map[string]any)["candidate_id"] == "provider:unrouted-model" {
			t.Error("unrouted model became a candidate")
		}
	}
	for _, x := range doc["excluded_candidates"].([]any) {
		if x.(map[string]any)["reason_code"] == "no_route_row" {
			t.Error("unrouted model appears in excluded_candidates")
		}
	}
}

// F26-T3 row 4: rank order is model_score desc; ties break by provider
// order (config priority), then model_id lexical (SPEC §2.2c).
func TestPickPipelineRankOrder(t *testing.T) {
	setSuccessfulUsage(t)
	providers := "[providers.codex]\nenabled = true\npriority = 10\n\n[providers.claude]\nenabled = true\npriority = 5\n"
	cfg := pickTestConfig(t, t.TempDir(), pickScoresConfigBody(filepath.Join(t.TempDir(), "scores.csv"), providers))
	setLoadRoutes(t, func(string) (routing.Table, error) {
		return routesTable(
			f26ClaudeRoute(),
			routing.Route{Provider: "claude", ModelID: "claude-opus-4-6", Model: "claude-opus-4-6", Reasoning: "default", WindowIDs: []string{"5h"}, Provenance: routing.ProvenanceProviderLive},
			routing.Route{Provider: "claude", ModelID: "claude-haiku-4-5", Model: "claude-haiku-4-5", Reasoning: "default", WindowIDs: []string{"5h"}, Provenance: routing.ProvenanceProviderLive},
			f26CodexRoute(),
			routing.Route{Provider: "codex", ModelID: "gpt-5-mini", Model: "gpt-5-mini", Reasoning: "default", WindowIDs: []string{"5h"}, Provenance: routing.ProvenanceProviderLive},
		), nil
	})
	setScoreFunc(t, pickFakeScores(map[string]decimal.Decimal{
		"gpt-5-codex":       decimal.NewFromInt(90),
		"gpt-5-mini":        decimal.NewFromInt(50),
		"claude-haiku-4-5":  decimal.NewFromInt(50),
		"claude-opus-4-6":   decimal.NewFromInt(50),
		"claude-sonnet-4-5": decimal.NewFromInt(10),
	}))

	err, out, _ := runPick(t, PickArgs{Profile: "complex_implementation", Strategy: "priority", ConfigPath: cfg})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	want := []string{"codex:gpt-5-codex", "claude:claude-haiku-4-5", "claude:claude-opus-4-6", "claude:claude-sonnet-4-5", "codex:gpt-5-mini"}
	got := make([]string, 0, len(want))
	for _, c := range pickJSON(t, out.String())["candidates"].([]any) {
		got = append(got, c.(map[string]any)["candidate_id"].(string))
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("rank order = %v, want %v", got, want)
	}
}

// F26-T3 row 5: zero survivors → CodedError no_pick, exit 3, nothing on
// stdout (the failure line is F22's job).
func TestPickPipelineZeroSurvivors(t *testing.T) {
	cfg := pickTestConfig(t, t.TempDir(), "")
	allow := filepath.Join(t.TempDir(), "available.txt")
	if err := os.WriteFile(allow, []byte("nothing-here\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	setLoadRoutes(t, func(string) (routing.Table, error) {
		return routesTable(f26ClaudeRoute()), nil
	})
	setScoreFunc(t, pickFakeScores(map[string]decimal.Decimal{"claude-sonnet-4-5": decimal.NewFromInt(90)}))

	err, out, _ := runPick(t, PickArgs{Profile: "complex_implementation", Strategy: "priority", Allowlists: []string{allow}, ConfigPath: cfg})
	var ce *CodedError
	if !errors.As(err, &ce) || ce.Code != "no_pick" {
		t.Fatalf("err = %v, want CodedError no_pick", err)
	}
	if ce.Message != "no candidate matched the request" {
		t.Errorf("message = %q", ce.Message)
	}
	if ExitCodeFor(err) != 3 {
		t.Errorf("exit = %d, want 3", ExitCodeFor(err))
	}
	if out.Len() != 0 {
		t.Errorf("stdout = %q, want empty", out.String())
	}
}

// F26-T3 row 6: the JSON document carries the full candidate shape — route
// object with exactly the five annex-c keys, model_score, provider_weight,
// final_score == model_score at this stage, warnings [], usage_enabled from
// the toggle seam, band fields omitted until T4.
func TestPickPipelineJSONShape(t *testing.T) {
	setSuccessfulUsage(t)
	cfg := pickTestConfig(t, t.TempDir(), "")
	setLoadRoutes(t, func(string) (routing.Table, error) {
		return routesTable(f26ClaudeRoute(), f26CodexRoute()), nil
	})
	setScoreFunc(t, pickFakeScores(map[string]decimal.Decimal{
		"claude-sonnet-4-5": decimal.NewFromInt(92),
		"gpt-5-codex":       decimal.NewFromInt(80),
	}))
	setToggleResolve(t, func(bool, *config.Config) (bool, string) { return true, "" })

	err, out, _ := runPick(t, PickArgs{Profile: "complex_implementation", Strategy: "priority", ConfigPath: cfg})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	doc := pickJSON(t, out.String())
	if doc["schema_version"] != "2.0" || doc["usage_enabled"] != true {
		t.Errorf("root = %v / %v", doc["schema_version"], doc["usage_enabled"])
	}
	if doc["usage_disabled_reason"] != nil {
		t.Errorf("usage_disabled_reason = %v, want null", doc["usage_disabled_reason"])
	}
	if doc["profile"] != "complex_implementation" || doc["strategy"] != "priority" {
		t.Errorf("profile/strategy = %v / %v", doc["profile"], doc["strategy"])
	}
	if _, ok := doc["seed"]; ok {
		t.Error("removed seed field is present")
	}
	cands := doc["candidates"].([]any)
	if len(cands) != 2 {
		t.Fatalf("candidates = %d, want 2", len(cands))
	}
	first := cands[0].(map[string]any)
	if first["candidate_id"] != "claude:claude-sonnet-4-5" {
		t.Errorf("candidate_id = %v", first["candidate_id"])
	}
	route := first["route"].(map[string]any)
	if len(route) != 5 {
		t.Fatalf("route keys = %d (%v), want exactly 5", len(route), route)
	}
	if route["provider"] != "claude" || route["model_id"] != "claude-sonnet-4-5" || route["model"] != "claude-sonnet-4-5" {
		t.Errorf("route = %v", route)
	}
	if route["reasoning"] != "default" {
		t.Errorf("reasoning = %v", route["reasoning"])
	}
	windows, ok := route["window_ids"].([]any)
	if !ok || len(windows) != 2 || windows[0] != "5h" || windows[1] != "7d" {
		t.Errorf("window_ids = %v, want [5h 7d]", route["window_ids"])
	}
	if first["model_score"] != 92.0 || first["provider_weight"] != 1.0 {
		t.Errorf("model_score/provider_weight = %v / %v", first["model_score"], first["provider_weight"])
	}
	if first["final_score"] != first["model_score"] {
		t.Errorf("final_score = %v, want model_score %v at this stage", first["final_score"], first["model_score"])
	}
	if w, ok := first["warnings"].([]any); !ok || len(w) != 0 {
		t.Errorf("warnings = %v, want []", first["warnings"])
	}
	if first["band"] != "available" {
		t.Errorf("band = %v, want available", first["band"])
	}
	if first["band_weight"] != 1.0 {
		t.Errorf("band_weight = %v, want 1", first["band_weight"])
	}
	ex := doc["excluded_candidates"].([]any)
	if len(ex) != 0 {
		t.Fatalf("excluded = %d, want 0", len(ex))
	}
}

// F26-T3 row 7: a missing allowlist file is a UsageError with the exact
// message (exit 2).
func TestPickPipelineMissingAllowlistFile(t *testing.T) {
	cfg := pickTestConfig(t, t.TempDir(), "")
	err, _, _ := runPick(t, PickArgs{Profile: "complex_implementation", Strategy: "priority", Allowlists: []string{filepath.Join(t.TempDir(), "missing.txt")}, ConfigPath: cfg})
	var ue *UsageError
	if !errors.As(err, &ue) {
		t.Fatalf("err = %v (%T), want *UsageError", err, err)
	}
	if !strings.Contains(ue.Message, `allowlist file "`) || !strings.Contains(ue.Message, "not found") {
		t.Errorf("message = %q", ue.Message)
	}
	if ExitCodeFor(err) != 2 {
		t.Errorf("exit = %d, want 2", ExitCodeFor(err))
	}
}

// F26-T3 extra: provider_weight comes from [providers.<id>].weight with a
// 1.0 default (SPEC §2.2d).
func TestPickPipelineProviderWeight(t *testing.T) {
	setSuccessfulUsage(t)
	providers := "[providers.claude]\nenabled = true\nweight = 2.5\n"
	cfg := pickTestConfig(t, t.TempDir(), pickScoresConfigBody(filepath.Join(t.TempDir(), "scores.csv"), providers))
	setLoadRoutes(t, func(string) (routing.Table, error) {
		return routesTable(f26ClaudeRoute()), nil
	})
	setScoreFunc(t, pickFakeScores(map[string]decimal.Decimal{"claude-sonnet-4-5": decimal.NewFromInt(90)}))
	setToggleResolve(t, func(bool, *config.Config) (bool, string) { return true, "" })

	err, out, _ := runPick(t, PickArgs{Profile: "complex_implementation", Strategy: "priority", ConfigPath: cfg})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	first := pickJSON(t, out.String())["candidates"].([]any)[0].(map[string]any)
	if first["provider_weight"] != 2.5 {
		t.Errorf("provider_weight = %v, want 2.5", first["provider_weight"])
	}
}
