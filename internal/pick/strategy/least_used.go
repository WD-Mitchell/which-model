package strategy

import "github.com/WD-Mitchell/which-model/internal/pick"

// LeastUsed picks the candidate whose provider has the lowest usage
// pressure, ties broken by Score's rule
// (specs/features/F20-strategies/SPEC.md §2.5, D11, D12).
type LeastUsed struct{}

func (LeastUsed) Name() pick.Strategy { return pick.StrategyLeastUsed }

func (LeastUsed) Pick(candidates []pick.Candidate, state *State) (pick.Candidate, []pick.Candidate, error) {
	if len(candidates) == 0 {
		return pick.Candidate{}, nil, ErrNoCandidates
	}
	if !state.UsageEnabled {
		return pick.Candidate{}, nil, &ErrLeastUsedRequiresUsage{Reason: state.UsageDisabledReason}
	}

	sorted := sortByScoreThenRouteKey(candidates)
	pressures := make([]float64, len(sorted))
	for i, c := range sorted {
		p, ok := state.PressureByProvider[c.Route.Provider]
		if !ok {
			return pick.Candidate{}, nil, &ErrMissingPressure{Provider: c.Route.Provider}
		}
		pressures[i] = p
	}

	minIdx := 0
	for i := 1; i < len(sorted); i++ {
		if pressures[i] < pressures[minIdx] {
			minIdx = i
		}
	}
	picked, excluded := splitPick(sorted, minIdx)
	return picked, excluded, nil
}
