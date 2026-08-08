package strategy

import "github.com/WD-Mitchell/which-model/internal/pick"

// CostOptimal picks the cheapest candidate (max cost score) among those
// within State.Config's score-drop threshold of the top FinalScore
// (specs/features/F20-strategies/SPEC.md §2.7, D4, D15, D16).
type CostOptimal struct{}

func (CostOptimal) Name() pick.Strategy { return pick.StrategyCostOptimal }

func (CostOptimal) Pick(candidates []pick.Candidate, state *State) (pick.Candidate, []pick.Candidate, error) {
	if len(candidates) == 0 {
		return pick.Candidate{}, nil, ErrNoCandidates
	}

	threshold := state.Config.ResolvedCostMaxScoreDrop()
	top := candidates[0].FinalScore
	for _, c := range candidates[1:] {
		if c.FinalScore.Cmp(top) > 0 {
			top = c.FinalScore
		}
	}
	limit := top.Sub(threshold)

	sorted := sortByScoreThenRouteKey(candidates)
	var pool []int
	for i, c := range sorted {
		if c.FinalScore.Cmp(limit) < 0 {
			continue
		}
		if _, ok := state.CostScoreByRouteKey[RouteKey(c)]; !ok {
			continue
		}
		pool = append(pool, i)
	}

	if len(pool) == 0 {
		return pick.Candidate{}, sortByRouteKey(sorted), ErrNoCandidates
	}

	best := pool[0]
	bestCost := state.CostScoreByRouteKey[RouteKey(sorted[best])]
	for _, i := range pool[1:] {
		cost := state.CostScoreByRouteKey[RouteKey(sorted[i])]
		if cost.Cmp(bestCost) > 0 {
			best, bestCost = i, cost
		}
	}

	picked, excluded := splitPick(sorted, best)
	return picked, excluded, nil
}
