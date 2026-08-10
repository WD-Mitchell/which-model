// F26-T7: exit matrix — zero-survivor classification with precedence
// 5 > 4 > 3 (specs/features/F26-cmd-pick/TASKS.md T7; SPEC §2.5, §3;
// CONTRACTS §4; Decision D-15). Drives full RunPick runs reusing the
// T3–T6 fakes from pick_pipeline_test.go / pick_usage_test.go.
package whichmodel

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/WD-Mitchell/which-model/internal/config"
	"github.com/WD-Mitchell/which-model/internal/routing"
	"github.com/WD-Mitchell/which-model/internal/usage"
)

// pickExcludedReasonCodes extracts the excluded_candidates reason_code
// values from a pick JSON document, in document order.
func pickExcludedReasonCodes(t *testing.T, data string) []string {
	t.Helper()
	ex := pickJSON(t, data)["excluded_candidates"].([]any)
	codes := make([]string, 0, len(ex))
	for _, x := range ex {
		codes = append(codes, x.(map[string]any)["reason_code"].(string))
	}
	return codes
}

// F26-T7 row 1: auth wins — band_gated + auth_required → *CodedError
// {Code: "auth_required"} (exit 5) with the exact SPEC §3 message; nothing
// on stdout (the failure line is F22's job).
func TestPickExitAuthWins(t *testing.T) {
	cfg, _ := pickPipelineSetup(t, pickTwoRoutes(), pickTwoScores(), nil,
		func(_ context.Context, _ []string, _ pickFetchOptions) (map[string]*usageSnapshot, map[string]timeValue, error) {
			return map[string]*usageSnapshot{
				"claude": {Provider: "claude"},
				"codex":  {Provider: "codex", Failure: &usage.Failure{Code: "unauthorized", Message: "unauthorized"}},
			}, map[string]timeValue{}, nil
		},
		func(snap *usage.Snapshot, _ string, _ *config.Config) (bandResult, error) {
			if snap.Provider == "claude" {
				return bandResult{Name: "five hour", UsedPercent: 95, Gated: true}, nil
			}
			return bandResult{Name: "five hour", UsedPercent: 10, Weight: 1}, nil
		})

	err, out, _ := runPick(t, PickArgs{Profile: "complex_implementation", Strategy: "score", ConfigPath: cfg})
	var ce *CodedError
	if !errors.As(err, &ce) {
		t.Fatalf("err = %v (%T), want *CodedError", err, err)
	}
	if ce.Code != "auth_required" {
		t.Errorf("code = %q, want auth_required", ce.Code)
	}
	if ce.Message != "auth required; check CodexBar credentials" {
		t.Errorf("message = %q, want %q", ce.Message, "auth required; check CodexBar credentials")
	}
	if ExitCodeFor(err) != 5 {
		t.Errorf("exit = %d, want 5", ExitCodeFor(err))
	}
	if got := len(pickJSON(t, out.String())["candidates"].([]any)); got != 0 {
		t.Errorf("candidates = %d, want 0", got)
	}
}

