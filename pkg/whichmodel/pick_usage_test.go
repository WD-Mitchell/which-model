// F26-T4: usage stage — fetch, bands, gating
// (specs/features/F26-cmd-pick/TASKS.md T4; SPEC §2.2e–f, §2.3;
// CONTRACTS §8.3, §8.4).
package whichmodel

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/WD-Mitchell/which-model/internal/config"
	"github.com/WD-Mitchell/which-model/internal/routing"
	"github.com/WD-Mitchell/which-model/internal/usage"
)

// pickPipelineSetup wires the shared F26-T4..T9 pipeline fixture: routes,
// scores, and injectable toggle/fetch/band fakes (nil → permissive
// defaults), an identity strategy fake, and a temp state dir. Returns the
// config path and state dir.
func pickPipelineSetup(
	t *testing.T,
	routes []routing.Route,
	scores map[string]decimal.Decimal,
	toggle func(flagNoUsage bool, cfg *config.Config) (bool, string),
	fetch func(ctx context.Context, providers []string, opts pickFetchOptions) (map[string]*usageSnapshot, map[string]timeValue, error),
	band func(snap *usage.Snapshot, route string, cfg *config.Config) (bandResult, error),
) (cfg string, stateDir string) {
	t.Helper()
	cfg = pickTestConfig(t, t.TempDir(), "")
	stateDir = t.TempDir()
	setLoadRoutes(t, func(string) (routing.Table, error) { return routesTable(routes...), nil })
	setScoreFunc(t, pickFakeScores(scores))
	setToggleResolve(t, func(flagNoUsage bool, cfg *config.Config) (bool, string) {
		if toggle == nil {
			return true, ""
		}
		return toggle(flagNoUsage, cfg)
	})
	setPickFetchAll(t, func(ctx context.Context, providers []string, opts pickFetchOptions) (map[string]*usageSnapshot, map[string]timeValue, error) {
		if fetch == nil {
			snaps := make(map[string]*usageSnapshot, len(providers))
			for _, p := range providers {
				snaps[p] = &usage.Snapshot{Provider: p}
			}
			return snaps, map[string]timeValue{}, nil
		}
		return fetch(ctx, providers, opts)
	})
	setBandEvaluate(t, func(snap *usage.Snapshot, route string, cfg *config.Config) (bandResult, error) {
		if band == nil {
			return bandResult{Name: "available", Weight: 1}, nil
		}
		return band(snap, route, cfg)
	})
	setStrategyApply(t, func(name string, cands []Candidate, opts strategyOptions) ([]Candidate, error) {
		return cands, nil
	})
	setStateDir(t, func() string { return stateDir })
	return cfg, stateDir
}

// pickTwoRoutes is the shared claude+codex fixture (provider_live routes).
func pickTwoRoutes() []routing.Route {
	return []routing.Route{f26ClaudeRoute(), f26CodexRoute()}
}

// pickTwoScores scores both routed models: claude 92, codex 80.
func pickTwoScores() map[string]decimal.Decimal {
	return map[string]decimal.Decimal{
		"claude-sonnet-4-5": decimal.NewFromInt(92),
		"gpt-5-codex":       decimal.NewFromInt(80),
	}
}

// F26-T4 row 1: band gating — a gated band excludes the candidate with
// reason_code band_gated and reason "band usage 95% > gate"; the surviving
// candidate still wins the run.
func TestPickUsageBandGated(t *testing.T) {
	cfg, _ := pickPipelineSetup(t, pickTwoRoutes(), pickTwoScores(), nil, nil,
		func(snap *usage.Snapshot, _ string, _ *config.Config) (bandResult, error) {
			if snap.Provider == "claude" {
				return bandResult{Name: "five hour", UsedPercent: 95, Gated: true}, nil
			}
			return bandResult{Name: "five hour", UsedPercent: 10, Weight: 1}, nil
		})

	err, out, _ := runPick(t, PickArgs{Profile: "complex_implementation", Strategy: "score", ConfigPath: cfg})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	doc := pickJSON(t, out.String())
	ex := doc["excluded_candidates"].([]any)
	if len(ex) != 1 {
		t.Fatalf("excluded = %d, want 1", len(ex))
	}
	first := ex[0].(map[string]any)
	if first["reason_code"] != "band_gated" {
		t.Errorf("reason_code = %v, want band_gated", first["reason_code"])
	}
	if first["reason"] != "band usage 95% > gate" {
		t.Errorf("reason = %v, want %q", first["reason"], "band usage 95% > gate")
	}
	route := first["route"].(map[string]any)
	if route["provider"] != "claude" || route["model_id"] != "claude-sonnet-4-5" {
		t.Errorf("excluded route = %v, want claude route ref", route)
	}
	if got := doc["candidates"].([]any)[0].(map[string]any)["candidate_id"]; got != "codex:gpt-5-codex" {
		t.Errorf("candidate = %v, want codex:gpt-5-codex", got)
	}
}

