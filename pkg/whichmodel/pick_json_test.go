// F26-T8: PickResult JSON golden (annex-c §4.2 verbatim)
// (specs/features/F26-cmd-pick/TASKS.md T8; SPEC §2.3.8; CONTRACTS §5).
package whichmodel

import (
	"errors"
	"strings"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/WD-Mitchell/which-model/internal/config"
	"github.com/WD-Mitchell/which-model/internal/routing"
	"github.com/WD-Mitchell/which-model/internal/usage"
)

// setGlobalPickValues pins the Global normalizer/aggregator the document
// echoes verbatim (SPEC §2.3.8), restoring the previous value on cleanup.
func setGlobalPickValues(t *testing.T) {
	t.Helper()
	prev := Global
	Global = GlobalFlags{Normalizer: "minmax-linear", Aggregator: "weighted-arithmetic-mean"}
	t.Cleanup(func() { Global = prev })
}

// t8BandGatesCodex is the T8 band fake: claude survives on band "five
// hour" (25% used, weight 0.8); codex is gated at 95%.
func t8BandGatesCodex(snap *usage.Snapshot, _ string, _ *config.Config) (bandResult, error) {
	if snap.Provider == "codex" {
		return bandResult{Name: "five hour", UsedPercent: 95, Gated: true}, nil
	}
	return bandResult{Name: "five hour", UsedPercent: 25, Weight: 0.8}, nil
}

// F26-T8 row 1: the golden document — full fixture (profile
// complex_implementation, strategy score, routes claude + codex, codex
// band-gated, usage enabled) unmarshals to exactly the annex-c §4.2
// shape, field by field (all 14 asserts).
func TestPickJSONGolden(t *testing.T) {
	setGlobalPickValues(t)
	cfg, _ := pickPipelineSetup(t, pickTwoRoutes(), pickTwoScores(), nil, nil, t8BandGatesCodex)

	err, out, _ := runPick(t, PickArgs{Profile: "complex_implementation", Strategy: "priority", ConfigPath: cfg})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	doc := pickJSON(t, out.String())

	// Root fields.
	if doc["schema_version"] != "2.0" {
		t.Errorf("schema_version = %v, want 2.0", doc["schema_version"])
	}
	if doc["usage_enabled"] != true {
		t.Errorf("usage_enabled = %v, want true", doc["usage_enabled"])
	}
	if doc["usage_disabled_reason"] != nil {
		t.Errorf("usage_disabled_reason = %v, want null", doc["usage_disabled_reason"])
	}
	if doc["profile"] != "complex_implementation" {
		t.Errorf("profile = %v, want complex_implementation", doc["profile"])
	}
	if doc["strategy"] != "priority" {
		t.Errorf("strategy = %v, want priority", doc["strategy"])
	}
	if _, ok := doc["seed"]; ok {
		t.Error("removed seed field is present")
	}
	if doc["normalizer"] != "minmax-linear" {
		t.Errorf("normalizer = %v, want minmax-linear", doc["normalizer"])
	}
	if doc["aggregator"] != "weighted-arithmetic-mean" {
		t.Errorf("aggregator = %v, want weighted-arithmetic-mean", doc["aggregator"])
	}

	// candidates[0].
	cands := doc["candidates"].([]any)
	if len(cands) != 1 {
		t.Fatalf("candidates = %d, want 1", len(cands))
	}
	first := cands[0].(map[string]any)
	if first["candidate_id"] != "claude:claude-sonnet-4-5" {
		t.Errorf("candidate_id = %v, want claude:claude-sonnet-4-5", first["candidate_id"])
	}
	route := first["route"].(map[string]any)
	if len(route) != 5 {
		t.Fatalf("route keys = %d (%v), want exactly 5", len(route), route)
	}
	if route["provider"] != "claude" || route["model_id"] != "claude-sonnet-4-5" || route["model"] != "claude-sonnet-4-5" {
		t.Errorf("route = %v, want claude/claude-sonnet-4-5", route)
	}
	if route["reasoning"] != "default" {
		t.Errorf("reasoning = %v, want default", route["reasoning"])
	}
	windows, ok := route["window_ids"].([]any)
	if !ok || len(windows) != 2 || windows[0] != "5h" || windows[1] != "7d" {
		t.Errorf("window_ids = %v, want [5h 7d]", route["window_ids"])
	}
	if first["model_score"] != 92.0 {
		t.Errorf("model_score = %v, want 92", first["model_score"])
	}
	if first["band"] != "five hour" {
		t.Errorf("band = %v, want five hour", first["band"])
	}
	if first["band_weight"] != 0.8 {
		t.Errorf("band_weight = %v, want 0.8", first["band_weight"])
	}
	if first["provider_weight"] != 1.0 {
		t.Errorf("provider_weight = %v, want 1.0", first["provider_weight"])
	}
	if first["final_score"] != 73.6 {
		t.Errorf("final_score = %v, want 73.6 (92 * 0.8 * 1.0)", first["final_score"])
	}
	if w, ok := first["warnings"].([]any); !ok || len(w) != 0 {
		t.Errorf("warnings = %v, want []", first["warnings"])
	}

	// excluded_candidates[0].
	ex := doc["excluded_candidates"].([]any)
	if len(ex) != 1 {
		t.Fatalf("excluded_candidates = %d, want 1", len(ex))
	}
	x := ex[0].(map[string]any)
	if x["reason_code"] != "band_gated" {
		t.Errorf("reason_code = %v, want band_gated", x["reason_code"])
	}
	if x["reason"] != "band usage 95% > gate" {
		t.Errorf("reason = %v, want %q", x["reason"], "band usage 95% > gate")
	}
	xr := x["route"].(map[string]any)
	if len(xr) != 5 {
		t.Errorf("excluded route keys = %d (%v), want exactly 5", len(xr), xr)
	}
	if xr["provider"] != "codex" || xr["model_id"] != "gpt-5-codex" {
		t.Errorf("excluded route = %v, want codex/gpt-5-codex", xr)
	}
}

