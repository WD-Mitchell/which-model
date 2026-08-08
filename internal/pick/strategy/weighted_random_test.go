package strategy

import (
	"errors"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/pick"
	"github.com/shopspring/decimal"
)

func weightedCandidate(provider, modelID, reasoning string, weight decimal.Decimal) pick.Candidate {
	c := newCandidate(provider, modelID, reasoning, score(50))
	c.BandWeight = weight
	c.ProviderWeight = decimal.NewFromInt(1)
	return c
}

func TestWeightedRandomPick(t *testing.T) {
	t.Run("case 1: seed required", func(t *testing.T) {
		a := weightedCandidate("claude", "x", "max", decimal.NewFromInt(1))
		b := weightedCandidate("codex", "y", "max", decimal.NewFromInt(1))
		st := &State{HasSeed: false}
		_, _, err := (WeightedRandom{}).Pick([]pick.Candidate{a, b}, st)
		if !errors.Is(err, ErrSeedRequired) {
			t.Fatalf("Pick() error = %v, want ErrSeedRequired", err)
		}
		want := "weighted-random requires --seed for reproducibility"
		if got := err.Error(); got != want {
			t.Errorf("Error() = %q, want %q", got, want)
		}
	})

	t.Run("case 2: same seed same result", func(t *testing.T) {
		a := weightedCandidate("claude", "x", "max", decimal.NewFromInt(1))
		b := weightedCandidate("codex", "y", "max", decimal.NewFromInt(1))
		st1 := &State{HasSeed: true, Seed: 42}
		got1, _, err := (WeightedRandom{}).Pick([]pick.Candidate{a, b}, st1)
		if err != nil {
			t.Fatalf("Pick() error = %v", err)
		}
		st2 := &State{HasSeed: true, Seed: 42}
		got2, _, err := (WeightedRandom{}).Pick([]pick.Candidate{a, b}, st2)
		if err != nil {
			t.Fatalf("Pick() error = %v", err)
		}
		if RouteKey(got1) != RouteKey(got2) {
			t.Errorf("Pick() with same seed differs: %+v vs %+v", got1, got2)
		}
	})

	t.Run("case 3: equal weights spread across seeds", func(t *testing.T) {
		a := weightedCandidate("claude", "x", "max", decimal.NewFromInt(1))
		b := weightedCandidate("codex", "y", "max", decimal.NewFromInt(1))
		seen := map[string]bool{}
		for seed := int64(1); seed <= 50; seed++ {
			st := &State{HasSeed: true, Seed: seed}
			got, _, err := (WeightedRandom{}).Pick([]pick.Candidate{a, b}, st)
			if err != nil {
				t.Fatalf("Pick() error = %v", err)
			}
			seen[RouteKey(got)] = true
		}
		if len(seen) != 2 {
			t.Errorf("seen = %v, want both candidates to appear", seen)
		}
	})

	t.Run("case 4: skewed weights favor the heavy candidate", func(t *testing.T) {
		a := weightedCandidate("claude", "x", "max", decimal.NewFromInt(100))
		b := weightedCandidate("codex", "y", "max", decimal.NewFromInt(1))
		countA := 0
		for seed := int64(1); seed <= 50; seed++ {
			st := &State{HasSeed: true, Seed: seed}
			got, _, err := (WeightedRandom{}).Pick([]pick.Candidate{a, b}, st)
			if err != nil {
				t.Fatalf("Pick() error = %v", err)
			}
			if got.Route.Provider == "claude" {
				countA++
			}
		}
		if countA < 45 {
			t.Errorf("countA = %d over 50 seeds, want >= 45", countA)
		}
	})

	t.Run("case 5: zero weights fall back to uniform", func(t *testing.T) {
		a := weightedCandidate("claude", "x", "max", decimal.Zero)
		b := weightedCandidate("codex", "y", "max", decimal.Zero)
		seen := map[string]bool{}
		for seed := int64(1); seed <= 50; seed++ {
			st := &State{HasSeed: true, Seed: seed}
			got, _, err := (WeightedRandom{}).Pick([]pick.Candidate{a, b}, st)
			if err != nil {
				t.Fatalf("Pick() error = %v", err)
			}
			seen[RouteKey(got)] = true
		}
		if len(seen) != 2 {
			t.Errorf("seen = %v, want both candidates to appear (uniform fallback)", seen)
		}
	})

	t.Run("case 6: single candidate always picked", func(t *testing.T) {
		a := weightedCandidate("claude", "x", "max", decimal.NewFromInt(1))
		st := &State{HasSeed: true, Seed: 99}
		got, _, err := (WeightedRandom{}).Pick([]pick.Candidate{a}, st)
		if err != nil {
			t.Fatalf("Pick() error = %v", err)
		}
		if RouteKey(got) != "claude/x/max" {
			t.Errorf("Pick() = %+v", got)
		}
	})

	t.Run("case 7: empty slice", func(t *testing.T) {
		_, _, err := (WeightedRandom{}).Pick(nil, &State{HasSeed: true, Seed: 1})
		if !errors.Is(err, ErrNoCandidates) {
			t.Errorf("Pick() error = %v, want ErrNoCandidates", err)
		}
	})

	t.Run("Name()", func(t *testing.T) {
		if (WeightedRandom{}).Name() != pick.StrategyWeightedRandom {
			t.Errorf("Name() = %v, want StrategyWeightedRandom", (WeightedRandom{}).Name())
		}
	})
}
