package strategy

import (
	"fmt"

	"github.com/WD-Mitchell/which-model/internal/pick"
)

// ParseStrategy validates a CLI/config strategy string, defaulting the empty
// string to score (specs/features/F20-strategies/SPEC.md §2.8, D17).
func ParseStrategy(s string) (pick.Strategy, error) {
	if s == "" {
		return pick.StrategyScore, nil
	}
	switch pick.Strategy(s) {
	case pick.StrategyScore, pick.StrategyPriority, pick.StrategyRoundRobin,
		pick.StrategyLeastUsed, pick.StrategyWeightedRandom, pick.StrategyCostOptimal:
		return pick.Strategy(s), nil
	default:
		return "", fmt.Errorf("%w: %q", ErrUnknownStrategy, s)
	}
}

// New constructs the Strategy implementation for one of the six enum values.
func New(s pick.Strategy) (Strategy, error) {
	switch s {
	case pick.StrategyScore:
		return &Score{}, nil
	case pick.StrategyPriority:
		return &Priority{}, nil
	case pick.StrategyRoundRobin:
		return &RoundRobin{}, nil
	case pick.StrategyLeastUsed:
		return &LeastUsed{}, nil
	case pick.StrategyWeightedRandom:
		return &WeightedRandom{}, nil
	case pick.StrategyCostOptimal:
		return &CostOptimal{}, nil
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnknownStrategy, s)
	}
}
