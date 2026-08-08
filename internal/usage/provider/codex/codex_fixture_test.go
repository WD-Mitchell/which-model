//go:build !nousage

package codex

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

// goldenCases lists the Node-parity fixtures under testdata/usage/codex/ and
// the canonical normalized shape each must produce (annex-a §3.1).
var goldenCases = []struct {
	name    string
	fixture string
	want    []usage.Window
}{
	{
		name:    "codex_basic",
		fixture: "testdata/usage/codex/codex_basic.json",
		want: []usage.Window{
			{ID: "5h", Label: "primary window", Unit: usage.UnitPercent, UsedPercent: f64(20), ResetsAt: reset2030, UsageKnown: true},
		},
	},
	{
		name:    "codex_camel",
		fixture: "testdata/usage/codex/codex_camel.json",
		want: []usage.Window{
			{ID: "weekly", Label: "secondary window", Unit: usage.UnitPercent, UsedPercent: f64(33), ResetsAt: reset2030, UsageKnown: true},
		},
	},
	{
		name:    "codex_credits",
		fixture: "testdata/usage/codex/codex_credits.json",
		want: []usage.Window{
			{ID: "5h", Label: "primary window", Unit: usage.UnitPercent, UsedPercent: f64(20), UsageKnown: true},
			{ID: "credits", Label: "credits", Unit: usage.UnitCredits, Remaining: f64(12.5), UsageKnown: true},
		},
	},
	{
		name:    "codex_additional",
		fixture: "testdata/usage/codex/codex_additional.json",
		want: []usage.Window{
			{ID: "5h", Label: "primary window", Unit: usage.UnitPercent, UsedPercent: f64(20), UsageKnown: true},
			{ID: "additional:o1-mini-weekly", Label: "o1-mini-weekly", Unit: usage.UnitPercent, UsedPercent: f64(55), ModelScope: []string{"o1-mini"}, WindowMinutes: intPtr(10080), ResetsAt: reset2030, UsageKnown: true},
		},
	},
	{
		name:    "codex_top_level",
		fixture: "testdata/usage/codex/codex_top_level.json",
		want: []usage.Window{
			{ID: "5h", Label: "primary window", Unit: usage.UnitPercent, UsedPercent: f64(20), UsageKnown: true},
		},
	},
	{
		name:    "codex_unsupported",
		fixture: "testdata/usage/codex/codex_unsupported.json",
		want:    nil,
	},
}

// TestGoldenFixtures asserts each fixture normalizes exactly as Node did
// (F16-T8 instruction 2).
func TestGoldenFixtures(t *testing.T) {
	for _, tc := range goldenCases {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := os.ReadFile(tc.fixture)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			if len(raw) == 0 {
				t.Fatal("empty fixture")
			}
			got, err := NormalizeUsage(raw)
			if tc.name == "codex_unsupported" {
				if err == nil {
					t.Fatalf("NormalizeUsage() = %+v, want unsupported_response", got)
				}
				var ce *Error
				if !asCodexError(err, &ce) || ce.Code != "unsupported_response" {
					t.Fatalf("error = %v, want code unsupported_response", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizeUsage() error: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("windows = %+v, want %+v", got, tc.want)
			}
			for i := range tc.want {
				w := got[i]
				want := tc.want[i]
				if w.ID != want.ID || w.Label != want.Label || w.Unit != want.Unit {
					t.Errorf("window[%d] = %+v, want %+v", i, w, want)
				}
				if !samePtr(w.UsedPercent, want.UsedPercent) {
					t.Errorf("window[%d].UsedPercent = %v, want %v", i, w.UsedPercent, want.UsedPercent)
				}
				if !samePtr(w.Remaining, want.Remaining) {
					t.Errorf("window[%d].Remaining = %v, want %v", i, w.Remaining, want.Remaining)
				}
				if !samePtrInt(w.WindowMinutes, want.WindowMinutes) {
					t.Errorf("window[%d].WindowMinutes = %v, want %v", i, w.WindowMinutes, want.WindowMinutes)
				}
				if !sameTime(w.ResetsAt, want.ResetsAt) {
					t.Errorf("window[%d].ResetsAt = %v, want %v", i, w.ResetsAt, want.ResetsAt)
				}
				if len(w.ModelScope) != len(want.ModelScope) {
					t.Errorf("window[%d].ModelScope = %v, want %v", i, w.ModelScope, want.ModelScope)
				}
				for j := range want.ModelScope {
					if w.ModelScope[j] != want.ModelScope[j] {
						t.Errorf("window[%d].ModelScope = %v, want %v", i, w.ModelScope, want.ModelScope)
					}
				}
				if !w.UsageKnown {
					t.Errorf("window[%d].UsageKnown = false, want true", i)
				}
			}
		})
	}
}

func samePtr(a, b *float64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func samePtrInt(a, b *int) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func sameTime(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Equal(*b)
}

// TestGoldenFixtureFetchRoundTrip runs the two success fixtures through the
// real Fetch path and asserts the golden windows survive end to end.
func TestGoldenFixtureFetchRoundTrip(t *testing.T) {
	for _, name := range []string{"codex_basic", "codex_credits", "codex_additional"} {
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile("testdata/usage/codex/" + name + ".json")
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			writeAuth(t, `{"tokens":{"access_token":"`+canaryToken+`","account_id":"`+canaryAcct+`"}}`)
			stub := &stubTransport{fn: canned(200, string(raw))}
			snap, err := Fetch(context.Background(), usage.Credential{}, &http.Client{Transport: stub})
			if err != nil {
				t.Fatalf("Fetch() error: %v", err)
			}
			if snap.Failure != nil {
				t.Fatalf("unexpected Failure: %+v", snap.Failure)
			}
			wantLen := len(goldenWindows(name))
			if len(snap.Windows) != wantLen {
				t.Fatalf("windows = %d, want %d: %+v", len(snap.Windows), wantLen, snap.Windows)
			}
			if !strings.HasPrefix(snap.Windows[0].ID, "5h") && snap.Windows[0].ID != "weekly" {
				t.Errorf("first window ID = %q, want 5h or weekly", snap.Windows[0].ID)
			}
		})
	}
}

func goldenWindows(name string) []usage.Window {
	for _, tc := range goldenCases {
		if tc.name == name {
			return tc.want
		}
	}
	return nil
}
