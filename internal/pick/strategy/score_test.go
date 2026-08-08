package strategy

import (
	"errors"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/pick"
	"github.com/WD-Mitchell/which-model/internal/routing"
	"github.com/shopspring/decimal"
)

func newCandidate(provider, modelID, reasoning string, finalScore decimal.Decimal) pick.Candidate {
	return pick.Candidate{
		Route:      routing.Route{Provider: provider, ModelID: modelID, Reasoning: reasoning},
		FinalScore: finalScore,
	}
}

func score(f float64) decimal.Decimal { return decimal.NewFromFloat(f) }

func TestScorePick(t *testing.T) {
	t.Run("case 1: single candidate", func(t *testing.T) {
		c := newCandidate("codex", "gpt-5.6-sol", "max", score(88.4))
		got, excluded, err := (Score{}).Pick([]pick.Candidate{c}, nil)
		if err != nil {
			t.Fatalf("Pick() error = %v", err)
		}
		if RouteKey(got) != "codex/gpt-5.6-sol/max" {
			t.Errorf("Pick() = %+v", got)
		}
		if len(excluded) != 0 {
			t.Errorf("excluded = %+v, want empty", excluded)
		}
	})

	t.Run("case 2: higher FinalScore wins", func(t *testing.T) {
		a := newCandidate("claude", "claude-opus-4-8-20260115", "max", score(75.14))
		b := newCandidate("codex", "gpt-5.6-sol", "max", score(88.4))
		got, _, err := (Score{}).Pick([]pick.Candidate{a, b}, nil)
		if err != nil {
			t.Fatalf("Pick() error = %v", err)
		}
		if RouteKey(got) != "codex/gpt-5.6-sol/max" {
			t.Errorf("Pick() = %+v, want codex", got)
		}
	})

	t.Run("case 3: tie broken by provider ID", func(t *testing.T) {
		a := newCandidate("codex", "x", "max", score(80))
		b := newCandidate("claude", "y", "max", score(80))
		got, _, err := (Score{}).Pick([]pick.Candidate{a, b}, nil)
		if err != nil {
			t.Fatalf("Pick() error = %v", err)
		}
		if RouteKey(got) != "claude/y/max" {
			t.Errorf("Pick() = %+v, want claude/y/max", got)
		}
	})

	t.Run("case 4: tie broken by model ID", func(t *testing.T) {
		a := newCandidate("claude", "b-model", "max", score(80))
		b := newCandidate("claude", "a-model", "max", score(80))
		got, _, err := (Score{}).Pick([]pick.Candidate{a, b}, nil)
		if err != nil {
			t.Fatalf("Pick() error = %v", err)
		}
		if RouteKey(got) != "claude/a-model/max" {
			t.Errorf("Pick() = %+v, want claude/a-model/max", got)
		}
	})

	t.Run("case 5: tie broken by reasoning", func(t *testing.T) {
		a := newCandidate("claude", "opus", "max", score(80))
		b := newCandidate("claude", "opus", "high", score(80))
		got, _, err := (Score{}).Pick([]pick.Candidate{a, b}, nil)
		if err != nil {
			t.Fatalf("Pick() error = %v", err)
		}
		if RouteKey(got) != "claude/opus/high" {
			t.Errorf("Pick() = %+v, want claude/opus/high", got)
		}
	})

	t.Run("case 6: 3-way tie route key asc", func(t *testing.T) {
		a := newCandidate("codex", "c", "max", score(80))
		b := newCandidate("claude", "a", "max", score(80))
		c := newCandidate("copilot", "b", "max", score(80))
		got, _, err := (Score{}).Pick([]pick.Candidate{a, b, c}, nil)
		if err != nil {
			t.Fatalf("Pick() error = %v", err)
		}
		if RouteKey(got) != "claude/a/max" {
			t.Errorf("Pick() = %+v, want claude/a/max", got)
		}
	})

	t.Run("case 7: excluded ordering", func(t *testing.T) {
		a := newCandidate("codex", "gpt-5.6-sol", "max", score(88.4))
		b := newCandidate("claude", "claude-opus-4-8-20260115", "max", score(75.14))
		c := newCandidate("copilot", "gpt-5.6-sol", "high", score(79.2))
		got, excluded, err := (Score{}).Pick([]pick.Candidate{a, b, c}, nil)
		if err != nil {
			t.Fatalf("Pick() error = %v", err)
		}
		if RouteKey(got) != "codex/gpt-5.6-sol/max" {
			t.Errorf("Pick() = %+v, want codex", got)
		}
		if len(excluded) != 2 {
			t.Fatalf("excluded = %+v, want 2 entries", excluded)
		}
		if RouteKey(excluded[0]) != "claude/claude-opus-4-8-20260115/max" || RouteKey(excluded[1]) != "copilot/gpt-5.6-sol/high" {
			t.Errorf("excluded = %+v, want route-key ascending order", excluded)
		}
	})

	t.Run("case 8: empty slice", func(t *testing.T) {
		_, _, err := (Score{}).Pick(nil, nil)
		if !errors.Is(err, ErrNoCandidates) {
			t.Errorf("Pick() error = %v, want ErrNoCandidates", err)
		}
	})

	t.Run("case 9: duplicates no panic", func(t *testing.T) {
		a := newCandidate("claude", "opus", "max", score(80))
		b := newCandidate("claude", "opus", "max", score(80))
		got, excluded, err := (Score{}).Pick([]pick.Candidate{a, b}, nil)
		if err != nil {
			t.Fatalf("Pick() error = %v", err)
		}
		if RouteKey(got) != "claude/opus/max" {
			t.Errorf("Pick() = %+v", got)
		}
		if len(excluded) != 1 {
			t.Fatalf("excluded = %+v, want exactly 1 entry", excluded)
		}
	})

	t.Run("Name()", func(t *testing.T) {
		if (Score{}).Name() != pick.StrategyScore {
			t.Errorf("Name() = %v, want StrategyScore", (Score{}).Name())
		}
	})
}
