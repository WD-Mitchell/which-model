// F26-T6: strategy application
// (specs/features/F26-cmd-pick/TASKS.md T6; SPEC §2.2g;
// CONTRACTS §8.5).
package whichmodel

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/WD-Mitchell/which-model/internal/config"
	"github.com/WD-Mitchell/which-model/internal/usage"
)

// An omitted strategy resolves to closest-to-reset when usage is enabled.
func TestPickStrategyDefaultWithUsage(t *testing.T) {
	var gotName string
	cfg, _ := pickPipelineSetup(t, pickTwoRoutes(), pickTwoScores(), nil, nil, nil)
	setStrategyApply(t, func(name string, cands []Candidate, _ strategyOptions) ([]Candidate, error) {
		gotName = name
		return cands, nil
	})

	err, out, _ := runPick(t, PickArgs{Profile: "complex_implementation", ConfigPath: cfg})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if gotName != "closest-to-reset" {
		t.Errorf("strategy seam received name %q, want closest-to-reset", gotName)
	}

	doc := pickJSON(t, out.String())
	if doc["strategy"] != "closest-to-reset" {
		t.Errorf("strategy = %v, want closest-to-reset", doc["strategy"])
	}
	if _, ok := doc["seed"]; ok {
		t.Error("removed seed field is present")
	}
}

// An omitted strategy resolves to priority when usage is disabled.
func TestPickStrategyDefaultWithoutUsage(t *testing.T) {
	var gotName string
	cfg, _ := pickPipelineSetup(t, pickTwoRoutes(), pickTwoScores(),
		func(_ bool, _ *config.Config) (bool, string) { return false, "flag" }, nil, nil)
	setStrategyApply(t, func(name string, cands []Candidate, _ strategyOptions) ([]Candidate, error) {
		gotName = name
		return cands, nil
	})

	err, out, _ := runPick(t, PickArgs{Profile: "complex_implementation", ConfigPath: cfg})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if gotName != "priority" {
		t.Errorf("strategy seam received name %q, want priority", gotName)
	}
	if got := pickJSON(t, out.String())["strategy"]; got != "priority" {
		t.Errorf("strategy = %v, want priority", got)
	}
}

