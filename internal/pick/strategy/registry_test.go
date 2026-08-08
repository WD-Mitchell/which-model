package strategy

import (
	"errors"
	"reflect"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/pick"
	"github.com/shopspring/decimal"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name string
		s    pick.Strategy
		want any
	}{
		{"case 1", pick.StrategyScore, &Score{}},
		{"case 2", pick.StrategyPriority, &Priority{}},
		{"case 3", pick.StrategyRoundRobin, &RoundRobin{}},
		{"case 4", pick.StrategyLeastUsed, &LeastUsed{}},
		{"case 5", pick.StrategyWeightedRandom, &WeightedRandom{}},
		{"case 6", pick.StrategyCostOptimal, &CostOptimal{}},
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

	t.Run("case 7: unknown strategy and ParseStrategy", func(t *testing.T) {
		_, err := New(pick.Strategy("bogus"))
		if !errors.Is(err, ErrUnknownStrategy) {
			t.Errorf("New(bogus) error = %v, want ErrUnknownStrategy", err)
		}
		got, err := ParseStrategy("")
		if err != nil || got != pick.StrategyScore {
			t.Errorf("ParseStrategy(\"\") = (%v, %v), want (score, nil)", got, err)
		}
		got, err = ParseStrategy("cost-optimal")
		if err != nil || got != pick.StrategyCostOptimal {
			t.Errorf("ParseStrategy(cost-optimal) = (%v, %v), want (cost-optimal, nil)", got, err)
		}
	})

	t.Run("case 8: degraded availability", func(t *testing.T) {
		a := newCandidate("claude", "claude-opus-4-8-20260115", "max", score(75.14))
		b := newCandidate("codex", "gpt-5.6-sol", "max", score(88.4))
		candidates := []pick.Candidate{a, b}
		st := &State{
			UsageEnabled:        false,
			UsageDisabledReason: "flag",
			DataDir:             t.TempDir(),
			HasSeed:             true,
			Seed:                7,
			CostScoreByRouteKey: map[string]decimal.Decimal{
				"claude/claude-opus-4-8-20260115/max": score(70),
				"codex/gpt-5.6-sol/max":               score(90),
			},
		}
		for _, name := range []pick.Strategy{
			pick.StrategyScore, pick.StrategyPriority, pick.StrategyRoundRobin,
			pick.StrategyWeightedRandom, pick.StrategyCostOptimal,
		} {
			strat, err := New(name)
			if err != nil {
				t.Fatalf("New(%v) error = %v", name, err)
			}
			if _, _, err := strat.Pick(candidates, st); err != nil {
				t.Errorf("%v.Pick() error = %v, want a successful degraded pick", name, err)
			}
		}
		strat, err := New(pick.StrategyLeastUsed)
		if err != nil {
			t.Fatalf("New(least-used) error = %v", err)
		}
		_, _, err = strat.Pick(candidates, st)
		var refusal *ErrLeastUsedRequiresUsage
		if !errors.As(err, &refusal) {
			t.Errorf("LeastUsed.Pick() error = %v, want ErrLeastUsedRequiresUsage", err)
		}
	})
}