// F26-T8 row 2: empty arrays serialize as [] — never null — on both
// containers: an all-excluded run still emits the no-pick report with
// candidates [] and excluded_candidates populated, and a zero-exclusion
// run emits excluded_candidates [].
func TestPickJSONEmptyArrays(t *testing.T) {
	t.Run("all excluded", func(t *testing.T) {
		cfg, _ := pickPipelineSetup(t, pickTwoRoutes(), pickTwoScores(), nil, nil,
			func(_ *usage.Snapshot, _ string, _ *config.Config) (bandResult, error) {
				return bandResult{Name: "five hour", UsedPercent: 95, Gated: true}, nil
			})

		err, out, _ := runPick(t, PickArgs{Profile: "complex_implementation", Strategy: "priority", ConfigPath: cfg})
		var ce *CodedError
		if !errors.As(err, &ce) || ce.Code != "usage_gated" {
			t.Fatalf("err = %v, want CodedError usage_gated", err)
		}
		if ExitCodeFor(err) != 4 {
			t.Errorf("exit = %d, want 4", ExitCodeFor(err))
		}
		// The report is emitted before the classified error returns: the
		// candidate list must be an empty ARRAY, not null (CONTRACTS §5).
		doc := pickJSON(t, out.String())
		cands, ok := doc["candidates"].([]any)
		if !ok || cands == nil || len(cands) != 0 {
			t.Errorf("candidates = %#v, want [] (not null)", doc["candidates"])
		}
		ex, ok := doc["excluded_candidates"].([]any)
		if !ok || len(ex) != 2 {
			t.Fatalf("excluded_candidates = %#v, want 2 entries", doc["excluded_candidates"])
		}
		for _, x := range ex {
			if x.(map[string]any)["reason_code"] != "band_gated" {
				t.Errorf("excluded = %v, want band_gated", x)
			}
		}
	})

	t.Run("no exclusions", func(t *testing.T) {
		cfg, _ := pickPipelineSetup(t, []routing.Route{f26ClaudeRoute()}, map[string]decimal.Decimal{
			"claude-sonnet-4-5": decimal.NewFromInt(92),
		}, nil, nil, nil)

		err, out, _ := runPick(t, PickArgs{Profile: "complex_implementation", Strategy: "priority", ConfigPath: cfg})
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		doc := pickJSON(t, out.String())
		ex, ok := doc["excluded_candidates"].([]any)
		if !ok || ex == nil || len(ex) != 0 {
			t.Errorf("excluded_candidates = %#v, want [] (not null)", doc["excluded_candidates"])
		}
	})
}

// F26-T8 row 3: degraded mode — usage disabled via the toggle → the
// candidate object carries NO band/band_weight keys (absent, not null),
// asserted via map key presence.
func TestPickJSONDegradedOmission(t *testing.T) {
	cfg, _ := pickPipelineSetup(t, pickTwoRoutes(), pickTwoScores(),
		func(_ bool, _ *config.Config) (bool, string) { return false, "flag" }, nil, nil)

	err, out, _ := runPick(t, PickArgs{Profile: "complex_implementation", Strategy: "priority", ConfigPath: cfg})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	doc := pickJSON(t, out.String())
	if doc["usage_enabled"] != false {
		t.Errorf("usage_enabled = %v, want false", doc["usage_enabled"])
	}
	if doc["usage_disabled_reason"] != "flag" {
		t.Errorf("usage_disabled_reason = %v, want flag", doc["usage_disabled_reason"])
	}
	first := doc["candidates"].([]any)[0].(map[string]any)
	if _, ok := first["band"]; ok {
		t.Errorf("candidate carries band key in degraded mode: %v", first["band"])
	}
	if _, ok := first["band_weight"]; ok {
		t.Errorf("candidate carries band_weight key in degraded mode: %v", first["band_weight"])
	}
}

// Removed strategy-specific seed metadata is absent from pick JSON.
func TestPickJSONOmitsSeed(t *testing.T) {
	cfg, _ := pickPipelineSetup(t, pickTwoRoutes(), pickTwoScores(), nil, nil, nil)
	err, out, _ := runPick(t, PickArgs{Profile: "complex_implementation", Strategy: "priority", ConfigPath: cfg})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := pickJSON(t, out.String())["seed"]; ok {
		t.Error("removed seed field is present")
	}
}

// F26-T8 row 5: text golden for the same fixture — the picked line plus
// the indented profile/strategy/band reason lines (CONTRACTS §7).
func TestPickJSONTextGolden(t *testing.T) {
	cfg, _ := pickPipelineSetup(t, pickTwoRoutes(), pickTwoScores(), nil, nil, t8BandGatesCodex)

	var o, e strings.Builder
	err := RunPick(PickArgs{Profile: "complex_implementation", Strategy: "priority", ConfigPath: cfg}, &o, &e)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	want := "picked claude-sonnet-4-5 via claude (score 73.6)\n" +
		"  profile: complex_implementation\n" +
		"  strategy: priority\n" +
		"  band: five hour (25% used, weight 0.8)\n"
	if o.String() != want {
		t.Errorf("text = %q, want %q", o.String(), want)
	}
}
