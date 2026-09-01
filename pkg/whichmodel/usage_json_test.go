//go:build !nousage

package whichmodel

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

func usageJSONRun(t *testing.T, result *FetchResult) map[string]any {
	t.Helper()
	old := fetchAllFunc
	t.Cleanup(func() { fetchAllFunc = old })
	fetchAllFunc = func(context.Context, FetchAllOptions) (*FetchResult, error) { return result, nil }
	var out, errOut strings.Builder
	if err := RunUsage(UsageArgs{Providers: []string{"claude"}, JSON: true}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err := json.Unmarshal([]byte(out.String()), &root); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestUsageJSONLastVerifiedPresent(t *testing.T) {
	s := claudeGoldenSnapshot()
	root := usageJSONRun(t, &FetchResult{Snapshots: []usage.Snapshot{s}, LastVerified: map[string]time.Time{"claude": time.Date(2026, 8, 7, 17, 3, 11, 0, time.UTC)}})
	last, ok := root["last_verified"].(map[string]any)
	if !ok || last["claude"] != "2026-08-07T17:03:11Z" {
		t.Fatalf("last_verified = %v", root["last_verified"])
	}
}

func TestUsageJSONLastVerifiedOmittedWhenEmpty(t *testing.T) {
	root := usageJSONRun(t, &FetchResult{Snapshots: []usage.Snapshot{claudeGoldenSnapshot()}})
	if _, ok := root["last_verified"]; ok {
		t.Fatalf("last_verified unexpectedly present: %v", root)
	}
}

func TestUsageJSONEnvelopeFields(t *testing.T) {
	root := usageJSONRun(t, &FetchResult{Snapshots: []usage.Snapshot{claudeGoldenSnapshot()}})
	if root["schema_version"] != "2.0" || root["usage_enabled"] != true {
		t.Fatalf("root = %v", root)
	}
	if _, ok := root["usage_disabled_reason"]; ok {
		t.Fatal("usage_disabled_reason must be absent")
	}
}

func TestUsageJSONSnapshotFidelity(t *testing.T) {
	root := usageJSONRun(t, &FetchResult{Snapshots: []usage.Snapshot{claudeGoldenSnapshot()}})
	snaps := root["snapshots"].([]any)
	snap := snaps[0].(map[string]any)
	if snap["provider"] != "claude" || snap["usage_known"] != true {
		t.Fatalf("snapshot = %v", snap)
	}
	window := snap["windows"].([]any)[0].(map[string]any)
	if window["id"] != "5h" || window["used_percent"] != 25.0 || window["resets_at"] != "2026-08-07T18:00:00Z" || window["usage_known"] != true {
		t.Fatalf("window = %v", window)
	}
}

func TestUsageJSONBandAtOrAboveFiltersSnapshots(t *testing.T) {
	old := fetchAllFunc
	t.Cleanup(func() { fetchAllFunc = old })
	criticalUsed, elevatedUsed := 80.0, 75.0
	critical, elevated := claudeGoldenSnapshot(), claudeGoldenSnapshot()
	critical.Provider = "claude"
	critical.Windows[0].UsedPercent = &criticalUsed
	elevated.Provider = "codex"
	elevated.Windows[0].UsedPercent = &elevatedUsed
	fetchAllFunc = func(context.Context, FetchAllOptions) (*FetchResult, error) {
		return &FetchResult{Snapshots: []usage.Snapshot{critical, elevated}}, nil
	}

	var out, errOut strings.Builder
	err := RunUsage(UsageArgs{
		Providers:     []string{"claude", "codex"},
		BandAtOrAbove: "critical",
		JSON:          true,
	}, &out, &errOut)
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err := json.Unmarshal([]byte(out.String()), &root); err != nil {
		t.Fatal(err)
	}
	snapshots := root["snapshots"].([]any)
	if len(snapshots) != 1 || snapshots[0].(map[string]any)["provider"] != "claude" {
		t.Fatalf("snapshots = %v, want only critical claude snapshot", snapshots)
	}
}

func TestUsageBandAtOrAboveRejectsUnknownBandBeforeFetch(t *testing.T) {
	old := fetchAllFunc
	t.Cleanup(func() { fetchAllFunc = old })
	called := false
	fetchAllFunc = func(context.Context, FetchAllOptions) (*FetchResult, error) {
		called = true
		return &FetchResult{}, nil
	}

	err := RunUsage(UsageArgs{
		Providers:     []string{"claude"},
		BandAtOrAbove: "missing",
		JSON:          true,
	}, nil, nil)
	if ExitCodeFor(err) != 2 || !strings.Contains(err.Error(), `invalid --band-at-or-above "missing"`) {
		t.Fatalf("err = %v, exit = %d", err, ExitCodeFor(err))
	}
	if called {
		t.Fatal("fetch called for invalid band")
	}
}