// F26-T4 row 2: surviving bands — the candidate carries band "five hour"
// and band_weight 0.8, and final_score is recomputed as ModelScore ×
// BandWeight × ProviderWeight (92 * 0.8 * 1.0 == 73.6).
func TestPickUsageSurvivorBand(t *testing.T) {
	cfg, _ := pickPipelineSetup(t, []routing.Route{f26ClaudeRoute()}, map[string]decimal.Decimal{
		"claude-sonnet-4-5": decimal.NewFromInt(92),
	}, nil, nil,
		func(_ *usage.Snapshot, _ string, _ *config.Config) (bandResult, error) {
			return bandResult{Name: "five hour", UsedPercent: 25, Weight: 0.8}, nil
		})

	err, out, _ := runPick(t, PickArgs{Profile: "complex_implementation", Strategy: "score", ConfigPath: cfg})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	first := pickJSON(t, out.String())["candidates"].([]any)[0].(map[string]any)
	if first["band"] != "five hour" {
		t.Errorf("band = %v, want five hour", first["band"])
	}
	if first["band_weight"] != 0.8 {
		t.Errorf("band_weight = %v, want 0.8", first["band_weight"])
	}
	if first["provider_weight"] != 1.0 {
		t.Errorf("provider_weight = %v, want 1.0", first["provider_weight"])
	}
	if first["model_score"] != 92.0 {
		t.Errorf("model_score = %v, want 92", first["model_score"])
	}
	if first["final_score"] != 73.6 {
		t.Errorf("final_score = %v, want 73.6", first["final_score"])
	}
}

// F26-T4 row 3: an auth-class fetch failure excludes every survivor of that
// provider with auth_required and reason "provider claude: <message>".
func TestPickUsageAuthFailure(t *testing.T) {
	cfg, _ := pickPipelineSetup(t, pickTwoRoutes(), pickTwoScores(), nil,
		func(_ context.Context, _ []string, _ pickFetchOptions) (map[string]*usageSnapshot, map[string]timeValue, error) {
			return map[string]*usageSnapshot{
				"claude": {Provider: "claude", Failure: &usage.Failure{Code: "unauthorized", Message: "unauthorized"}},
				"codex":  {Provider: "codex"},
			}, map[string]timeValue{}, nil
		}, nil)

	err, out, _ := runPick(t, PickArgs{Profile: "complex_implementation", Strategy: "score", ConfigPath: cfg})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	ex := pickJSON(t, out.String())["excluded_candidates"].([]any)
	if len(ex) != 1 {
		t.Fatalf("excluded = %d, want 1", len(ex))
	}
	first := ex[0].(map[string]any)
	if first["reason_code"] != "auth_required" {
		t.Errorf("reason_code = %v, want auth_required", first["reason_code"])
	}
	if first["reason"] != "provider claude: unauthorized" {
		t.Errorf("reason = %v, want %q", first["reason"], "provider claude: unauthorized")
	}
	if route := first["route"].(map[string]any); route["provider"] != "claude" || route["model_id"] != "claude-sonnet-4-5" {
		t.Errorf("excluded route = %v", route)
	}
}

// F26-T4 row 4: a non-auth fetch failure excludes the provider's survivors
// with provider_error and the sanitised failure message as the reason.
func TestPickUsageProviderError(t *testing.T) {
	cfg, _ := pickPipelineSetup(t, pickTwoRoutes(), pickTwoScores(), nil,
		func(_ context.Context, _ []string, _ pickFetchOptions) (map[string]*usageSnapshot, map[string]timeValue, error) {
			return map[string]*usageSnapshot{
				"claude": {Provider: "claude", Failure: &usage.Failure{Code: "rate_limited", Message: "rate limit exceeded"}},
				"codex":  {Provider: "codex"},
			}, map[string]timeValue{}, nil
		}, nil)

	err, out, _ := runPick(t, PickArgs{Profile: "complex_implementation", Strategy: "score", ConfigPath: cfg})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	first := pickJSON(t, out.String())["excluded_candidates"].([]any)[0].(map[string]any)
	if first["reason_code"] != "provider_error" {
		t.Errorf("reason_code = %v, want provider_error", first["reason_code"])
	}
	if first["reason"] != "rate limit exceeded" {
		t.Errorf("reason = %v, want %q", first["reason"], "rate limit exceeded")
	}
}

