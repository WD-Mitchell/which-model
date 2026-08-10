// Package strategy implements the F20 pick strategies: pure functions that
// select one candidate from a routed, banded set
// (specs/features/F20-strategies/SPEC.md).
package strategy

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/WD-Mitchell/which-model/internal/pick"
	"github.com/WD-Mitchell/which-model/internal/routing"
)

// State carries every strategy's shared runtime context
// (specs/features/F20-strategies/CONTRACTS.md §3).
type State struct {
	Profile             string
	DataDir             string
	ProviderPriority    []string
	Config              Config
	UsageEnabled        bool
	UsageDisabledReason string
	PressureByProvider  map[string]float64
	ResetAtByProvider   map[string]time.Time
	DryRun              bool
}

// Config is the strategy-owned subset of which-model.toml's [pick] table.
type Config struct {
	Default string `toml:"default"`
}

// Strategy selects one candidate from a slice, returning the pick and the
// remaining excluded candidates.
type Strategy interface {
	Name() pick.Strategy
	Pick(candidates []pick.Candidate, state *State) (pick.Candidate, []pick.Candidate, error)
}

// RouteKey is the canonical sort/lookup key for a Candidate:
// "<provider>/<model_id>/<reasoning>", never trimmed or normalized.
func RouteKey(c pick.Candidate) string {
	return c.Route.Provider + "/" + c.Route.ModelID + "/" + c.Route.Reasoning
}

// RouteKeyFromRoute is RouteKey's equivalent for a bare routing.Route.
func RouteKeyFromRoute(r routing.Route) string {
	return r.Provider + "/" + r.ModelID + "/" + r.Reasoning
}

// Sentinel errors (specs/features/F20-strategies/CONTRACTS.md §5).
var (
	ErrNoCandidates    = errors.New("no candidates to pick from")
	ErrUnknownStrategy = errors.New("unknown strategy (valid: priority, round-robin, least-used, most-used, closest-to-reset)")
)

// ErrLeastUsedRequiresUsage is returned by LeastUsed.Pick when usage is
// disabled at any level.
type ErrLeastUsedRequiresUsage struct{ Reason string }

func (e *ErrLeastUsedRequiresUsage) Error() string {
	return "least-used requires usage data; usage is disabled by " + disableSource(e.Reason)
}

// ErrMostUsedRequiresUsage is returned by MostUsed.Pick when usage is disabled.
type ErrMostUsedRequiresUsage struct{ Reason string }

func (e *ErrMostUsedRequiresUsage) Error() string {
	return "most-used requires usage data; usage is disabled by " + disableSource(e.Reason)
}

// ErrClosestToResetRequiresUsage is returned by ClosestToReset.Pick when usage is disabled.
type ErrClosestToResetRequiresUsage struct{ Reason string }

func (e *ErrClosestToResetRequiresUsage) Error() string {
	return "closest-to-reset requires usage data; usage is disabled by " + disableSource(e.Reason)
}

// ErrMissingPressure is returned by LeastUsed.Pick when a candidate's
// provider has no usage pressure reading.
type ErrMissingPressure struct{ Provider string }

func (e *ErrMissingPressure) Error() string {
	return fmt.Sprintf("no usage pressure data for provider %q", e.Provider)
}

// ErrMissingReset is returned when a candidate provider has no reset timestamp.
type ErrMissingReset struct{ Provider string }

func (e *ErrMissingReset) Error() string {
	return fmt.Sprintf("no usage reset data for provider %q", e.Provider)
}

// disableSource maps a usage-disable reason code to its human-readable
// source description.
func disableSource(reason string) string {
	switch reason {
	case "flag":
		return "--no-usage"
	case "config":
		return "[usage] enabled = false"
	case "compiled_out":
		return "nousage build"
	case "no_providers_enabled":
		return "no providers enabled"
	default:
		return reason
	}
}

// PriorityOrder returns the keys of priorities sorted by descending value,
// ties broken by ascending key.
func PriorityOrder(priorities map[string]int) []string {
	order := make([]string, 0, len(priorities))
	for k := range priorities {
		order = append(order, k)
	}
	sort.Slice(order, func(i, j int) bool {
		pi, pj := priorities[order[i]], priorities[order[j]]
		if pi != pj {
			return pi > pj
		}
		return order[i] < order[j]
	})
	return order
}

// sortByScoreThenRouteKey sorts candidates by FinalScore descending, ties
// broken by RouteKey ascending (F20 SPEC D1).
func sortByScoreThenRouteKey(candidates []pick.Candidate) []pick.Candidate {
	sorted := make([]pick.Candidate, len(candidates))
	copy(sorted, candidates)
	sort.SliceStable(sorted, func(i, j int) bool {
		cmp := sorted[i].FinalScore.Cmp(sorted[j].FinalScore)
		if cmp != 0 {
			return cmp > 0
		}
		return RouteKey(sorted[i]) < RouteKey(sorted[j])
	})
	return sorted
}

// sortByRouteKey sorts candidates by RouteKey ascending.
func sortByRouteKey(candidates []pick.Candidate) []pick.Candidate {
	sorted := make([]pick.Candidate, len(candidates))
	copy(sorted, candidates)
	sort.SliceStable(sorted, func(i, j int) bool {
		return RouteKey(sorted[i]) < RouteKey(sorted[j])
	})
	return sorted
}

// splitPick returns the candidate at index i within sorted, plus every other
// element of sorted (candidates minus exactly that one occurrence — safe for
// duplicate route keys), sorted by route key ascending
// (specs/features/F20-strategies/SPEC.md §2.1.2).
func splitPick(sorted []pick.Candidate, i int) (pick.Candidate, []pick.Candidate) {
	picked := sorted[i]
	rest := make([]pick.Candidate, 0, len(sorted)-1)
	rest = append(rest, sorted[:i]...)
	rest = append(rest, sorted[i+1:]...)
	return picked, sortByRouteKey(rest)
}
