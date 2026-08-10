//go:build !nousage

package claude

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

func fp(v float64) *float64 { return &v }
func ip(v int) *int         { return &v }

func tm(s string) *time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return &t
}

// assertWindow checks every observable Window field against want.
func assertWindow(t *testing.T, got, want usage.Window) {
	t.Helper()
	if got.ID != want.ID {
		t.Errorf("ID = %q, want %q", got.ID, want.ID)
	}
	if got.Label != want.Label {
		t.Errorf("Label = %q, want %q", got.Label, want.Label)
	}
	if got.Unit != want.Unit {
		t.Errorf("Unit = %q, want %q", got.Unit, want.Unit)
	}
	if !ptrEq(got.UsedPercent, want.UsedPercent) {
		t.Errorf("UsedPercent = %v, want %v", got.UsedPercent, want.UsedPercent)
	}
	if !ptrEq(got.Used, want.Used) {
		t.Errorf("Used = %v, want %v", got.Used, want.Used)
	}
	if !ptrEq(got.Limit, want.Limit) {
		t.Errorf("Limit = %v, want %v", got.Limit, want.Limit)
	}
	if !ptrEq(got.Remaining, want.Remaining) {
		t.Errorf("Remaining = %v, want %v", got.Remaining, want.Remaining)
	}
	if !intPtrEq(got.WindowMinutes, want.WindowMinutes) {
		t.Errorf("WindowMinutes = %v, want %v", got.WindowMinutes, want.WindowMinutes)
	}
	if !timePtrEq(got.ResetsAt, want.ResetsAt) {
		t.Errorf("ResetsAt = %v, want %v", got.ResetsAt, want.ResetsAt)
	}
	if got.ResetHint != want.ResetHint {
		t.Errorf("ResetHint = %q, want %q", got.ResetHint, want.ResetHint)
	}
	if !stringSliceEq(got.ModelScope, want.ModelScope) {
		t.Errorf("ModelScope = %v, want %v", got.ModelScope, want.ModelScope)
	}
	if got.Synthetic != want.Synthetic {
		t.Errorf("Synthetic = %v, want %v", got.Synthetic, want.Synthetic)
	}
	if got.UsageKnown != want.UsageKnown {
		t.Errorf("UsageKnown = %v, want %v", got.UsageKnown, want.UsageKnown)
	}
}

