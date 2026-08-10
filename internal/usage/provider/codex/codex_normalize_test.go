//go:build !nousage

package codex

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

var reset2030 = mustParse2030()

func mustParse2030() *time.Time {
	ts, err := time.Parse(time.RFC3339, "2030-03-17T17:46:40Z")
	if err != nil {
		panic(err)
	}
	return &ts
}

func f64(v float64) *float64 { return &v }

// TestNormalizeUsage ports the normalizeCodexUsage table from F16-T4
// (codex.mjs:63-81 plus annex-a §3.1).
func TestNormalizeUsage(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    []usage.Window
		wantErr string
	}{
		{
			name:  "primary window basic (mjs case 6 fixture)",
			input: `{"rate_limit":{"primary_window":{"used_percent":20,"reset_at":1900000000}}}`,
			want: []usage.Window{
				{ID: "5h", Label: "primary window", Unit: usage.UnitPercent, UsedPercent: f64(20), ResetsAt: reset2030, UsageKnown: true},
			},
		},
		{
			name:  "camel secondary",
			input: `{"rateLimit":{"secondaryWindow":{"usedPercent":33,"resetAt":1900000000}}}`,
			want: []usage.Window{
				{ID: "weekly", Label: "secondary window", Unit: usage.UnitPercent, UsedPercent: f64(33), ResetsAt: reset2030, UsageKnown: true},
			},
		},
		{
			name:  "both windows in order",
			input: `{"rate_limit":{"primary_window":{"used_percent":20},"secondary_window":{"used_percent":50}}}`,
			want: []usage.Window{
				{ID: "5h", Label: "primary window", Unit: usage.UnitPercent, UsedPercent: f64(20), UsageKnown: true},
				{ID: "weekly", Label: "secondary window", Unit: usage.UnitPercent, UsedPercent: f64(50), UsageKnown: true},
			},
		},
		{
			name:  "top-level fallback",
			input: `{"primary_window":{"used_percent":20}}`,
			want: []usage.Window{
				{ID: "5h", Label: "primary window", Unit: usage.UnitPercent, UsedPercent: f64(20), UsageKnown: true},
			},
		},
		{
			name:  "credits balance",
			input: `{"rate_limit":{"primary_window":{"used_percent":20}},"credits":{"balance":12.5}}`,
			want: []usage.Window{
				{ID: "5h", Label: "primary window", Unit: usage.UnitPercent, UsedPercent: f64(20), UsageKnown: true},
				{ID: "credits", Label: "credits", Unit: usage.UnitCredits, Remaining: f64(12.5), UsageKnown: true},
			},
		},
		{
			name:  "limit_window_seconds to WindowMinutes",
			input: `{"rate_limit":{"primary_window":{"used_percent":20},"limit_window_seconds":18000}}`,
			want: []usage.Window{
				{ID: "5h", Label: "primary window", Unit: usage.UnitPercent, UsedPercent: f64(20), WindowMinutes: intPtr(300), UsageKnown: true},
			},
		},
		{
			name:  "additional rate limit",
			input: `{"rate_limit":{"primary_window":{"used_percent":20}},"additional_rate_limits":[{"limit_name":"o1-mini-weekly","metered_feature":"o1-mini","rate_limit":{"primary_window":{"used_percent":55,"reset_at":1900000000},"limit_window_seconds":604800}}]}`,
			want: []usage.Window{
				{ID: "5h", Label: "primary window", Unit: usage.UnitPercent, UsedPercent: f64(20), UsageKnown: true},
				{ID: "additional:o1-mini-weekly", Label: "o1-mini-weekly", Unit: usage.UnitPercent, UsedPercent: f64(55), ModelScope: []string{"o1-mini"}, WindowMinutes: intPtr(10080), ResetsAt: reset2030, UsageKnown: true},
			},
		},
		{
			name:    "percent out of range",
			input:   `{"rate_limit":{"primary_window":{"used_percent":150}}}`,
			wantErr: "unsupported_response: Codex returned an unsupported usage shape.",
		},
		{
			name:    "credits balance not a number",
			input:   `{"credits":{"balance":"not-a-number"}}`,
			wantErr: "unsupported_response: Codex returned an unsupported usage shape.",
		},
		{
			name:    "null primary window",
			input:   `{"rate_limit":{"primary_window":null}}`,
			wantErr: "unsupported_response: Codex returned an unsupported usage shape.",
		},
		{
			name:    "not json",
			input:   `not json`,
			wantErr: "response_json: The provider returned unsupported JSON.",
		},
		{
			name:    "empty object",
			input:   `{}`,
			wantErr: "unsupported_response: Codex returned an unsupported usage shape.",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			windows, err := NormalizeUsage([]byte(tc.input))
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("NormalizeUsage() = %v, want error %q", windows, tc.wantErr)
				}
				if got := err.Error(); got != tc.wantErr {
					t.Errorf("error = %q, want %q", got, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizeUsage() error: %v", err)
			}
			if len(windows) != len(tc.want) {
				t.Fatalf("len(windows) = %d, want %d: %+v", len(windows), len(tc.want), windows)
			}
			for i := range tc.want {
				got, want := windows[i], tc.want[i]
				if got.ID != want.ID || got.Label != want.Label || got.Unit != want.Unit {
					t.Errorf("windows[%d] = %+v, want %+v", i, got, want)
					continue
				}
				if (got.UsedPercent == nil) != (want.UsedPercent == nil) ||
					(got.UsedPercent != nil && *got.UsedPercent != *want.UsedPercent) {
					t.Errorf("windows[%d].UsedPercent = %v, want %v", i, got.UsedPercent, want.UsedPercent)
				}
				if (got.Remaining == nil) != (want.Remaining == nil) ||
					(got.Remaining != nil && *got.Remaining != *want.Remaining) {
					t.Errorf("windows[%d].Remaining = %v, want %v", i, got.Remaining, want.Remaining)
				}
				if (got.WindowMinutes == nil) != (want.WindowMinutes == nil) ||
					(got.WindowMinutes != nil && *got.WindowMinutes != *want.WindowMinutes) {
					t.Errorf("windows[%d].WindowMinutes = %v, want %v", i, got.WindowMinutes, want.WindowMinutes)
				}
				if (got.ResetsAt == nil) != (want.ResetsAt == nil) ||
					(got.ResetsAt != nil && !got.ResetsAt.Equal(*want.ResetsAt)) {
					t.Errorf("windows[%d].ResetsAt = %v, want %v", i, got.ResetsAt, want.ResetsAt)
				}
				if len(got.ModelScope) != len(want.ModelScope) {
					t.Errorf("windows[%d].ModelScope = %v, want %v", i, got.ModelScope, want.ModelScope)
				} else {
					for j := range want.ModelScope {
						if got.ModelScope[j] != want.ModelScope[j] {
							t.Errorf("windows[%d].ModelScope = %v, want %v", i, got.ModelScope, want.ModelScope)
						}
					}
				}
				if got.UsageKnown != want.UsageKnown {
					t.Errorf("windows[%d].UsageKnown = %v, want %v", i, got.UsageKnown, want.UsageKnown)
				}
			}
		})
	}
}

