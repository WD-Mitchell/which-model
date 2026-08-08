//go:build !nousage

package copilot

import (
	"reflect"
	"testing"
	"time"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

func TestNormalizeUsage(t *testing.T) {
	cases := []struct {
		name      string
		raw       string
		wantCode  string
		wantMsg   string
		wantCount int
	}{
		{
			// copy from usage-allowance.test.mjs case 15.
			name:      "case 1: chat window with percent + reset",
			raw:       `{"quota_snapshots":{"chat":{"entitlement":300,"remaining":225,"percent_remaining":75}},"quota_reset_date":"2030-01-01"}`,
			wantCount: 1,
		},
		{
			// copy from usage-allowance.test.mjs case 9.
			name:      "case 2: chat window without percent",
			raw:       `{"quota_snapshots":{"chat":{"remaining":10,"entitlement":20}}}`,
			wantCount: 1,
		},
		{
			name:      "case 3: all three windows in order",
			raw:       `{"quota_snapshots":{"chat":{"remaining":10},"completions":{"remaining":5},"premium_interactions":{"remaining":1,"entitlement":2}}}`,
			wantCount: 3,
		},
		{
			name:      "case 4: unlimited",
			raw:       `{"quota_snapshots":{"chat":{"unlimited":true}}}`,
			wantCount: 1,
		},
		{
			name:     "case 5: entitlement alone is not enough",
			raw:      `{"quota_snapshots":{"chat":{"entitlement":300}}}`,
			wantCode: "unsupported_response",
			wantMsg:  "GitHub Copilot returned an unsupported usage shape.",
		},
		{
			name:     "case 6: percent out of range",
			raw:      `{"quota_snapshots":{"chat":{"percent_remaining":150}}}`,
			wantCode: "unsupported_response",
			wantMsg:  "GitHub Copilot returned an unsupported usage shape.",
		},
		{
			name:     "case 7: snapshots is an array",
			raw:      `{"quota_snapshots":[]}`,
			wantCode: "unsupported_response",
			wantMsg:  "GitHub Copilot returned an unsupported usage shape.",
		},
		{
			name:     "case 8: snapshots absent",
			raw:      `{"garbage":1}`,
			wantCode: "unsupported_response",
			wantMsg:  "GitHub Copilot returned an unsupported usage shape.",
		},
		{
			name:      "case 9: per-window reset_at beats quota_reset_date",
			raw:       `{"quota_snapshots":{"chat":{"remaining":10,"reset_at":"2030-01-01T00:00:00Z"},"completions":{"remaining":5,"reset_at":"2030-01-02T00:00:00Z"}}}`,
			wantCount: 2,
		},
		{
			name:      "case 10: unparseable reset_at",
			raw:       `{"quota_snapshots":{"chat":{"remaining":10,"reset_at":"not-a-date"}}}`,
			wantCount: 1,
		},
		{
			name:     "case 11: not json",
			raw:      "not json",
			wantCode: "response_json",
			wantMsg:  "The provider returned unsupported JSON.",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			windows, err := NormalizeUsage([]byte(tc.raw))
			if tc.wantCode != "" {
				if err == nil {
					t.Fatalf("NormalizeUsage = %v, nil; want error %q", windows, tc.wantCode)
				}
				pe, ok := err.(*Error)
				if !ok {
					t.Fatalf("error type = %T, want *copilot.Error", err)
				}
				if pe.Code != tc.wantCode {
					t.Errorf("code = %q, want %q", pe.Code, tc.wantCode)
				}
				if pe.Message != tc.wantMsg {
					t.Errorf("message = %q, want %q", pe.Message, tc.wantMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizeUsage = %v", err)
			}
			if len(windows) != tc.wantCount {
				t.Fatalf("len(windows) = %d, want %d", len(windows), tc.wantCount)
			}
		})
	}
}

func TestNormalizeUsageWindows(t *testing.T) {
	// Case 1: values must match normalizeCopilotUsage (copilot.mjs) for the
	// same fixture: remaining 225, entitlement 300, percent_remaining 75 →
	// UsedPercent 25 (SPEC D7), reset 2030-01-01T00:00:00Z.
	windows, err := NormalizeUsage([]byte(`{"quota_snapshots":{"chat":{"entitlement":300,"remaining":225,"percent_remaining":75}},"quota_reset_date":"2030-01-01"}`))
	if err != nil {
		t.Fatalf("NormalizeUsage = %v", err)
	}
	used := 25.0
	remaining := 225.0
	limit := 300.0
	reset := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	want := []usage.Window{{
		ID:          "chat",
		Label:       "chat",
		Unit:        usage.UnitRequests,
		UsedPercent: &used,
		Limit:       &limit,
		Remaining:   &remaining,
		ResetsAt:    &reset,
		UsageKnown:  true,
	}}
	if !reflect.DeepEqual(windows, want) {
		t.Errorf("windows = %+v\nwant      %+v", windows, want)
	}

	// Case 2: no percent, no reset (case 9 fixture).
	windows, err = NormalizeUsage([]byte(`{"quota_snapshots":{"chat":{"remaining":10,"entitlement":20}}}`))
	if err != nil {
		t.Fatalf("NormalizeUsage = %v", err)
	}
	remaining2 := 10.0
	limit2 := 20.0
	want2 := []usage.Window{{
		ID:         "chat",
		Label:      "chat",
		Unit:       usage.UnitRequests,
		Limit:      &limit2,
		Remaining:  &remaining2,
		UsageKnown: true,
	}}
	if !reflect.DeepEqual(windows, want2) {
		t.Errorf("windows = %+v\nwant      %+v", windows, want2)
	}
}

func TestNormalizeUsageOrderAndLabels(t *testing.T) {
	// Case 3: fixed .mjs order chat, completions, premium_interactions;
	// premium label "premium interactions".
	windows, err := NormalizeUsage([]byte(`{"quota_snapshots":{"chat":{"remaining":10},"completions":{"remaining":5},"premium_interactions":{"remaining":1,"entitlement":2}}}`))
	if err != nil {
		t.Fatalf("NormalizeUsage = %v", err)
	}
	ids := []string{"chat", "completions", "premium"}
	for i, id := range ids {
		if windows[i].ID != id {
			t.Errorf("windows[%d].ID = %q, want %q", i, windows[i].ID, id)
		}
	}
	if windows[0].Label != "chat" {
		t.Errorf("windows[0].Label = %q, want %q", windows[0].Label, "chat")
	}
	if windows[1].Label != "completions" {
		t.Errorf("windows[1].Label = %q, want %q", windows[1].Label, "completions")
	}
	if windows[2].Label != "premium interactions" {
		t.Errorf("windows[2].Label = %q, want %q", windows[2].Label, "premium interactions")
	}

	// Case 4: unlimited window carries nothing else.
	windows, err = NormalizeUsage([]byte(`{"quota_snapshots":{"chat":{"unlimited":true}}}`))
	if err != nil {
		t.Fatalf("NormalizeUsage = %v", err)
	}
	if len(windows) != 1 {
		t.Fatalf("len(windows) = %d, want 1", len(windows))
	}
	w := windows[0]
	if !w.Unlimited {
		t.Errorf("Unlimited = false, want true")
	}
	if w.Remaining != nil || w.Limit != nil || w.UsedPercent != nil || w.ResetsAt != nil {
		t.Errorf("unlimited window carries unexpected readings: %+v", w)
	}
	if !w.UsageKnown {
		t.Errorf("UsageKnown = false, want true")
	}

	// Case 9: per-window reset_at beats the top-level quota_reset_date.
	windows, err = NormalizeUsage([]byte(`{"quota_snapshots":{"chat":{"remaining":10,"reset_at":"2030-01-01T00:00:00Z"},"completions":{"remaining":5,"reset_at":"2030-01-02T00:00:00Z"}}}`))
	if err != nil {
		t.Fatalf("NormalizeUsage = %v", err)
	}
	wantResets := []string{"2030-01-01T00:00:00Z", "2030-01-02T00:00:00Z"}
	for i, want := range wantResets {
		if windows[i].ResetsAt == nil {
			t.Fatalf("windows[%d].ResetsAt = nil, want %s", i, want)
		}
		if got := windows[i].ResetsAt.Format(time.RFC3339); got != want {
			t.Errorf("windows[%d].ResetsAt = %s, want %s", i, got, want)
		}
	}

	// Case 10: unparseable per-window reset_at → nil, window still present.
	windows, err = NormalizeUsage([]byte(`{"quota_snapshots":{"chat":{"remaining":10,"reset_at":"not-a-date"}}}`))
	if err != nil {
		t.Fatalf("NormalizeUsage = %v", err)
	}
	if len(windows) != 1 {
		t.Fatalf("len(windows) = %d, want 1", len(windows))
	}
	if windows[0].ResetsAt != nil {
		t.Errorf("ResetsAt = %v, want nil", windows[0].ResetsAt)
	}
}
