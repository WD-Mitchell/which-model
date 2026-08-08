package strategy

import "github.com/WD-Mitchell/which-model/internal/pick"

// RoundRobin rotates through candidates in route-key ascending order using a
// persisted, flock-protected cursor per scope
// (specs/features/F20-strategies/SPEC.md §2.4).
type RoundRobin struct{}

func (RoundRobin) Name() pick.Strategy { return pick.StrategyRoundRobin }

func (RoundRobin) Pick(candidates []pick.Candidate, state *State) (pick.Candidate, []pick.Candidate, error) {
	if len(candidates) == 0 {
		return pick.Candidate{}, nil, ErrNoCandidates
	}
	sorted := sortByRouteKey(candidates)
	keys := make([]string, len(sorted))
	for i, c := range sorted {
		keys[i] = RouteKey(c)
	}
	key := scopeKey(state.Profile, keys)
	index, err := nextCursor(state.DataDir, key, state.DryRun)
	if err != nil {
		return pick.Candidate{}, nil, err
	}

	picked := sorted[index%len(sorted)]

	excluded := make([]pick.Candidate, 0, len(sorted)-1)
	for i, c := range sorted {
		if i != index%len(sorted) {
			excluded = append(excluded, c)
		}
	}
	return picked, excluded, nil
}