func intPtr(v int) *int { return &v }

// jsonRaw parses s as a raw JSON value (for the private helper tests).
func jsonRaw(t *testing.T, s string) json.RawMessage {
	t.Helper()
	var raw json.RawMessage
	if err := json.Unmarshal([]byte(s), &raw); err != nil {
		t.Fatalf("jsonRaw(%q): %v", s, err)
	}
	return raw
}

// TestResetTime pins the resetText port (core.mjs:212-224): epoch seconds,
// millisecond epochs (> 10_000_000_000), ISO strings, and invalid input.
func TestResetTime(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string // RFC3339 or "" for nil
	}{
		{name: "epoch seconds", in: `1900000000`, want: "2030-03-17T17:46:40Z"},
		{name: "epoch milliseconds", in: `1900000000000`, want: "2030-03-17T17:46:40Z"},
		{name: "iso string", in: `"2030-03-17T17:46:40Z"`, want: "2030-03-17T17:46:40Z"},
		{name: "null", in: `null`, want: ""},
		{name: "zero", in: `0`, want: ""},
		{name: "invalid string", in: `"not-a-date"`, want: ""},
		{name: "non-number", in: `"oops"`, want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resetTime(jsonRaw(t, tc.in))
			if tc.want == "" {
				if got != nil {
					t.Errorf("resetTime(%s) = %v, want nil", tc.in, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("resetTime(%s) = nil, want %s", tc.in, tc.want)
			}
			if got.UTC().Format(time.RFC3339) != tc.want {
				t.Errorf("resetTime(%s) = %s, want %s", tc.in, got.UTC().Format(time.RFC3339), tc.want)
			}
		})
	}
}

// TestSlug pins the slug port: lowercase, runs of non-alphanumerics -> "-",
// trimmed.
func TestSlug(t *testing.T) {
	cases := []struct{ in, want string }{
		{"o1-mini-weekly", "o1-mini-weekly"},
		{"O1 Mini Weekly", "o1-mini-weekly"},
		{"  --hello__world..", "hello-world"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := slug(tc.in); got != tc.want {
			t.Errorf("slug(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
