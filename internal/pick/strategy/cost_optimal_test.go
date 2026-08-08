package strategy

import (
	"errors"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/pick"
	"github.com/shopspring/decimal"
)

func TestCostOptimalPick(t *testing.T) {
	t.Run("case 1: max cost score within threshold", func(t *testing.T) {
		a := newCandidate("claude", "a", "max", score(90))
		b := newCandidate("codex", "b", "max", score(89))
		st := &State{CostScoreByRouteKey: map[string]decimal.Decimal{
			"claude/a/max": score(70), "codex/b/max": score(90),
		}}
		got, _, err := (CostOptimal{}).Pick([]pick.Candidate{a, b}, st)
		if err != nil {
			t.Fatalf("Pick() error = %v", err)
		}
		if RouteKey(got) != "codex/b/max" {
			t.Errorf("Pick() = %+v, want codex/b/max", got)
		}
	})

	t.Run("case 2: below threshold excluded", func(t *testing.T) {
		a := newCandidate("claude", "a", "max", score(90))
		b := newCandidate("codex", "b", "max", score(84))
		st := &State{CostScoreByRouteKey: map[string]decimal.Decimal{
			"claude/a/max": score(70), "codex/b/max": score(90),
		}}
		got, excluded, err := (CostOptimal{}).Pick([]pick.Candidate{a, b}, st)
		if err != nil {
			t.Fatalf("Pick() error = %v", err)
		}
		if RouteKey(got) != "claude/a/max" {
			t.Errorf("Pick() = %+v, want claude/a/max", got)
		}
		if len(excluded) != 1 || RouteKey(excluded[0]) != "codex/b/max" {
			t.Errorf("excluded = %+v, want codex/b/max", excluded)
		}
	})

	t.Run("case 3: missing cost entry excluded", func(t *testing.T) {
		a := newCandidate("claude", "a", "max", score(90))
		b := newCandidate("codex", "b", "max", score(89))
		st := &State{CostScoreByRouteKey: map[string]decimal.Decimal{
			"claude/a/max": score(70),
		}}
		got, excluded, err := (CostOptimal{}).Pick([]pick.Candidate{a, b}, st)
		if err != nil {
			t.Fatalf("Pick() error = %v", err)
		}
		if RouteKey(got) != "claude/a/max" {
			t.Errorf("Pick() = %+v, want claude/a/max", got)
		}
		if len(excluded) != 1 || RouteKey(excluded[0]) != "codex/b/max" {
			t.Errorf("excluded = %+v, want codex/b/max", excluded)
		}
	})

	t.Run("case 4: equal cost tie by FinalScore then route key", func(t *testing.T) {
		a := newCandidate("claude", "a", "max", score(90))
		b := newCandidate("codex", "b", "max", score(90))
		st := &State{CostScoreByRouteKey: map[string]decimal.Decimal{
			"claude/a/max": score(70), "codex/b/max": score(70),
		}}
		got, _, err := (CostOptimal{}).Pick([]pick.Candidate{a, b}, st)
		if err != nil {
			t.Fatalf("Pick() error = %v", err)
		}
		if RouteKey(got) != "claude/a/max" {
			t.Errorf("Pick() = %+v, want claude/a/max", got)
		}
	})

	t.Run("case 5: custom threshold excludes a drop-1.5 candidate", func(t *testing.T) {
		a := newCandidate("claude", "a", "max", score(90))
		b := newCandidate("codex", "b", "max", score(88.5))
		st := &State{
			Config: Config{CostMaxScoreDrop: 1},
			CostScoreByRouteKey: map[string]decimal.Decimal{
				"claude/a/max": score(50), "codex/b/max": score(50),
			},
		}
		got, _, err := (CostOptimal{}).Pick([]pick.Candidate{a, b}, st)
		if err != nil {
			t.Fatalf("Pick() error = %v", err)
		}
		if RouteKey(got) != "claude/a/max" {
			t.Errorf("Pick() = %+v, want claude/a/max", got)
		}
	})

	t.Run("case 6: default threshold 5.0 is inclusive at exactly 5", func(t *testing.T) {
		a := newCandidate("claude", "a", "max", score(90))
		b := newCandidate("codex", "b", "max", score(85))
		st := &State{CostScoreByRouteKey: map[string]decimal.Decimal{
			"claude/a/max": score(50), "codex/b/max": score(50),
		}}
		got, _, err := (CostOptimal{}).Pick([]pick.Candidate{a, b}, st)
		if err != nil {
			t.Fatalf("Pick() error = %v", err)
		}
		if RouteKey(got) != "claude/a/max" {
			t.Errorf("Pick() = %+v, want claude/a/max (tie cost, higher FinalScore wins)", got)
		}
	})

	t.Run("case 7: three-way pool picks max cost", func(t *testing.T) {
		a := newCandidate("claude", "a", "max", score(90))
		b := newCandidate("codex", "b", "max", score(89))
		c := newCandidate("copilot", "c", "max", score(88))
		st := &State{CostScoreByRouteKey: map[string]decimal.Decimal{
			"claude/a/max": score(40), "codex/b/max": score(95), "copilot/c/max": score(60),
		}}
		got, excluded, err := (CostOptimal{}).Pick([]pick.Candidate{a, b, c}, st)
		if err != nil {
			t.Fatalf("Pick() error = %v", err)
		}
		if RouteKey(got) != "codex/b/max" {
			t.Errorf("Pick() = %+v, want codex/b/max", got)
		}
		if len(excluded) != 2 {
			t.Fatalf("excluded = %+v, want 2 entries", excluded)
		}
	})

	t.Run("case 8: empty slice", func(t *testing.T) {
		_, _, err := (CostOptimal{}).Pick(nil, &State{})
		if !errors.Is(err, ErrNoCandidates) {
			t.Errorf("Pick() error = %v, want ErrNoCandidates", err)
		}
	})

	t.Run("Name()", func(t *testing.T) {
		if (CostOptimal{}).Name() != pick.StrategyCostOptimal {
			t.Errorf("Name() = %v, want StrategyCostOptimal", (CostOptimal{}).Name())
		}
	})
}
