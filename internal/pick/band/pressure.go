// Package band turns a provider's usage Snapshot and a route's gating
// window set into pressure, then maps pressure onto a configurable band
// ladder and enforces the hard quota gate (specs/features/F19-bands/SPEC.md).
package band

import (
	"github.com/WD-Mitchell/which-model/internal/usage"
	"github.com/shopspring/decimal"
)

// Pressure is the single scalar describing how constrained a route's
// provider is. Known == false means the usage is unmeasurable (SPEC §2.3).
type Pressure struct {
	Known   bool
	Percent decimal.Decimal // meaningful only when Known; may exceed 100
}

// WindowPercent derives the percent used for ONE window, in the priority
// chain (SPEC §2.2):
//  1. Synthetic        -> unknown (placeholder, not a real lane)
//  2. Unlimited        -> 0, known
//  3. UsageKnown false -> unknown (reset metadata, no usage number)
//  4. UsedPercent set  -> as reported (may exceed 100)
//  5. Used + Limit>0   -> Used / Limit * 100
//  6. Remaining+Limit>0-> (Limit - Remaining) / Limit * 100
//  7. otherwise        -> unknown (balance only, or non-positive Limit)
//
// Float64 conversion uses decimal.NewFromFloat (shortest-representation,
// exact for 25/50/75/100 and the default weights). No rounding here.
func WindowPercent(w usage.Window) (decimal.Decimal, bool) {
	if w.Synthetic {
		return decimal.Decimal{}, false
	}
	if w.Unlimited {
		return decimal.NewFromFloat(0), true
	}
	if !w.UsageKnown {
		return decimal.Decimal{}, false
	}
	if w.UsedPercent != nil {
		return decimal.NewFromFloat(*w.UsedPercent), true
	}
	if w.Used != nil && w.Limit != nil && *w.Limit > 0 {
		return decimal.NewFromFloat(*w.Used).Div(decimal.NewFromFloat(*w.Limit)).Mul(decimal.NewFromFloat(100)), true
	}
	if w.Remaining != nil && w.Limit != nil && *w.Limit > 0 {
		return decimal.NewFromFloat(*w.Limit).Sub(decimal.NewFromFloat(*w.Remaining)).Div(decimal.NewFromFloat(*w.Limit)).Mul(decimal.NewFromFloat(100)), true
	}
	return decimal.Decimal{}, false
}

// Pressure reduces a usage Snapshot to one scalar for a route (SPEC §2.1):
// max over the snapshot windows whose ID is in windowIDs, skipping windows
// whose WindowPercent is unknown. Windows named in windowIDs but absent from
// the snapshot contribute nothing. Returns Known == false when the snapshot
// carries Failure, when windowIDs is empty, or when no gating window yields
// a computable percent.
// NOTE: named NewPressure (constructor) because Go forbids a package-level
// func sharing the name of a type; CONTRACTS §1 lists it as the "Pressure
// constructor".
func NewPressure(snapshot usage.Snapshot, windowIDs []string) Pressure {
	if snapshot.Failure != nil {
		return Pressure{Known: false}
	}
	if len(windowIDs) == 0 {
		return Pressure{Known: false}
	}
	max := decimal.Zero
	found := false
	for _, id := range windowIDs {
		for i := range snapshot.Windows {
			if snapshot.Windows[i].ID != id {
				continue
			}
			pct, ok := WindowPercent(snapshot.Windows[i])
			if !ok {
				break
			}
			found = true
			if pct.Cmp(max) > 0 {
				max = pct
			}
			break
		}
	}
	if !found {
		return Pressure{Known: false}
	}
	return Pressure{Known: true, Percent: max}
}
