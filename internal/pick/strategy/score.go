package strategy

import "github.com/WD-Mitchell/which-model/internal/pick"

// Score picks the highest FinalScore, ties broken by RouteKey ascending
// (specs/features/F20-strategies/SPEC.md §2.2, D1).
type Score struct{}

func (Score) Name() pick.Strategy { return pick.StrategyScore }

// Pick ignores state: score selection needs no runtime context.
func (Score) Pick(candidates []pick.Candidate, state *State) (pick.Candidate, []pick.Candidate, error) {
	if len(candidates) == 0 {
		return pick.Candidate{}, nil, ErrNoCandidates
	}
	sorted := sortByScoreThenRouteKey(candidates)
	picked, excluded := splitPick(sorted, 0)
	return picked, excluded, nil
}