func ptrEq(a, b *float64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func intPtrEq(a, b *int) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func timePtrEq(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Equal(*b)
}

func stringSliceEq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestNormalizeUsage(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    []usage.Window
		wantErr string // expected Error.Code
	}{
		{
			name: "case 1 basic five_hour",
			raw:  `{"five_hour":{"utilization":25,"resets_at":"2030-01-01T00:00:00Z"}}`,
			want: []usage.Window{
				{ID: "5h", Label: "five hour", Unit: usage.UnitPercent, UsedPercent: fp(25), WindowMinutes: ip(300), ResetsAt: tm("2030-01-01T00:00:00Z"), UsageKnown: true},
			},
		},
		{
			name: "case 2 used_percent and seven_day",
			raw:  `{"five_hour":{"used_percent":25},"seven_day":{"utilization":41}}`,
			want: []usage.Window{
				{ID: "5h", Label: "five hour", Unit: usage.UnitPercent, UsedPercent: fp(25), WindowMinutes: ip(300), UsageKnown: true},
				{ID: "weekly", Label: "seven day", Unit: usage.UnitPercent, UsedPercent: fp(41), WindowMinutes: ip(10080), UsageKnown: true},
			},
		},
		{
			name:    "case 3 utilization out of range",
			raw:     `{"five_hour":{"utilization":150}}`,
			wantErr: "unsupported_response",
		},
		{
			name: "case 4 null five_hour synthetic plus weekly",
			raw:  `{"five_hour":null,"seven_day":{"used_percent":41,"resets_at":"2030-01-01T00:00:00Z"}}`,
			want: []usage.Window{
				{ID: "5h", Label: "five hour", Unit: usage.UnitPercent, Synthetic: true},
				{ID: "weekly", Label: "seven day", Unit: usage.UnitPercent, UsedPercent: fp(41), WindowMinutes: ip(10080), ResetsAt: tm("2030-01-01T00:00:00Z"), UsageKnown: true},
			},
		},
		{
			name: "case 5 null five_hour only",
			raw:  `{"five_hour":null}`,
			want: []usage.Window{
				{ID: "5h", Label: "five hour", Unit: usage.UnitPercent, Synthetic: true},
			},
		},
		{
			name:    "case 6 garbage shape",
			raw:     `{"garbage":1}`,
			wantErr: "unsupported_response",
		},
		{
			name: "case 7 extraUsage",
			raw:  `{"five_hour":{"utilization":25},"extraUsage":{"isEnabled":true,"monthlyLimit":40,"usedCredits":7.5,"utilization":18.75,"currency":"USD"}}`,
			want: []usage.Window{
				{ID: "5h", Label: "five hour", Unit: usage.UnitPercent, UsedPercent: fp(25), WindowMinutes: ip(300), UsageKnown: true},
				{ID: "extra_usage", Label: "Extra usage", Unit: usage.UnitUSD, Used: fp(7.5), Limit: fp(40), UsedPercent: fp(18.75), UsageKnown: true},
			},
		},
		{
			name: "case 8 limits",
			raw:  `{"five_hour":{"utilization":25},"limits":[{"kind":"weekly","group":"sonnet","percent":41,"resetsAt":"2026-08-07T18:00:00Z","scope":{"model":{"id":"claude-sonnet-4-5","display_name":"Claude Sonnet 4.5"}},"isActive":true}]}`,
			want: []usage.Window{
				{ID: "5h", Label: "five hour", Unit: usage.UnitPercent, UsedPercent: fp(25), WindowMinutes: ip(300), UsageKnown: true},
				{ID: "limit:weekly_sonnet", Label: "sonnet", Unit: usage.UnitPercent, UsedPercent: fp(41), ResetsAt: tm("2026-08-07T18:00:00Z"), ModelScope: []string{"claude-sonnet-4-5"}, UsageKnown: true},
			},
		},
		{
			name: "case 9 unparseable resets_at",
			raw:  `{"five_hour":{"utilization":25,"resets_at":"not-a-date"}}`,
			want: []usage.Window{
				{ID: "5h", Label: "five hour", Unit: usage.UnitPercent, UsedPercent: fp(25), WindowMinutes: ip(300), UsageKnown: true},
			},
		},
		{
			name: "case 10 numeric string utilization",
			raw:  `{"five_hour":{"utilization":"25"}}`,
			want: []usage.Window{
				{ID: "5h", Label: "five hour", Unit: usage.UnitPercent, UsedPercent: fp(25), WindowMinutes: ip(300), UsageKnown: true},
			},
		},
		{
			name:    "case 11 not json",
			raw:     `not json`,
			wantErr: "response_json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeUsage([]byte(tt.raw))
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("NormalizeUsage: expected error code %q, got nil", tt.wantErr)
				}
				var e *Error
				if !errors.As(err, &e) {
					t.Fatalf("error = %T %v, want *claude.Error", err, err)
				}
				if e.Code != tt.wantErr {
					t.Fatalf("error code = %q, want %q", e.Code, tt.wantErr)
				}
				if got != nil {
					t.Errorf("windows = %v, want nil on error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizeUsage: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("windows = %d entries, want %d:\n%+v", len(got), len(tt.want), got)
			}
			for i := range tt.want {
				assertWindow(t, got[i], tt.want[i])
			}
		})
	}
}