// F26-T7 row 2: gating beats availability — not_in_availability_list +
// band_gated → *CodedError {Code: "usage_gated"} (exit 4) with the exact
// SPEC §3 message.
func TestPickExitGatingBeatsAvailability(t *testing.T) {
	allow := filepath.Join(t.TempDir(), "available.txt")
	if err := os.WriteFile(allow, []byte("# one per line\nclaude-sonnet-4-5\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, _ := pickPipelineSetup(t, pickTwoRoutes(), pickTwoScores(), nil, nil,
		func(_ *usage.Snapshot, _ string, _ *config.Config) (bandResult, error) {
			return bandResult{Name: "five hour", UsedPercent: 95, Gated: true}, nil
		})

	err, out, _ := runPick(t, PickArgs{Profile: "complex_implementation", Strategy: "score", Allowlists: []string{allow}, ConfigPath: cfg})
	var ce *CodedError
	if !errors.As(err, &ce) {
		t.Fatalf("err = %v (%T), want *CodedError", err, err)
	}
	if ce.Code != "usage_gated" {
		t.Errorf("code = %q, want usage_gated", ce.Code)
	}
	if ce.Message != "usage gating excluded every candidate" {
		t.Errorf("message = %q, want %q", ce.Message, "usage gating excluded every candidate")
	}
	if ExitCodeFor(err) != 4 {
		t.Errorf("exit = %d, want 4", ExitCodeFor(err))
	}
	if got := len(pickJSON(t, out.String())["candidates"].([]any)); got != 0 {
		t.Errorf("candidates = %d, want 0", got)
	}
}

// F26-T7 row 3: availability only — not_in_availability_list +
// no_score_row → *CodedError {Code: "no_pick"} (exit 3) with the exact
// SPEC §3 message. No fetch/band involvement: the run ends at the filter
// stage.
func TestPickExitAvailabilityOnly(t *testing.T) {
	allow := filepath.Join(t.TempDir(), "available.txt")
	if err := os.WriteFile(allow, []byte("claude-sonnet-4-5\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, _ := pickPipelineSetup(t, pickTwoRoutes(), nil, nil, nil, nil)

	err, out, _ := runPick(t, PickArgs{Profile: "complex_implementation", Strategy: "score", Allowlists: []string{allow}, ConfigPath: cfg})
	var ce *CodedError
	if !errors.As(err, &ce) {
		t.Fatalf("err = %v (%T), want *CodedError", err, err)
	}
	if ce.Code != "no_pick" {
		t.Errorf("code = %q, want no_pick", ce.Code)
	}
	if ce.Message != "no candidate matched the request" {
		t.Errorf("message = %q, want %q", ce.Message, "no candidate matched the request")
	}
	if ExitCodeFor(err) != 3 {
		t.Errorf("exit = %d, want 3", ExitCodeFor(err))
	}
	if out.Len() != 0 {
		t.Errorf("stdout = %q, want empty", out.String())
	}
}

// F26-T7 row 4: provider_error class — provider_error + no_score_row →
// *CodedError {Code: "usage_gated"} (exit 4).
func TestPickExitProviderErrorClass(t *testing.T) {
	cfg, _ := pickPipelineSetup(t, pickTwoRoutes(), map[string]decimal.Decimal{
		"claude-sonnet-4-5": decimal.NewFromInt(92),
	}, nil,
		func(_ context.Context, _ []string, _ pickFetchOptions) (map[string]*usageSnapshot, map[string]timeValue, error) {
			return map[string]*usageSnapshot{
				"claude": {Provider: "claude", Failure: &usage.Failure{Code: "rate_limited", Message: "rate limit exceeded"}},
			}, map[string]timeValue{}, nil
		}, nil)

	err, out, _ := runPick(t, PickArgs{Profile: "complex_implementation", Strategy: "score", ConfigPath: cfg})
	var ce *CodedError
	if !errors.As(err, &ce) {
		t.Fatalf("err = %v (%T), want *CodedError", err, err)
	}
	if ce.Code != "usage_gated" {
		t.Errorf("code = %q, want usage_gated", ce.Code)
	}
	if ce.Message != "usage gating excluded every candidate" {
		t.Errorf("message = %q, want %q", ce.Message, "usage gating excluded every candidate")
	}
	if ExitCodeFor(err) != 4 {
		t.Errorf("exit = %d, want 4", ExitCodeFor(err))
	}
	if got := len(pickJSON(t, out.String())["candidates"].([]any)); got != 0 {
		t.Errorf("candidates = %d, want 0", got)
	}
}

// F26-T7 row 5: excluded but some survive — provider_error + no_score_row
// exclusions alongside one surviving candidate → exit 0 and a normal pick
// document (candidates + excluded_candidates both present).
func TestPickExitSurvivorWins(t *testing.T) {
	mini := routing.Route{
		Provider: "codex", ModelID: "gpt-5-mini", Model: "gpt-5-mini",
		Reasoning: "default", WindowIDs: []string{"5h"}, Provenance: routing.ProvenanceProviderLive,
	}
	cfg, _ := pickPipelineSetup(t, []routing.Route{f26ClaudeRoute(), f26CodexRoute(), mini}, map[string]decimal.Decimal{
		"claude-sonnet-4-5": decimal.NewFromInt(92),
		"gpt-5-codex":       decimal.NewFromInt(80),
	}, nil,
		func(_ context.Context, _ []string, _ pickFetchOptions) (map[string]*usageSnapshot, map[string]timeValue, error) {
			return map[string]*usageSnapshot{
				"claude": {Provider: "claude", Failure: &usage.Failure{Code: "rate_limited", Message: "rate limit exceeded"}},
				"codex":  {Provider: "codex"},
			}, map[string]timeValue{}, nil
		}, nil)

	err, out, _ := runPick(t, PickArgs{Profile: "complex_implementation", Strategy: "score", ConfigPath: cfg})
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if ExitCodeFor(err) != 0 {
		t.Errorf("exit = %d, want 0", ExitCodeFor(err))
	}
	doc := pickJSON(t, out.String())
	cands := doc["candidates"].([]any)
	if len(cands) != 1 || cands[0].(map[string]any)["candidate_id"] != "codex:gpt-5-codex" {
		t.Fatalf("candidates = %v, want codex:gpt-5-codex only", cands)
	}
	codes := pickExcludedReasonCodes(t, out.String())
	have := make(map[string]bool, len(codes))
	for _, c := range codes {
		have[c] = true
	}
	if !have["provider_error"] || !have["no_score_row"] {
		t.Errorf("excluded reason codes = %v, want provider_error and no_score_row", codes)
	}
}
