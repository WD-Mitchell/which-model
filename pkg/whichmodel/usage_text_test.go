//go:build !nousage

package whichmodel

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

func TestFormatUsageTextGolden(t *testing.T) {
	report := &UsageReport{SchemaVersion: "2.0", UsageEnabled: true, Snapshots: []usage.Snapshot{
		claudeGoldenSnapshot(),
		{Provider: "codex", Windows: []usage.Window{
			{ID: "primary", Label: "primary window", Unit: usage.UnitPercent, UsedPercent: f64(12), ResetsAt: tptr("2026-08-08T00:00:00Z")},
			{ID: "credits", Label: "credits", Unit: usage.UnitCredits, Remaining: f64(340)},
		}},
	}}
	want := "claude usage allowance\n" +
		"- five hour: 25% used; 75% available; resets 2026-08-07T18:00:00Z\n" +
		"- seven day: 41% used; 59% available\n\n" +
		"codex usage allowance\n" +
		"- primary window: 12% used; 88% available; resets 2026-08-08T00:00:00Z\n" +
		"- credits: 340 remaining\n"
	if got := FormatUsageText(report, false); got != want {
		t.Fatalf("text = %q, want %q", got, want)
	}
}

func TestFormatUsageTextUnlimited(t *testing.T) {
	report := &UsageReport{Snapshots: []usage.Snapshot{{Provider: "claude", Windows: []usage.Window{{Label: "chat", Unlimited: true}}}}}
	if got := FormatUsageText(report, false); got != "claude usage allowance\n- chat: unlimited\n" {
		t.Fatalf("text = %q", got)
	}
}

func TestFormatUsageTextRemainingAndTotal(t *testing.T) {
	report := &UsageReport{Snapshots: []usage.Snapshot{{Provider: "claude", Windows: []usage.Window{{Label: "chat", Remaining: f64(1200), Limit: f64(4800)}}}}}
	if got := FormatUsageText(report, false); got != "claude usage allowance\n- chat: 1200 remaining; 4800 total\n" {
		t.Fatalf("text = %q", got)
	}
}

func TestFormatUsageTextResetHint(t *testing.T) {
	report := &UsageReport{Snapshots: []usage.Snapshot{{Provider: "claude", Windows: []usage.Window{{Label: "a", ResetHint: "resets at midnight UTC"}, {Label: "b", ResetHint: "midnight UTC"}}}}}
	got := FormatUsageText(report, false)
	if !strings.Contains(got, "- a: resets at midnight UTC") || !strings.Contains(got, "- b: resets midnight UTC") {
		t.Fatalf("text = %q", got)
	}
}

func TestFormatUsageTextIdentity(t *testing.T) {
	report := &UsageReport{Snapshots: []usage.Snapshot{{Provider: "claude", Account: "user@x"}}}
	if got := FormatUsageText(report, true); !strings.HasSuffix(got, "- account: user@x\n") {
		t.Fatalf("identity text = %q", got)
	}
	if got := FormatUsageText(report, false); strings.Contains(got, "user@x") || strings.Contains(got, "account") {
		t.Fatalf("redacted text = %q", got)
	}
}

func TestFormatUsageTextNumber(t *testing.T) {
	report := &UsageReport{Snapshots: []usage.Snapshot{{Provider: "claude", Windows: []usage.Window{{Label: "chat", UsedPercent: f64(12.5)}}}}}
	if got := FormatUsageText(report, false); !strings.Contains(got, "12.5% used") {
		t.Fatalf("text = %q", got)
	}
}

func TestRunUsageNoUsageFlag(t *testing.T) {
	var out, errOut strings.Builder
	err := RunUsage(UsageArgs{Providers: []string{"claude"}, NoUsage: true}, &out, &errOut)
	coded, ok := err.(*CodedError)
	if !ok || coded.Code != "usage_disabled" || ExitCodeFor(err) != 2 || coded.Message != "usage is disabled by --no-usage" {
		t.Fatalf("err = %#v, exit = %d", err, ExitCodeFor(err))
	}
}

func TestRunUsageNoUsageConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("[usage]\nenabled = false\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errOut strings.Builder
	err := RunUsage(UsageArgs{Providers: []string{"claude"}, ConfigPath: path}, &out, &errOut)
	if ExitCodeFor(err) != 2 || !strings.Contains(err.Error(), "usage is disabled by [usage] enabled = false") || out.Len() != 0 {
		t.Fatalf("err = %v, exit = %d, out = %q", err, ExitCodeFor(err), out.String())
	}
}

func TestRunUsageTextRenderer(t *testing.T) {
	old := fetchAllFunc
	t.Cleanup(func() { fetchAllFunc = old })
	fetchAllFunc = func(context.Context, FetchAllOptions) (*FetchResult, error) {
		return &FetchResult{Snapshots: []usage.Snapshot{{Provider: "claude"}}}, nil
	}
	var out, errOut strings.Builder
	if err := RunUsage(UsageArgs{Providers: []string{"claude"}, JSON: false}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "schema_version") {
		t.Fatalf("text output still JSON: %q", out.String())
	}
}
