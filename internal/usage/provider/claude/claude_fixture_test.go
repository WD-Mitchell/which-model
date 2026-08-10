//go:build !nousage

package claude

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

// testdataDir is the golden-fixture root (annex-a §8 golden-file policy).
const testdataDir = "testdata/usage/claude"

// readFixture loads one golden response fixture.
func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(testdataDir, name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return data
}

// TestGoldenFixtures runs the five recorded/constructed response shapes
// through NormalizeUsage and asserts the exact normalized windows
// (F15-T7 step 3). Fixture provenance:
//
//   - oauth_basic.json: copied verbatim from
//     usage-allowance-checks/tests/usage-allowance.test.mjs case 1.
//   - oauth_synthetic_5h.json: constructed from SPEC D5 (no .mjs case exists).
//   - oauth_unsupported.json: constructed (fail-closed shape).
//   - oauth_extra_usage.json: constructed from annex-a §3.2/survey:136-143
//     field names.
//   - oauth_limits.json: constructed from annex-a §3.2/survey:136-143.
//
// Parity note: for oauth_basic.json, normalizeClaudeUsage in
// usage-allowance-checks/lib/claude.mjs yields {label:"five hour",
// usedPercent:25, remainingPercent:75, resetAt:"2030-01-01T00:00:00Z"} — the
// Go windows carry the same values (used 25; remaining derived as 75 by the
// F24 renderer per global CONTRACTS §1.4), so the rendered text
// "- five hour: 25% used; 75% available; resets 2030-01-01T00:00:00Z" is
// identical to the Node script's output for the same fixture.
func TestGoldenFixtures(t *testing.T) {
	tests := []struct {
		name     string
		fixture  string
		want     []usage.Window
		wantErr  string
	}{
		{
			name:    "oauth_basic",
			fixture: "oauth_basic.json",
			want: []usage.Window{
				{ID: "5h", Label: "five hour", Unit: usage.UnitPercent, UsedPercent: fp(25), WindowMinutes: ip(300), ResetsAt: tm("2030-01-01T00:00:00Z"), UsageKnown: true},
			},
		},
		{
			name:    "oauth_synthetic_5h",
			fixture: "oauth_synthetic_5h.json",
			want: []usage.Window{
				{ID: "5h", Label: "five hour", Unit: usage.UnitPercent, Synthetic: true},
				{ID: "weekly", Label: "seven day", Unit: usage.UnitPercent, UsedPercent: fp(41), WindowMinutes: ip(10080), ResetsAt: tm("2030-01-01T00:00:00Z"), UsageKnown: true},
			},
		},
		{
			name:     "oauth_unsupported",
			fixture:  "oauth_unsupported.json",
			wantErr:  "unsupported_response",
		},
		{
			name:    "oauth_extra_usage",
			fixture: "oauth_extra_usage.json",
			want: []usage.Window{
				{ID: "5h", Label: "five hour", Unit: usage.UnitPercent, UsedPercent: fp(25), WindowMinutes: ip(300), UsageKnown: true},
				{ID: "extra_usage", Label: "Extra usage", Unit: usage.UnitUSD, Used: fp(7.5), Limit: fp(40), UsedPercent: fp(18.75), UsageKnown: true},
			},
		},
		{
			name:    "oauth_limits",
			fixture: "oauth_limits.json",
			want: []usage.Window{
				{ID: "5h", Label: "five hour", Unit: usage.UnitPercent, UsedPercent: fp(25), WindowMinutes: ip(300), UsageKnown: true},
				{ID: "limit:weekly_sonnet", Label: "sonnet", Unit: usage.UnitPercent, UsedPercent: fp(41), ResetsAt: tm("2026-08-07T18:00:00Z"), ModelScope: []string{"claude-sonnet-4-5"}, UsageKnown: true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeUsage(readFixture(t, tt.fixture))
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("NormalizeUsage: expected error code %q, got nil", tt.wantErr)
				}
				var e *Error
				if !errors.As(err, &e) || e.Code != tt.wantErr {
					t.Fatalf("error = %v, want code %q", err, tt.wantErr)
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

// TestGoldenFixtureParityBasic re-pins the Node-script parity values for
// oauth_basic.json explicitly: used 25, label "five hour", reset
// 2030-01-01T00:00:00Z, and the renderer-derived 75% available line
// (100 - UsedPercent per global CONTRACTS §1.4 / SPEC D6).
func TestGoldenFixtureParityBasic(t *testing.T) {
	got, err := NormalizeUsage(readFixture(t, "oauth_basic.json"))
	if err != nil {
		t.Fatalf("NormalizeUsage: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("windows = %d, want 1", len(got))
	}
	w := got[0]
	if w.Label != "five hour" || w.UsedPercent == nil || *w.UsedPercent != 25 {
		t.Errorf("window = label %q used %v, want label %q used 25", w.Label, w.UsedPercent, "five hour")
	}
	if w.ResetsAt == nil || !w.ResetsAt.Equal(*tm("2030-01-01T00:00:00Z")) {
		t.Errorf("ResetsAt = %v, want 2030-01-01T00:00:00Z", w.ResetsAt)
	}
	if remaining := 100 - *w.UsedPercent; remaining != 75 {
		t.Errorf("renderer-derived remaining = %v, want 75 (Node parity line)", remaining)
	}
}