// F26-T4 row 5: confidence capture — live iff LastVerified[provider] is
// present in the fetch result, else cached (internal pick state that T9's
// evidence record consumes; asserted here via the history evidence).
func TestPickUsageConfidenceCapture(t *testing.T) {
	verifiedAt := time.Date(2026, 8, 7, 17, 3, 11, 0, time.UTC)
	t.Run("live", func(t *testing.T) {
		cfg, dir := pickPipelineSetup(t, []routing.Route{f26ClaudeRoute()}, map[string]decimal.Decimal{
			"claude-sonnet-4-5": decimal.NewFromInt(92),
		}, nil,
			func(_ context.Context, _ []string, _ pickFetchOptions) (map[string]*usageSnapshot, map[string]timeValue, error) {
				return map[string]*usageSnapshot{
					"claude": {Provider: "claude", UsageKnown: true},
				}, map[string]timeValue{"claude": verifiedAt}, nil
			}, nil)

		err, _, _ := runPick(t, PickArgs{Profile: "complex_implementation", Strategy: "score", ConfigPath: cfg})
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		entry := readHistoryFile(t, dir)[0]
		if entry.Evidence.Confidence != "live" {
			t.Errorf("confidence = %q, want live", entry.Evidence.Confidence)
		}
	})
	t.Run("cached", func(t *testing.T) {
		cfg, dir := pickPipelineSetup(t, []routing.Route{f26ClaudeRoute()}, map[string]decimal.Decimal{
			"claude-sonnet-4-5": decimal.NewFromInt(92),
		}, nil,
			func(_ context.Context, _ []string, _ pickFetchOptions) (map[string]*usageSnapshot, map[string]timeValue, error) {
				return map[string]*usageSnapshot{
					"claude": {Provider: "claude", UsageKnown: true},
				}, map[string]timeValue{}, nil
			}, nil)

		err, _, _ := runPick(t, PickArgs{Profile: "complex_implementation", Strategy: "score", ConfigPath: cfg})
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		entry := readHistoryFile(t, dir)[0]
		if entry.Evidence.Confidence != "cached" {
			t.Errorf("confidence = %q, want cached", entry.Evidence.Confidence)
		}
	})
}

// F26-T4 row 6: a band.Evaluate error excludes the candidate with
// provider_error and the error text as the reason.
func TestPickUsageBandError(t *testing.T) {
	cfg, _ := pickPipelineSetup(t, pickTwoRoutes(), pickTwoScores(), nil, nil,
		func(snap *usage.Snapshot, _ string, _ *config.Config) (bandResult, error) {
			if snap.Provider == "claude" {
				return bandResult{}, errors.New("band boom")
			}
			return bandResult{Name: "five hour", UsedPercent: 10, Weight: 1}, nil
		})

	err, out, _ := runPick(t, PickArgs{Profile: "complex_implementation", Strategy: "score", ConfigPath: cfg})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	first := pickJSON(t, out.String())["excluded_candidates"].([]any)[0].(map[string]any)
	if first["reason_code"] != "provider_error" {
		t.Errorf("reason_code = %v, want provider_error", first["reason_code"])
	}
	if first["reason"] != "band boom" {
		t.Errorf("reason = %v, want %q", first["reason"], "band boom")
	}
}

// F26-T4 extra: a fetch-level error (outside the per-provider failure
// class) is a runtime CodedError (exit 1) with the error text.
func TestPickUsageFetchError(t *testing.T) {
	cfg, _ := pickPipelineSetup(t, []routing.Route{f26ClaudeRoute()}, map[string]decimal.Decimal{
		"claude-sonnet-4-5": decimal.NewFromInt(92),
	}, nil,
		func(_ context.Context, _ []string, _ pickFetchOptions) (map[string]*usageSnapshot, map[string]timeValue, error) {
			return nil, nil, errors.New("fetch boom")
		}, nil)

	err, _, _ := runPick(t, PickArgs{Profile: "complex_implementation", Strategy: "score", ConfigPath: cfg})
	var ce *CodedError
	if !errors.As(err, &ce) || ce.Code != "runtime" {
		t.Fatalf("err = %v, want CodedError runtime", err)
	}
	if ce.Message != "fetch boom" {
		t.Errorf("message = %q, want %q", ce.Message, "fetch boom")
	}
	if ExitCodeFor(err) != 1 {
		t.Errorf("exit = %d, want 1", ExitCodeFor(err))
	}
}
