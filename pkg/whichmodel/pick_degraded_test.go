// F26-T5: degraded mode (usage disabled) + strict no_providers
// (specs/features/F26-cmd-pick/TASKS.md T5; SPEC §2.4, §2.14, §2.15;
// CONTRACTS §8.2; specs/global/CONTRACTS.md §1.6).
package whichmodel

import (
	"context"
	"errors"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/WD-Mitchell/which-model/internal/config"
	"github.com/WD-Mitchell/which-model/internal/routing"
	"github.com/WD-Mitchell/which-model/internal/usage"
)

// F26-T5 row 1: flag-disabled degraded mode — usage_enabled false, reason
// "flag", candidates carry NO band/band_weight keys, and neither the fetch
// nor the band seam is ever called.
func TestPickDegradedFlagDisabled(t *testing.T) {
	var fetchCalls, bandCalls int
	cfg, _ := pickPipelineSetup(t, pickTwoRoutes(), pickTwoScores(),
		func(_ bool, _ *config.Config) (bool, string) { return false, "flag" },
		func(_ context.Context, _ []string, _ pickFetchOptions) (map[string]*usageSnapshot, map[string]timeValue, error) {
			fetchCalls++
			return nil, nil, errors.New("fetch must not be called")
		},
		func(_ *usage.Snapshot, _ string, _ *config.Config) (bandResult, error) {
			bandCalls++
			return bandResult{}, errors.New("band must not be called")
		})

	err, out, _ := runPick(t, PickArgs{Profile: "complex_implementation", Strategy: "score", ConfigPath: cfg})
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
		t.Errorf("candidate carries a band key in degraded mode: %v", first["band"])
	}
	if _, ok := first["band_weight"]; ok {
		t.Errorf("candidate carries a band_weight key in degraded mode: %v", first["band_weight"])
	}
	if fetchCalls != 0 {
		t.Errorf("fetch seam called %d times, want 0", fetchCalls)
	}
	if bandCalls != 0 {
		t.Errorf("band seam called %d times, want 0", bandCalls)
	}
}

// F26-T5 row 2: config-disabled degraded mode reports the toggle reason.
func TestPickDegradedConfigDisabled(t *testing.T) {
	cfg, _ := pickPipelineSetup(t, pickTwoRoutes(), pickTwoScores(),
		func(_ bool, _ *config.Config) (bool, string) { return false, "config" }, nil, nil)

	err, out, _ := runPick(t, PickArgs{Profile: "complex_implementation", Strategy: "score", ConfigPath: cfg})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	doc := pickJSON(t, out.String())
	if doc["usage_enabled"] != false {
		t.Errorf("usage_enabled = %v, want false", doc["usage_enabled"])
	}
	if doc["usage_disabled_reason"] != "config" {
		t.Errorf("usage_disabled_reason = %v, want config", doc["usage_disabled_reason"])
	}
}

// F26-T5 row 3: least_used in degraded mode is refused with usage_disabled
// (exit 2) BEFORE the strategy seam — Apply is never called.
func TestPickDegradedLeastUsedRefusal(t *testing.T) {
	setStrategyNames(t, []string{"score", "least_used", "weighted_random"})
	var applyCalls int
	cfg, _ := pickPipelineSetup(t, pickTwoRoutes(), pickTwoScores(),
		func(_ bool, _ *config.Config) (bool, string) { return false, "flag" }, nil, nil)
	setStrategyApply(t, func(name string, cands []Candidate, opts strategyOptions) ([]Candidate, error) {
		applyCalls++
		return cands, nil
	})

	err, _, _ := runPick(t, PickArgs{Profile: "complex_implementation", Strategy: "least_used", ConfigPath: cfg})
	var ce *CodedError
	if !errors.As(err, &ce) {
		t.Fatalf("err = %v, want *CodedError", err)
	}
	if ce.Code != "usage_disabled" {
		t.Errorf("code = %q, want usage_disabled", ce.Code)
	}
	if ce.Message != `strategy "least_used" requires usage data` {
		t.Errorf("message = %q, want %q", ce.Message, `strategy "least_used" requires usage data`)
	}
	if ExitCodeFor(err) != 2 {
		t.Errorf("exit = %d, want 2", ExitCodeFor(err))
	}
	if applyCalls != 0 {
		t.Errorf("strategy seam called %d times, want 0", applyCalls)
	}
}

