package strategy

import (
	"fmt"

	"github.com/WD-Mitchell/which-model/internal/pick"
)

// ParseStrategy validates a CLI/config strategy string, defaulting to priority.
func ParseStrategy(s string) (pick.Strategy, error) {
	if s == "" {
		return pick.StrategyPriority, nil
	}
	switch pick.Strategy(s) {
	case pick.StrategyPriority, pick.StrategyRoundRobin, pick.StrategyLeastUsed,
		pick.StrategyMostUsed, pick.StrategyClosestToReset:
		return pick.Strategy(s), nil
	default:
		return "", fmt.Errorf("%w: %q", ErrUnknownStrategy, s)
	}
}

// New constructs an implementation for a registered strategy.
func New(s pick.Strategy) (Strategy, error) {
	switch s {
	case pick.StrategyPriority:
		return &Priority{}, nil
	case pick.StrategyRoundRobin:
		return &RoundRobin{}, nil
	case pick.StrategyLeastUsed:
		return &LeastUsed{}, nil
	case pick.StrategyMostUsed:
		return &MostUsed{}, nil
	case pick.StrategyClosestToReset:
		return &ClosestToReset{}, nil
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnknownStrategy, s)
	}
}