// TestNormalizeNullFiveHourNeverZeroPercent guards SPEC D5: a null five_hour
// must never render as 0% used.
func TestNormalizeNullFiveHourNeverZeroPercent(t *testing.T) {
	got, err := NormalizeUsage([]byte(`{"five_hour":null}`))
	if err != nil {
		t.Fatalf("NormalizeUsage: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("windows = %d, want 1", len(got))
	}
	if got[0].UsedPercent != nil || got[0].Used != nil || got[0].Limit != nil || got[0].Remaining != nil {
		t.Errorf("synthetic window carries percent fields: %+v", got[0])
	}
	if !got[0].Synthetic || got[0].UsageKnown {
		t.Errorf("synthetic window flags = synthetic:%v usageKnown:%v, want true/false", got[0].Synthetic, got[0].UsageKnown)
	}
}

// TestNormalizeRoutinesTryKeys verifies the try-key chain wins in order.
func TestNormalizeRoutinesTryKeys(t *testing.T) {
	got, err := NormalizeUsage([]byte(`{"five_hour":{"utilization":25},"seven_day_routines":null,"seven_day_claude_routines":{"utilization":12}}`))
	if err != nil {
		t.Fatalf("NormalizeUsage: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("windows = %d, want 2", len(got))
	}
	w := got[1]
	if w.ID != "routines_7d" || w.Label != "seven day Routines" {
		t.Errorf("routines window = %q %q, want routines_7d / seven day Routines", w.ID, w.Label)
	}
	if !ptrEq(w.UsedPercent, fp(12)) || !intPtrEq(w.WindowMinutes, ip(10080)) {
		t.Errorf("routines window = %+v, want used 12 minutes 10080", w)
	}
}

// TestNormalizeRoutinesFallbackKeys verifies a later try-key wins when earlier
// ones are absent, and that a non-object value never wins.
func TestNormalizeRoutinesFallbackKeys(t *testing.T) {
	got, err := NormalizeUsage([]byte(`{"five_hour":{"utilization":25},"claude_routines":42,"cowork":{"utilization":9}}`))
	if err != nil {
		t.Fatalf("NormalizeUsage: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("windows = %d, want 2 (got %+v)", len(got), got)
	}
	if w := got[1]; w.ID != "routines_7d" || !ptrEq(w.UsedPercent, fp(9)) {
		t.Errorf("routines window = %+v, want routines_7d used 9 from cowork", w)
	}
}

// TestNormalizeSonnetOpusScopes pins the fixed model scopes (CONTRACTS §5).
func TestNormalizeSonnetOpusScopes(t *testing.T) {
	raw := `{"five_hour":{"utilization":1},"seven_day_sonnet":{"utilization":2},"seven_day_opus":{"utilization":3},"seven_day_oauth_apps":{"utilization":4}}`
	got, err := NormalizeUsage([]byte(raw))
	if err != nil {
		t.Fatalf("NormalizeUsage: %v", err)
	}
	want := []usage.Window{
		{ID: "5h", Label: "five hour", Unit: usage.UnitPercent, UsedPercent: fp(1), WindowMinutes: ip(300), UsageKnown: true},
		{ID: "sonnet_7d", Label: "seven day Sonnet", Unit: usage.UnitPercent, UsedPercent: fp(2), WindowMinutes: ip(10080), ModelScope: []string{"sonnet"}, UsageKnown: true},
		{ID: "opus_7d", Label: "seven day Opus", Unit: usage.UnitPercent, UsedPercent: fp(3), WindowMinutes: ip(10080), ModelScope: []string{"opus"}, UsageKnown: true},
		{ID: "oauth_apps_7d", Label: "seven day OAuth apps", Unit: usage.UnitPercent, UsedPercent: fp(4), WindowMinutes: ip(10080), UsageKnown: true},
	}
	if len(got) != len(want) {
		t.Fatalf("windows = %d, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		assertWindow(t, got[i], want[i])
	}
}

// TestNormalizeUnsupportedMessage pins the exact failure message.
func TestNormalizeUnsupportedMessage(t *testing.T) {
	_, err := NormalizeUsage([]byte(`{"garbage":1}`))
	if err == nil || !strings.Contains(err.Error(), "Claude returned an unsupported usage shape.") {
		t.Errorf("error = %v, want message %q", err, "Claude returned an unsupported usage shape.")
	}
}
