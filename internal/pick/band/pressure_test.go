package band

import (
	"testing"

	"github.com/WD-Mitchell/which-model/internal/usage"
	"github.com/shopspring/decimal"
)

// win builds a usage.Window fixture. f converts a float64 literal to a
// pointer, matching the canonical *float64 field types.
func win(usedPercent, used, limit, remaining *float64, unlimited, synthetic, usageKnown bool) usage.Window {
	return usage.Window{
		UsedPercent: usedPercent,
		Used:        used,
		Limit:       limit,
		Remaining:   remaining,
		Unlimited:   unlimited,
		Synthetic:   synthetic,
		UsageKnown:  usageKnown,
	}
}

func f(v float64) *float64 { return &v }

// TestWindowPercent locks the priority chain (SPEC §2.2, TASKS T2).
func TestWindowPercent(t *testing.T) {
	tests := []struct {
		name    string
		window  usage.Window
		wantPct decimal.Decimal
		wantOK  bool
	}{
		{
			name:    "synthetic wins over used percent",
			window:  win(f(50), nil, nil, nil, false, true, true),
			wantOK:  false,
			wantPct: decimal.Decimal{},
		},
		{
			name:    "unlimited wins over used percent",
			window:  win(f(50), nil, nil, nil, true, false, true),
			wantOK:  true,
			wantPct: decimal.NewFromFloat(0),
		},
		{
			name:    "usage known false is unknown",
			window:  win(nil, nil, nil, nil, false, false, false),
			wantOK:  false,
			wantPct: decimal.Decimal{},
		},
		{
			name:    "used percent as reported",
			window:  win(f(75.5), nil, nil, nil, false, false, true),
			wantOK:  true,
			wantPct: decimal.NewFromFloat(75.5),
		},
		{
			name:    "used percent may exceed 100",
			window:  win(f(112.3), nil, nil, nil, false, false, true),
			wantOK:  true,
			wantPct: decimal.NewFromFloat(112.3),
		},
		{
			name:    "used over limit",
			window:  win(nil, f(30), f(40), nil, false, false, true),
			wantOK:  true,
			wantPct: decimal.NewFromFloat(75),
		},
		{
			name:    "non-positive limit is unknown",
			window:  win(nil, f(30), f(0), nil, false, false, true),
			wantOK:  false,
			wantPct: decimal.Decimal{},
		},
		{
			name:    "remaining over limit",
			window:  win(nil, nil, f(40), f(10), false, false, true),
			wantOK:  true,
			wantPct: decimal.NewFromFloat(75),
		},
		{
			name:    "remaining equals limit is zero",
			window:  win(nil, nil, f(40), f(40), false, false, true),
			wantOK:  true,
			wantPct: decimal.NewFromFloat(0),
		},
		{
			name:    "balance only is unknown",
			window:  win(nil, nil, f(40), nil, false, false, true),
			wantOK:  false,
			wantPct: decimal.Decimal{},
		},
		{
			name:    "remaining without limit is unknown",
			window:  win(nil, nil, nil, f(10), false, false, true),
			wantOK:  false,
			wantPct: decimal.Decimal{},
		},
		{
			name:    "all zero window is unknown",
			window:  win(nil, nil, nil, nil, false, false, true),
			wantOK:  false,
			wantPct: decimal.Decimal{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := WindowPercent(tc.window)
			if ok != tc.wantOK {
				t.Fatalf("WindowPercent() ok = %v, want %v", ok, tc.wantOK)
			}
			if ok && !got.Equal(tc.wantPct) {
				t.Errorf("WindowPercent() = %s, want %s", got, tc.wantPct)
			}
			if !ok && !got.Equal(decimal.Decimal{}) {
				t.Errorf("WindowPercent() = %s, want zero value when unknown", got)
			}
		})
	}
}

// TestPressure locks the snapshot-to-scalar reduction: max over the route's
// gating windows, not mean (SPEC §2.1, §2.3; TASKS T3).
func TestPressure(t *testing.T) {
	// withID tags a window with the ID used in windowIDs.
	withID := func(id string, w usage.Window) usage.Window { w.ID = id; return w }
	known := func(pct float64) usage.Window { return win(f(pct), nil, nil, nil, false, false, true) }
	unknown := usage.Window{}
	synthetic := usage.Window{Synthetic: true, UsageKnown: true}
	unlimited := usage.Window{Unlimited: true, UsageKnown: true}
	failure := &usage.Failure{Code: "fetch_failed", Message: "boom"}

	tests := []struct {
		name      string
		snapshot  usage.Snapshot
		windowIDs []string
		want      Pressure
	}{
		{
			name:      "max not mean",
			snapshot:  usage.Snapshot{Windows: []usage.Window{withID("w1", known(50)), withID("w2", known(75))}},
			windowIDs: []string{"w1", "w2"},
			want:      Pressure{Known: true, Percent: decimal.NewFromFloat(75)},
		},
		{
			name:      "unknown window skipped",
			snapshot:  usage.Snapshot{Windows: []usage.Window{withID("w1", known(50)), withID("w2", unknown)}},
			windowIDs: []string{"w1", "w2"},
			want:      Pressure{Known: true, Percent: decimal.NewFromFloat(50)},
		},
		{
			name:      "empty window ids",
			snapshot:  usage.Snapshot{Windows: []usage.Window{withID("w1", known(50))}},
			windowIDs: []string{},
			want:      Pressure{Known: false},
		},
		{
			name:      "snapshot failure",
			snapshot:  usage.Snapshot{Failure: failure, Windows: []usage.Window{withID("w1", known(50))}},
			windowIDs: []string{"w1"},
			want:      Pressure{Known: false},
		},
		{
			name:      "no computable window",
			snapshot:  usage.Snapshot{Windows: []usage.Window{withID("w1", known(50))}},
			windowIDs: []string{"missing"},
			want:      Pressure{Known: false},
		},
		{
			name:      "absent window contributes nothing",
			snapshot:  usage.Snapshot{Windows: []usage.Window{withID("w1", known(50))}},
			windowIDs: []string{"missing", "w1"},
			want:      Pressure{Known: true, Percent: decimal.NewFromFloat(50)},
		},
		{
			name:      "single window",
			snapshot:  usage.Snapshot{Windows: []usage.Window{withID("w1", known(50))}},
			windowIDs: []string{"w1"},
			want:      Pressure{Known: true, Percent: decimal.NewFromFloat(50)},
		},
		{
			name:      "window ids order irrelevant",
			snapshot:  usage.Snapshot{Windows: []usage.Window{withID("w1", known(50)), withID("w2", known(75))}},
			windowIDs: []string{"w2", "w1"},
			want:      Pressure{Known: true, Percent: decimal.NewFromFloat(75)},
		},
		{
			name:      "synthetic only",
			snapshot:  usage.Snapshot{Windows: []usage.Window{withID("w1", synthetic)}},
			windowIDs: []string{"w1"},
			want:      Pressure{Known: false},
		},
		{
			name:      "known zero participates",
			snapshot:  usage.Snapshot{Windows: []usage.Window{withID("w1", unlimited), withID("w2", known(25))}},
			windowIDs: []string{"w1", "w2"},
			want:      Pressure{Known: true, Percent: decimal.NewFromFloat(25)},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := NewPressure(tc.snapshot, tc.windowIDs)
			if got.Known != tc.want.Known {
				t.Fatalf("Pressure().Known = %v, want %v", got.Known, tc.want.Known)
			}
			if got.Known && !got.Percent.Equal(tc.want.Percent) {
				t.Errorf("Pressure().Percent = %s, want %s", got.Percent, tc.want.Percent)
			}
		})
	}
}
