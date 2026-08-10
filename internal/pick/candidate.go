package pick

import "github.com/shopspring/decimal"

import "github.com/WD-Mitchell/which-model/internal/routing"

// Candidate is one selectable model+provider pair after the join and band
// stages (specs/global/CONTRACTS.md §4.1; placement recorded in
// specs/DEFERRED.md D11).
type Candidate struct {
	Route          routing.Route
	ModelScore     decimal.Decimal
	Band           string          // band name, omitted when usage disabled
	BandWeight     decimal.Decimal // 1.0 when usage disabled
	ProviderWeight decimal.Decimal
	FinalScore     decimal.Decimal
	Warnings       []string
}

// Strategy selects how a pick chooses among candidates
// (specs/global/CONTRACTS.md §4.2).
type Strategy string

const (
	StrategyPriority       Strategy = "priority"
	StrategyRoundRobin     Strategy = "round-robin"
	StrategyLeastUsed      Strategy = "least-used"
	StrategyMostUsed       Strategy = "most-used"
	StrategyClosestToReset Strategy = "closest-to-reset"
)
