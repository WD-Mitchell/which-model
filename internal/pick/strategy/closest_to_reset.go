package strategy

import "github.com/WD-Mitchell/which-model/internal/pick"

// ClosestToReset picks the candidate whose provider has the earliest reset,
// ties broken by final score and route key.
type ClosestToReset struct{}

func (ClosestToReset) Name() pick.Strategy { return pick.StrategyClosestToReset }

func (ClosestToReset) Pick(candidates []pick.Candidate, state *State) (pick.Candidate, []pick.Candidate, error) {
	if len(candidates) == 0 {
		return pick.Candidate{}, nil, ErrNoCandidates
	}
	if !state.UsageEnabled {
		return pick.Candidate{}, nil, &ErrClosestToResetRequiresUsage{Reason: state.UsageDisabledReason}
	}

	sorted := sortByScoreThenRouteKey(candidates)
	resets := make([]int64, len(sorted))
	for i, candidate := range sorted {
		reset, ok := state.ResetAtByProvider[candidate.Route.Provider]
		if !ok || reset.IsZero() {
			return pick.Candidate{}, nil, &ErrMissingReset{Provider: candidate.Route.Provider}
		}
		resets[i] = reset.UnixNano()
	}

	minIdx := 0
	for i := 1; i < len(sorted); i++ {
		if resets[i] < resets[minIdx] {
			minIdx = i
		}
	}
	picked, excluded := splitPick(sorted, minIdx)
	return picked, excluded, nil
}