func TestPickStrategyConfiguredDefaultWithFullSection(t *testing.T) {
	var gotName string
	cfg, _ := pickPipelineSetup(t, pickTwoRoutes(), pickTwoScores(), nil, nil, nil)
	body := `[strategy]
default = "priority"
default_profile = "review"
tier1_share = 80
tier2_share = 20
`
	if err := os.WriteFile(cfg, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	setStrategyApply(t, func(name string, cands []Candidate, _ strategyOptions) ([]Candidate, error) {
		gotName = name
		return cands, nil
	})

	err, out, _ := runPick(t, PickArgs{Profile: "complex_implementation", ConfigPath: cfg})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if gotName != "priority" {
		t.Errorf("strategy seam received name %q, want configured priority", gotName)
	}
	if got := pickJSON(t, out.String())["strategy"]; got != "priority" {
		t.Errorf("strategy = %v, want configured priority", got)
	}
}

func TestPickStrategyEnvironmentDefault(t *testing.T) {
	t.Setenv("WHICH_MODEL_STRATEGY_DEFAULT", "priority")
	var gotName string
	cfg, _ := pickPipelineSetup(t, pickTwoRoutes(), pickTwoScores(), nil, nil, nil)
	setStrategyApply(t, func(name string, cands []Candidate, _ strategyOptions) ([]Candidate, error) {
		gotName = name
		return cands, nil
	})

	err, _, _ := runPick(t, PickArgs{Profile: "complex_implementation", ConfigPath: cfg})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if gotName != "priority" {
		t.Errorf("strategy seam received name %q, want environment priority", gotName)
	}
}

func TestPickStrategyExplicitFlagOverridesConfiguredDefault(t *testing.T) {
	var gotName string
	cfg, _ := pickPipelineSetup(t, pickTwoRoutes(), pickTwoScores(), nil, nil, nil)
	body := `[strategy]
default = "round-robin"
default_profile = "review"
tier1_share = 80
tier2_share = 20
`
	if err := os.WriteFile(cfg, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	setStrategyApply(t, func(name string, cands []Candidate, _ strategyOptions) ([]Candidate, error) {
		gotName = name
		return cands, nil
	})

	err, _, _ := runPick(t, PickArgs{Profile: "complex_implementation", Strategy: "priority", ConfigPath: cfg})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if gotName != "priority" {
		t.Errorf("strategy seam received name %q, want explicit priority", gotName)
	}
}

func TestPickStrategyReceivesEarliestProviderReset(t *testing.T) {
	claudeLater := time.Date(2026, 8, 10, 14, 0, 0, 0, time.UTC)
	claudeSooner := time.Date(2026, 8, 10, 13, 0, 0, 0, time.UTC)
	codexReset := time.Date(2026, 8, 10, 12, 30, 0, 0, time.UTC)
	cfg, _ := pickPipelineSetup(t, pickTwoRoutes(), pickTwoScores(), nil,
		func(_ context.Context, _ []string, _ pickFetchOptions) (map[string]*usageSnapshot, map[string]timeValue, error) {
			return map[string]*usageSnapshot{
				"claude": {Provider: "claude", Windows: []usage.Window{{ResetsAt: &claudeLater}, {ResetsAt: &claudeSooner}}},
				"codex":  {Provider: "codex", Windows: []usage.Window{{ResetsAt: &codexReset}}},
			}, nil, nil
		}, nil)
	setStrategyApply(t, func(_ string, cands []Candidate, _ strategyOptions) ([]Candidate, error) {
		if got := pickRun.resetAtByProvider["claude"]; !got.Equal(claudeSooner) {
			t.Errorf("claude reset = %v, want %v", got, claudeSooner)
		}
		if got := pickRun.resetAtByProvider["codex"]; !got.Equal(codexReset) {
			t.Errorf("codex reset = %v, want %v", got, codexReset)
		}
		return cands, nil
	})

	if err, _, _ := runPick(t, PickArgs{Profile: "complex_implementation", Strategy: "closest-to-reset", ConfigPath: cfg}); err != nil {
		t.Fatal(err)
	}
}

// F26-T6 row 3: the fake Apply errors → *CodedError{Code: "runtime"}
// (exit 1) with the error text, and nothing is emitted on stdout.
func TestPickStrategyApplyError(t *testing.T) {
	cfg, _ := pickPipelineSetup(t, pickTwoRoutes(), pickTwoScores(), nil, nil, nil)
	setStrategyApply(t, func(name string, cands []Candidate, opts strategyOptions) ([]Candidate, error) {
		return nil, errors.New("boom")
	})

	err, out, _ := runPick(t, PickArgs{Profile: "complex_implementation", Strategy: "priority", ConfigPath: cfg})
	var ce *CodedError
	if !errors.As(err, &ce) {
		t.Fatalf("err = %v, want *CodedError", err)
	}
	if ce.Code != "runtime" {
		t.Errorf("code = %q, want runtime", ce.Code)
	}
	if !strings.Contains(ce.Message, "boom") {
		t.Errorf("message = %q, want it to contain boom", ce.Message)
	}
	if ExitCodeFor(err) != 1 {
		t.Errorf("exit = %d, want 1", ExitCodeFor(err))
	}
	if out.String() != "" {
		t.Errorf("stdout = %q, want empty", out.String())
	}
}

// F26-T6 row 4: the fake Apply returns no survivors → *CodedError{Code:
// "no_pick"} (exit 3; classification refinement lands in T7) and nothing
// is emitted on stdout.
func TestPickStrategyApplyEmpty(t *testing.T) {
	cfg, _ := pickPipelineSetup(t, pickTwoRoutes(), pickTwoScores(), nil, nil, nil)
	setStrategyApply(t, func(name string, cands []Candidate, opts strategyOptions) ([]Candidate, error) {
		return []Candidate{}, nil
	})

	err, out, _ := runPick(t, PickArgs{Profile: "complex_implementation", Strategy: "priority", ConfigPath: cfg})
	var ce *CodedError
	if !errors.As(err, &ce) {
		t.Fatalf("err = %v, want *CodedError", err)
	}
	if ce.Code != "no_pick" {
		t.Errorf("code = %q, want no_pick", ce.Code)
	}
	if ExitCodeFor(err) != 3 {
		t.Errorf("exit = %d, want 3", ExitCodeFor(err))
	}
	if out.String() != "" {
		t.Errorf("stdout = %q, want empty", out.String())
	}
}
