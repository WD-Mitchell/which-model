//go:build !nousage

package codexbar

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

func TestNormalizePayloads(t *testing.T) {
	used := 37.5
	minutes := 300
	reset := "2026-08-10T16:00:00Z"
	payloads := []cbPayload{{
		Provider: "claude",
		Source:   "web",
		Account:  "account@example.com",
		Usage: &cbUsage{
			Primary:   &cbRateWindow{UsedPercent: &used, WindowMinutes: &minutes, ResetsAt: &reset},
			Secondary: &cbRateWindow{UsedPercent: &used},
			UpdatedAt: "2026-08-10T15:00:00Z",
			Identity:  &cbIdentity{AccountEmail: "identity@example.com", LoginMethod: "oauth"},
		},
	}}

	got := normalizePayloads(payloads)
	if len(got) != 1 {
		t.Fatalf("normalizePayloads() returned %d snapshots, want 1", len(got))
	}
	snap := got[0]
	if snap.Provider != "claude" || snap.Source != usage.SourceWeb || snap.Account != "identity@example.com" || snap.Plan != "oauth" {
		t.Fatalf("snapshot identity/source = %#v", snap)
	}
	if !snap.FetchedAt.Equal(time.Date(2026, 8, 10, 15, 0, 0, 0, time.UTC)) {
		t.Fatalf("FetchedAt = %s", snap.FetchedAt)
	}
	if len(snap.Windows) != 2 || snap.Windows[0].ID != "5h" || snap.Windows[1].ID != "weekly" {
		t.Fatalf("windows = %#v", snap.Windows)
	}
	if snap.Windows[0].UsedPercent == nil || *snap.Windows[0].UsedPercent != used || snap.Windows[0].WindowMinutes == nil || *snap.Windows[0].WindowMinutes != minutes || !snap.Windows[0].UsageKnown || !snap.UsageKnown {
		t.Fatalf("primary window = %#v, snapshot usage_known=%v", snap.Windows[0], snap.UsageKnown)
	}
	if snap.Windows[0].ResetsAt == nil || !snap.Windows[0].ResetsAt.Equal(time.Date(2026, 8, 10, 16, 0, 0, 0, time.UTC)) {
		t.Fatalf("ResetsAt = %#v", snap.Windows[0].ResetsAt)
	}
}

func TestNormalizeErrorPayload(t *testing.T) {
	got := normalizePayloads([]cbPayload{{Provider: "codex", Source: "api", Error: &cbError{Code: "login_required", Message: "sign in"}}})
	if len(got) != 1 || got[0].Failure == nil || got[0].Failure.Code != "login_required" || got[0].Failure.Message != "sign in" {
		t.Fatalf("normalized error = %#v", got)
	}
	if got[0].Provider != "codex" || got[0].Confidence != "live" {
		t.Fatalf("normalized error metadata = %#v", got[0])
	}
}

func TestNormalizeEmptyPayloads(t *testing.T) {
	if got := normalizePayloads(nil); len(got) != 0 {
		t.Fatalf("normalizePayloads(nil) = %#v, want empty", got)
	}
}

func TestFetchMalformedJSON(t *testing.T) {
	bin := writeScript(t, "printf 'not-json'")
	t.Setenv("CODEXBAR_BIN", bin)
	got, err := Fetch(context.Background(), "claude")
	if err != nil {
		t.Fatalf("Fetch() error = %v, want failure snapshot", err)
	}
	if got.Failure == nil || got.Failure.Code != "provider_status" {
		t.Fatalf("Fetch() snapshot = %#v, want provider_status failure", got)
	}
}

func TestBinaryDiscoveryNotFound(t *testing.T) {
	oldLookPath := lookPath
	oldFixed := fixedBinaryPaths
	t.Cleanup(func() { lookPath = oldLookPath; fixedBinaryPaths = oldFixed })
	lookPath = func(string) (string, error) { return "", errors.New("not found") }
	fixedBinaryPaths = nil
	t.Setenv("CODEXBAR_BIN", filepath.Join(t.TempDir(), "missing"))
	if _, err := findBinary(); err == nil {
		t.Fatal("findBinary() error = nil, want BinaryNotFoundError")
	} else {
		var notFound *BinaryNotFoundError
		if !errors.As(err, &notFound) {
			t.Fatalf("findBinary() error = %T %v, want BinaryNotFoundError", err, err)
		}
	}
}

func TestFetchTimeout(t *testing.T) {
	bin := writeScript(t, "sleep 1")
	t.Setenv("CODEXBAR_BIN", bin)
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	got, err := Fetch(ctx, "claude")
	if err != nil {
		t.Fatalf("Fetch() error = %v, want timeout failure snapshot", err)
	}
	if got.Failure == nil || got.Failure.Code != "timeout" {
		t.Fatalf("Fetch() snapshot = %#v, want timeout failure", got)
	}
}

func TestFetchPassesSource(t *testing.T) {
	bin := writeScript(t, "printf '[{\"provider\":\"claude\",\"source\":\"web\",\"usage\":{}}]'")
	t.Setenv("CODEXBAR_BIN", bin)
	got, err := fetchWithSource(context.Background(), "claude", usage.SourceWeb)
	if err != nil || got.Provider != "claude" {
		t.Fatalf("fetchWithSource() = %#v, %v", got, err)
	}
}

func writeScript(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "codexbar")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestParseProviderIDs(t *testing.T) {
	text := "[--provider codex|claude|copilot|all]\n"
	got := parseProviderIDs(text)
	want := []string{"claude", "codex", "copilot"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("parseProviderIDs() = %v, want %v", got, want)
	}
}
