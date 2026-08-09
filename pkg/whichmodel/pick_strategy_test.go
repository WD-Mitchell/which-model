// F26-T6: strategy application and seed wiring
// (specs/features/F26-cmd-pick/TASKS.md T6; SPEC §2.2g, §2.4;
// CONTRACTS §8.5).
package whichmodel

import (
	"errors"
	"strings"
	"testing"
)

// F26-T6 row 1: Strategy "score" — the fake Apply receives the ranked
// candidate list and Seed == nil, returns it unchanged, the result equals
// the ranked list (claude 92 first, codex 80 second), and the JSON
// document carries "seed": null.
func TestPickStrategyScoreDefault(t *testing.T) {
	var gotName string
	var gotSeed *uint64
	cfg, _ := pickPipelineSetup(t, pickTwoRoutes(), pickTwoScores(), nil, nil, nil)
	setStrategyApply(t, func(name string, cands []Candidate, opts strategyOptions) ([]Candidate, error) {
		gotName = name
		gotSeed = opts.Seed
		return cands, nil
	})

	err, out, _ := runPick(t, PickArgs{Profile: "complex_implementation", Strategy: "score", ConfigPath: cfg})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if gotName != "score" {
		t.Errorf("strategy seam received name %q, want score", gotName)
	}
	if gotSeed != nil {
		t.Errorf("strategy seam received seed %v, want nil", gotSeed)
	}

	doc := pickJSON(t, out.String())
	if doc["strategy"] != "score" {
		t.Errorf("strategy = %v, want score", doc["strategy"])
	}
	if seed, ok := doc["seed"]; ok && seed != nil {
		t.Errorf("seed = %v, want null", seed)
	}
	cands := doc["candidates"].([]any)
	if len(cands) != 2 {
		t.Fatalf("len(candidates) = %d, want 2", len(cands))
	}
	first := cands[0].(map[string]any)["route"].(map[string]any)
	second := cands[1].(map[string]any)["route"].(map[string]any)
	if first["provider"] != "claude" || second["provider"] != "codex" {
		t.Errorf("ranked order = [%v, %v], want [claude, codex]",
			first["provider"], second["provider"])
	}
}

// F26-T6 row 2: Strategy "weighted_random" + seed 7 — the fake Apply
// receives opts.Seed == 7 and reorders the candidates; the emitted top
// candidate is survivors[0] (codex) and the JSON document carries
// "seed": 7.
func TestPickStrategyWeightedRandomSeed(t *testing.T) {
	setStrategyNames(t, []string{"score", "weighted_random"})
	var gotName string
	var gotSeed *uint64
	cfg, _ := pickPipelineSetup(t, pickTwoRoutes(), pickTwoScores(), nil, nil, nil)
	setStrategyApply(t, func(name string, cands []Candidate, opts strategyOptions) ([]Candidate, error) {
		gotName = name
		gotSeed = opts.Seed
		// Reorder to prove the result follows the strategy output, not the
		// ranked input: codex (ranked second) becomes survivors[0].
		return []Candidate{cands[1], cands[0]}, nil
	})

	seed := uint64(7)
	err, out, _ := runPick(t, PickArgs{Profile: "complex_implementation", Strategy: "weighted_random", Seed: &seed, ConfigPath: cfg})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if gotName != "weighted_random" {
		t.Errorf("strategy seam received name %q, want weighted_random", gotName)
	}
	if gotSeed == nil || *gotSeed != 7 {
		t.Errorf("strategy seam received seed %v, want 7", gotSeed)
	}

	doc := pickJSON(t, out.String())
	if doc["strategy"] != "weighted_random" {
		t.Errorf("strategy = %v, want weighted_random", doc["strategy"])
	}
	if seed := doc["seed"]; seed != float64(7) {
		t.Errorf("seed = %v, want 7", seed)
	}
	cands := doc["candidates"].([]any)
	if len(cands) != 2 {
		t.Fatalf("len(candidates) = %d, want 2", len(cands))
	}
	top := cands[0].(map[string]any)["route"].(map[string]any)
	if top["provider"] != "codex" {
		t.Errorf("top candidate provider = %v, want codex (survivors[0])", top["provider"])
	}
}

// F26-T6 row 3: the fake Apply errors → *CodedError{Code: "runtime"}
// (exit 1) with the error text, and nothing is emitted on stdout.
func TestPickStrategyApplyError(t *testing.T) {
	cfg, _ := pickPipelineSetup(t, pickTwoRoutes(), pickTwoScores(), nil, nil, nil)
	setStrategyApply(t, func(name string, cands []Candidate, opts strategyOptions) ([]Candidate, error) {
		return nil, errors.New("boom")
	})

	err, out, _ := runPick(t, PickArgs{Profile: "complex_implementation", Strategy: "score", ConfigPath: cfg})
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

	err, out, _ := runPick(t, PickArgs{Profile: "complex_implementation", Strategy: "score", ConfigPath: cfg})
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