// F26-T5 row 4: strict no_providers misconfiguration — toggle reason
// no_providers_enabled with [usage] enabled = "true" is a usage_config
// error (exit 2) with the exact SPEC §3 message, never a silent degrade.
func TestPickDegradedStrictNoProviders(t *testing.T) {
	cfg := pickTestConfig(t, t.TempDir(), "[usage]\nenabled = true\n")
	setLoadRoutes(t, func(string) (routing.Table, error) {
		return routesTable(f26ClaudeRoute()), nil
	})
	setScoreFunc(t, pickFakeScores(map[string]decimal.Decimal{"claude-sonnet-4-5": decimal.NewFromInt(92)}))
	setToggleResolve(t, func(_ bool, _ *config.Config) (bool, string) { return false, "no_providers_enabled" })
	setStateDir(t, func() string { return t.TempDir() })

	err, _, _ := runPick(t, PickArgs{Profile: "complex_implementation", Strategy: "score", ConfigPath: cfg})
	var ce *CodedError
	if !errors.As(err, &ce) {
		t.Fatalf("err = %v, want *CodedError", err)
	}
	if ce.Code != "usage_config" {
		t.Errorf("code = %q, want usage_config", ce.Code)
	}
	want := `usage is enabled but no providers are enabled; set [providers.<id>] enabled = true or [usage] enabled = "auto"`
	if ce.Message != want {
		t.Errorf("message = %q, want %q", ce.Message, want)
	}
	if ExitCodeFor(err) != 2 {
		t.Errorf("exit = %d, want 2", ExitCodeFor(err))
	}
}

// F26-T5 row 5: byte-reproducibility — two identical RunPick calls produce
// identical stdout bytes, in degraded mode and with weighted_random plus a
// fixed seed.
func TestPickDegradedByteReproducible(t *testing.T) {
	cfg, _ := pickPipelineSetup(t, pickTwoRoutes(), pickTwoScores(),
		func(_ bool, _ *config.Config) (bool, string) { return false, "flag" }, nil, nil)

	_, out1, _ := runPick(t, PickArgs{Profile: "complex_implementation", Strategy: "score", ConfigPath: cfg})
	_, out2, _ := runPick(t, PickArgs{Profile: "complex_implementation", Strategy: "score", ConfigPath: cfg})
	if out1.String() != out2.String() {
		t.Errorf("degraded stdout differs between runs:\n%q\nvs\n%q", out1.String(), out2.String())
	}

	setStrategyNames(t, []string{"score", "weighted_random"})
	seed := uint64(7)
	_, out3, _ := runPick(t, PickArgs{Profile: "complex_implementation", Strategy: "weighted_random", Seed: &seed, ConfigPath: cfg})
	_, out4, _ := runPick(t, PickArgs{Profile: "complex_implementation", Strategy: "weighted_random", Seed: &seed, ConfigPath: cfg})
	if out3.String() != out4.String() {
		t.Errorf("weighted_random stdout differs between runs:\n%q\nvs\n%q", out3.String(), out4.String())
	}
}

// F26-T5 row 6: compiled_out degrades like the flag — no refusal, reason
// "compiled_out" (F21 nousage stub path).
func TestPickDegradedCompiledOut(t *testing.T) {
	cfg, _ := pickPipelineSetup(t, pickTwoRoutes(), pickTwoScores(),
		func(_ bool, _ *config.Config) (bool, string) { return false, "compiled_out" }, nil, nil)

	err, out, _ := runPick(t, PickArgs{Profile: "complex_implementation", Strategy: "score", ConfigPath: cfg})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	doc := pickJSON(t, out.String())
	if doc["usage_enabled"] != false {
		t.Errorf("usage_enabled = %v, want false", doc["usage_enabled"])
	}
	if doc["usage_disabled_reason"] != "compiled_out" {
		t.Errorf("usage_disabled_reason = %v, want compiled_out", doc["usage_disabled_reason"])
	}
}
