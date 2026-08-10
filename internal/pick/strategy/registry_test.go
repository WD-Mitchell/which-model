package strategy

import (
	"errors"
	"reflect"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/pick"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name string
		s    pick.Strategy
		want any
	}{
		{"priority", pick.StrategyPriority, &Priority{}},
		{"round-robin", pick.StrategyRoundRobin, &RoundRobin{}},
		{"least-used", pick.StrategyLeastUsed, &LeastUsed{}},
		{"most-used", pick.StrategyMostUsed, &MostUsed{}},
		{"closest-to-reset", pick.StrategyClosestToReset, &ClosestToReset{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := New(tc.s)
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			if reflect.TypeOf(got) != reflect.TypeOf(tc.want) {
				t.Errorf("New() type = %T, want %T", got, tc.want)
			}
		})
	}

	t.Run("unknown and removed strategies", func(t *testing.T) {
		for _, name := range []string{"bogus", "score", "weighted-random", "cost-optimal"} {
			if _, err := New(pick.Strategy(name)); !errors.Is(err, ErrUnknownStrategy) {
				t.Errorf("New(%q) error = %v, want ErrUnknownStrategy", name, err)
			}
			if _, err := ParseStrategy(name); !errors.Is(err, ErrUnknownStrategy) {
				t.Errorf("ParseStrategy(%q) error = %v, want ErrUnknownStrategy", name, err)
			}
		}
	})

	t.Run("empty defaults to priority", func(t *testing.T) {
		got, err := ParseStrategy("")
		if err != nil || got != pick.StrategyPriority {
			t.Errorf("ParseStrategy(\"\") = (%v, %v), want (priority, nil)", got, err)
		}
	})

	t.Run("degraded availability", func(t *testing.T) {
		candidates := []pick.Candidate{
			newCandidate("claude", "a", "max", score(75)),
			newCandidate("codex", "b", "max", score(88)),
		}
		st := &State{UsageDisabledReason: "flag", DataDir: t.TempDir()}
		for _, name := range []pick.Strategy{pick.StrategyPriority, pick.StrategyRoundRobin} {
			strat, err := New(name)
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err := strat.Pick(candidates, st); err != nil {
				t.Errorf("%v.Pick() error = %v", name, err)
			}
		}
		for _, name := range []pick.Strategy{pick.StrategyLeastUsed, pick.StrategyMostUsed, pick.StrategyClosestToReset} {
			strat, err := New(name)
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err := strat.Pick(candidates, st); err == nil {
				t.Errorf("%v.Pick() unexpectedly succeeded without usage", name)
			}
		}
	})
}
