//go:build !nousage

package whichmodel

import (
	"context"
	"strings"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

var exitFiveCodes = map[string]bool{"unauthorized": true, "login_required": true, "expired_credential": true, "credential_file": true, "credential_json": true, "unsafe_credential": true, "access_denied": true, "device_expired": true, "cookie_unavailable": true, "signing_failed": true}

func usageFailureSnapshots(codes ...string) []usage.Snapshot {
	snaps := make([]usage.Snapshot, len(codes))
	for i, code := range codes {
		snaps[i] = usage.Snapshot{Provider: []string{"claude", "codex", "copilot"}[i%3], Failure: &usage.Failure{Code: code, Message: code + " message"}}
	}
	return snaps
}

func runUsageWithSnapshots(t *testing.T, snaps []usage.Snapshot, jsonMode bool) (error, string, string) {
	t.Helper()
	old := fetchAllFunc
	t.Cleanup(func() { fetchAllFunc = old })
	fetchAllFunc = func(context.Context, FetchAllOptions) (*FetchResult, error) { return &FetchResult{Snapshots: snaps}, nil }
	var out, errOut strings.Builder
	err := RunUsage(UsageArgs{Providers: []string{"claude", "codex"}, JSON: jsonMode}, &out, &errOut)
	return err, out.String(), errOut.String()
}

func TestUsageExitMixedSuccess(t *testing.T) {
	err, out, _ := runUsageWithSnapshots(t, []usage.Snapshot{{Provider: "claude"}, usageFailureSnapshots("unauthorized")[0]}, true)
	if err != nil || !strings.Contains(out, `"provider": "claude"`) || !strings.Contains(out, `"unauthorized"`) {
		t.Fatalf("err = %v, out = %q", err, out)
	}
}

func TestUsageExitAllNonAuth(t *testing.T) {
	err, out, errOut := runUsageWithSnapshots(t, usageFailureSnapshots("rate_limited", "rate_limited"), true)
	coded, ok := err.(*CodedError)
	if !ok || coded.Code != "rate_limited" || ExitCodeFor(err) != 1 || out != "" || strings.Count(errOut, "[rate_limited]") != 2 {
		t.Fatalf("err = %#v, out = %q, stderr = %q", err, out, errOut)
	}
}

func TestUsageExitMixedFailuresAuthWins(t *testing.T) {
	err, out, _ := runUsageWithSnapshots(t, usageFailureSnapshots("rate_limited", "unauthorized"), true)
	coded, ok := err.(*CodedError)
	if !ok || coded.Code != "unauthorized" || ExitCodeFor(err) != 5 || out != "" {
		t.Fatalf("err = %#v, out = %q", err, out)
	}
}

func TestUsageExitPureAuth(t *testing.T) {
	err, out, _ := runUsageWithSnapshots(t, usageFailureSnapshots("login_required", "login_required"), true)
	coded, ok := err.(*CodedError)
	if !ok || coded.Code != "login_required" || ExitCodeFor(err) != 5 || out != "" {
		t.Fatalf("err = %#v, out = %q", err, out)
	}
}

func TestUsageExitIndependentOfJSONMode(t *testing.T) {
	errJSON, _, _ := runUsageWithSnapshots(t, usageFailureSnapshots("rate_limited", "timeout"), true)
	errText, _, _ := runUsageWithSnapshots(t, usageFailureSnapshots("rate_limited", "timeout"), false)
	if ExitCodeFor(errJSON) != ExitCodeFor(errText) || ExitCodeFor(errText) != 1 {
		t.Fatalf("json exit = %d, text exit = %d", ExitCodeFor(errJSON), ExitCodeFor(errText))
	}
}

func TestUsageExitFailureLines(t *testing.T) {
	_, _, errOut := runUsageWithSnapshots(t, usageFailureSnapshots("rate_limited", "timeout"), false)
	if strings.Count(errOut, "[") != 2 || !strings.Contains(errOut, "[rate_limited] rate_limited message") || !strings.Contains(errOut, "[timeout] timeout message") {
		t.Fatalf("stderr = %q", errOut)
	}
}
