package strategy

import (
	"math/rand/v2"

	"github.com/WD-Mitchell/which-model/internal/pick"
)

// WeightedRandom samples one candidate with probability proportional to
// BandWeight × ProviderWeight, deterministically given --seed
// (specs/features/F20-strategies/SPEC.md §2.6, D3, D13, D14).
type WeightedRandom struct{}

func (WeightedRandom) Name() pick.Strategy { return pick.StrategyWeightedRandom }

func (WeightedRandom) Pick(candidates []pick.Candidate, state *State) (pick.Candidate, []pick.Candidate, error) {
	if len(candidates) == 0 {
		return pick.Candidate{}, nil, ErrNoCandidates
	}
	if !state.HasSeed {
		return pick.Candidate{}, nil, ErrSeedRequired
	}

	sorted := sortByRouteKey(candidates)
	weights := make([]float64, len(sorted))
	total := 0.0
	for i, c := range sorted {
		w := c.BandWeight.Mul(c.ProviderWeight)
		weights[i] = w.InexactFloat64()
		total += weights[i]
	}
	if total == 0 {
		for i := range weights {
			weights[i] = 1
			total += 1
		}
	}

	rng := rand.New(rand.NewPCG(uint64(state.Seed), uint64(state.Seed)))
	draw := rng.Float64() * total

	picked := len(sorted) - 1
	cumulative := 0.0
	for i, w := range weights {
		cumulative += w
		if draw < cumulative {
			picked = i
			break
		}
	}

	result, excluded := splitPick(sorted, picked)
	return result, excluded, nil
}
