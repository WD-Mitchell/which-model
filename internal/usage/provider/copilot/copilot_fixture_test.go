//go:build !nousage

package copilot

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

// Golden fixtures: every fixture's field names are verbatim from the .mjs
// (copilot.mjs normalizeCopilotUsage field names / usage-allowance.test.mjs
// recorded fixtures) — no invented field names (annex-a §8 golden-file policy).
//
// Parity: for copilot_chat.json (usage-allowance.test.mjs case 15),
// normalizeCopilotUsage in usage-allowance-checks/lib/copilot.mjs yields
// {label:"chat", unlimited:false, remaining:225, entitlement:300,
// remainingPercent:75, resetAt:"2030-01-01T00:00:00.000Z"}; the Go window
// carries the same data under the canonical fields (remaining 225, limit 300,
// UsedPercent 25 per SPEC D7, reset 2030-01-01T00:00:00Z), and the F24
// renderer derives the "75% available" line from UsedPercent. copilot_minimal
// (case 9) has remaining 10 / entitlement 20 with no percent. The F17-T6
// request order [USER, USAGE] matches .mjs test 15 (asserted in
// copilot_check_test.go case 1).
func TestGoldenFixtures(t *testing.T) {
	dir := filepath.Join("testdata", "usage", "copilot")

	used25 := 25.0
	rem225 := 225.0
	lim300 := 300.0
	reset := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	rem10 := 10.0
	lim20 := 20.0
	rem5 := 5.0
	rem1 := 1.0
	lim2 := 2.0

	cases := []struct {
		name     string
		file     string
		want     []usage.Window
		wantCode string
		wantMsg  string
	}{
		{
			name: "copilot_chat.json (case 15)",
			file: "copilot_chat.json",
			want: []usage.Window{{
				ID:          "chat",
				Label:       "chat",
				Unit:        usage.UnitRequests,
				UsedPercent: &used25,
				Limit:       &lim300,
				Remaining:   &rem225,
				ResetsAt:    &reset,
				UsageKnown:  true,
			}},
		},
		{
			name: "copilot_minimal.json (case 9)",
			file: "copilot_minimal.json",
			want: []usage.Window{{
				ID:         "chat",
				Label:      "chat",
				Unit:       usage.UnitRequests,
				Limit:      &lim20,
				Remaining:  &rem10,
				UsageKnown: true,
			}},
		},
		{
			name: "copilot_unlimited.json",
			file: "copilot_unlimited.json",
			want: []usage.Window{{
				ID:         "chat",
				Label:      "chat",
				Unit:       usage.UnitRequests,
				Unlimited:  true,
				UsageKnown: true,
			}},
		},
		{
			name: "copilot_all.json (order check)",
			file: "copilot_all.json",
			want: []usage.Window{
				{
					ID:         "chat",
					Label:      "chat",
					Unit:       usage.UnitRequests,
					Limit:      &lim20,
					Remaining:  &rem10,
					UsageKnown: true,
				},
				{
					ID:         "completions",
					Label:      "completions",
					Unit:       usage.UnitRequests,
					Remaining:  &rem5,
					UsageKnown: true,
				},
				{
					ID:         "premium",
					Label:      "premium interactions",
					Unit:       usage.UnitRequests,
					Limit:      &lim2,
					Remaining:  &rem1,
					UsageKnown: true,
				},
			},
		},
		{
			name:     "copilot_unsupported.json",
			file:     "copilot_unsupported.json",
			wantCode: "unsupported_response",
			wantMsg:  "GitHub Copilot returned an unsupported usage shape.",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(dir, tc.file))
			if err != nil {
				t.Fatalf("os.ReadFile: %v", err)
			}
			windows, err := NormalizeUsage(raw)
			if tc.wantCode != "" {
				if err == nil {
					t.Fatalf("NormalizeUsage = %v, nil; want %q", windows, tc.wantCode)
				}
				pe, ok := err.(*Error)
				if !ok || pe.Code != tc.wantCode || pe.Message != tc.wantMsg {
					t.Errorf("error = %v, want %q / %q", err, tc.wantCode, tc.wantMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizeUsage = %v", err)
			}
			if len(windows) != len(tc.want) {
				t.Fatalf("len(windows) = %d, want %d", len(windows), len(tc.want))
			}
			for i := range tc.want {
				got, want := windows[i], tc.want[i]
				if got.ID != want.ID || got.Label != want.Label || got.Unit != want.Unit ||
					got.Unlimited != want.Unlimited || got.UsageKnown != want.UsageKnown {
					t.Errorf("windows[%d] = %+v, want %+v", i, got, want)
				}
				for _, p := range []struct {
					name string
					got  *float64
					want *float64
				}{
					{"UsedPercent", got.UsedPercent, want.UsedPercent},
					{"Limit", got.Limit, want.Limit},
					{"Remaining", got.Remaining, want.Remaining},
				} {
					if (p.got == nil) != (p.want == nil) {
						t.Errorf("windows[%d].%s = %v, want %v", i, p.name, p.got, p.want)
					} else if p.got != nil && *p.got != *p.want {
						t.Errorf("windows[%d].%s = %v, want %v", i, p.name, *p.got, *p.want)
					}
				}
				if (got.ResetsAt == nil) != (want.ResetsAt == nil) {
					t.Errorf("windows[%d].ResetsAt = %v, want %v", i, got.ResetsAt, want.ResetsAt)
				} else if got.ResetsAt != nil && !got.ResetsAt.Equal(*want.ResetsAt) {
					t.Errorf("windows[%d].ResetsAt = %v, want %v", i, *got.ResetsAt, *want.ResetsAt)
				}
			}
		})
	}
}
