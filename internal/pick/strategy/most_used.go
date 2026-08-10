package strategy

import "github.com/WD-Mitchell/which-model/internal/pick"

// MostUsed picks the candidate whose provider has the highest usage pressure,
// ties broken by final score and route key.
type MostUsed struct{}

func (MostUsed) Name() pick.Strategy { return pick.StrategyMostUsed }

func (MostUsed) Pick(candidates []pick.Candidate, state *State) (pick.Candidate, []pick.Candidate, error) {
	if len(candidates) == 0 {
		return pick.Candidate{}, nil, ErrNoCandidates
	}
	if !state.UsageEnabled {
		return pick.Candidate{}, nil, &ErrMostUsedRequiresUsage{Reason: state.UsageDisabledReason}
	}

	sorted := sortByScoreThenRouteKey(candidates)
	pressures := make([]float64, len(sorted))
	for i, candidate := range sorted {
		pressure, ok := state.PressureByProvider[candidate.Route.Provider]
		if !ok {
			return pick.Candidate{}, nil, &ErrMissingPressure{Provider: candidate.Route.Provider}
		}
		pressures[i] = pressure
	}

	maxIdx := 0
	for i := 1; i < len(sorted); i++ {
		if pressures[i] > pressures[maxIdx] {
			maxIdx = i
		}
	}
	picked, excluded := splitPick(sorted, maxIdx)
	return picked, excluded, nil
}
