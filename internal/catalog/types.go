// Package catalog holds the canonical catalog data types shared by the
// scoring and ranking layers (specs/global/CONTRACTS.md §2.1 and §4.3;
// placement recorded in specs/DEFERRED.md D9). Leaf package: imports only
// shopspring/decimal.
package catalog

import "github.com/shopspring/decimal"

// ScoreRow is one scored model row (specs/global/CONTRACTS.md §2.1).
type ScoreRow struct {
	Model      string
	Reasoning  string                     // minimal|low|medium|high|xhigh|max|default
	Tier1      map[string]decimal.Decimal // intelligence, cost, speed
	Categories map[string]decimal.Decimal // 12 category composites
	Benchmarks map[string]decimal.Decimal // dynamic benchmark columns
}

// Profile is one ranking profile (specs/global/CONTRACTS.md §4.3; Go file
// placement per specs/features/F10-ranking/CONTRACTS.md).
type Profile struct {
	Name         string
	Tier1Share   decimal.Decimal
	Tier2Share   decimal.Decimal
	Tier1Weights map[string]decimal.Decimal
	Tier2Weights map[string]decimal.Decimal
}
