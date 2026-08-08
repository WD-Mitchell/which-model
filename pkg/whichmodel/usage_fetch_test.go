//go:build !nousage

package whichmodel

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)
import "github.com/WD-Mitchell/which-model/internal/usage"

func f64(v float64) *float64 { return &v }

func tptr(s string) *time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return &t
}

func claudeGoldenSnapshot() usage.Snapshot {
	return usage.Snapshot{
		Provider: "claude",
		Windows: []usage.Window{
			{ID: "5h", Label: "five hour", Unit: usage.UnitPercent, UsedPercent: f64(25), ResetsAt: tptr("2026-08-07T18:00:00Z"), UsageKnown: true},
			{ID: "7d", Label: "seven day", Unit: usage.UnitPercent, UsedPercent: f64(41), UsageKnown: true},
		},
		UsageKnown: true,
	}
}

func TestRunUsageFetchAllJSONGolden(t *testing.T) {
	old := fetchAllFunc
	t.Cleanup(func() { fetchAllFunc = old })
	fetchAllFunc = func(ctx context.Context, opts FetchAllOptions) (*FetchResult, error) {
		if !reflect.DeepEqual(opts.Providers, []string{"claude", "codex"}) {
			t.Errorf("providers = %v", opts.Providers)
		}
		return &FetchResult{Snapshots: []usage.Snapshot{claudeGoldenSnapshot()}}, nil
	}
	cfg := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(cfg, []byte("[providers.claude]\nenabled = true\n[providers.codex]\nenabled = true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errOut strings.Builder
	err := RunUsage(UsageArgs{All: true, JSON: true, ConfigPath: cfg}, &out, &errOut)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out.String()), &got); err != nil {
		t.Fatal(err)
	}
	if got["schema_version"] != "2.0" || got["usage_enabled"] != true {
		t.Fatalf("envelope = %v", got)
	}
	if _, ok := got["last_verified"]; ok {
		t.Fatalf("last_verified present: %v", got)
	}
	snaps, ok := got["snapshots"].([]any)
	if !ok || len(snaps) != 1 || snaps[0].(map[string]any)["provider"] != "claude" {
		t.Fatalf("snapshots = %v", got["snapshots"])
	}
}

func TestRunUsageFetchOptionsPassthrough(t *testing.T) {
	old := fetchAllFunc
	t.Cleanup(func() { fetchAllFunc = old })
	var got FetchAllOptions
	fetchAllFunc = func(ctx context.Context, opts FetchAllOptions) (*FetchResult, error) {
		got = opts
		return &FetchResult{Snapshots: []usage.Snapshot{{Provider: "claude"}}}, nil
	}
	var out, errOut strings.Builder
	err := RunUsage(UsageArgs{Providers: []string{"claude"}, ForceRefresh: true, MaxAge: 90 * time.Minute, Timeout: 7 * time.Second}, &out, &errOut)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Providers, []string{"claude"}) || !got.ForceRefresh || got.MaxAge != 90*time.Minute || got.Timeout != 7*time.Second {
		t.Fatalf("options = %#v", got)
	}
}

func TestRunUsageFetchError(t *testing.T) {
	old := fetchAllFunc
	t.Cleanup(func() { fetchAllFunc = old })
	fetchAllFunc = func(context.Context, FetchAllOptions) (*FetchResult, error) { return nil, errors.New("boom") }
	var out, errOut strings.Builder
	err := RunUsage(UsageArgs{Providers: []string{"claude"}}, &out, &errOut)
	if ExitCodeFor(err) != 1 || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("err = %v, exit = %d", err, ExitCodeFor(err))
	}
}

func TestRunUsageTextPlaceholderJSON(t *testing.T) {
	old := fetchAllFunc
	t.Cleanup(func() { fetchAllFunc = old })
	fetchAllFunc = func(context.Context, FetchAllOptions) (*FetchResult, error) {
		return &FetchResult{Snapshots: []usage.Snapshot{{Provider: "claude"}}}, nil
	}
	var out, errOut strings.Builder
	if err := RunUsage(UsageArgs{Providers: []string{"claude"}}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "usage allowance") {
		t.Fatalf("stdout = %q", out.String())
	}
}
