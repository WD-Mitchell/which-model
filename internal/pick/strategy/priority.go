package strategy

import (
	"sort"

	"github.com/WD-Mitchell/which-model/internal/pick"
)

// Priority picks the first candidate belonging to the highest-priority
// provider that has one, using Score's tie-break within that provider
// (specs/features/F20-strategies/SPEC.md §2.3, D9).
type Priority struct{}

func (Priority) Name() pick.Strategy { return pick.StrategyPriority }

func (Priority) Pick(candidates []pick.Candidate, state *State) (pick.Candidate, []pick.Candidate, error) {
	if len(candidates) == 0 {
		return pick.Candidate{}, nil, ErrNoCandidates
	}

	present := make(map[string]bool)
	for _, c := range candidates {
		present[c.Route.Provider] = true
	}

	listed := make(map[string]bool, len(state.ProviderPriority))
	order := make([]string, 0, len(present))
	for _, p := range state.ProviderPriority {
		listed[p] = true
		order = append(order, p)
	}
	var unlisted []string
	for p := range present {
		if !listed[p] {
			unlisted = append(unlisted, p)
		}
	}
	sort.Strings(unlisted)
	order = append(order, unlisted...)

	sorted := sortByScoreThenRouteKey(candidates)
	for _, provider := range order {
		for i, c := range sorted {
			if c.Route.Provider == provider {
				picked, excluded := splitPick(sorted, i)
				return picked, excluded, nil
			}
		}
	}
	return pick.Candidate{}, nil, ErrNoCandidates
}
