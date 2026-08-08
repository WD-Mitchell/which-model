//go:build !nousage

package whichmodel

import (
	"context"
	"strings"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

const canary = "CANARY-7f3a9c-IDENTITY"

func TestUsageIdentityRedactedByDefault(t *testing.T) {
	old := fetchAllFunc
	t.Cleanup(func() { fetchAllFunc = old })
	fetchAllFunc = func(context.Context, FetchAllOptions) (*FetchResult, error) {
		return &FetchResult{Snapshots: []usage.Snapshot{{Provider: "claude", Account: canary, Plan: "pro"}}}, nil
	}
	var out, errOut strings.Builder
	if err := RunUsage(UsageArgs{Providers: []string{"claude"}, JSON: true}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), `"account"`) || strings.Contains(out.String(), canary) {
		t.Fatalf("redacted output = %q", out.String())
	}
}

func TestUsageIdentityShownWithFlag(t *testing.T) {
	old := fetchAllFunc
	t.Cleanup(func() { fetchAllFunc = old })
	fetchAllFunc = func(context.Context, FetchAllOptions) (*FetchResult, error) {
		return &FetchResult{Snapshots: []usage.Snapshot{{Provider: "claude", Account: canary}}}, nil
	}
	var out, errOut strings.Builder
	if err := RunUsage(UsageArgs{Providers: []string{"claude"}, JSON: true, ShowIdentity: true}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"account": "`+canary+`"`) {
		t.Fatalf("identity output = %q", out.String())
	}
}

func TestUsageFailureMessagePassThrough(t *testing.T) {
	old := fetchAllFunc
	t.Cleanup(func() { fetchAllFunc = old })
	fetchAllFunc = func(context.Context, FetchAllOptions) (*FetchResult, error) {
		return &FetchResult{Snapshots: []usage.Snapshot{
			{Provider: "claude"},
			{Provider: "codex", Failure: &usage.Failure{Code: "unauthorized", Message: "unauthorized: " + canary}},
		}}, nil
	}
	var out, errOut strings.Builder
	if err := RunUsage(UsageArgs{Providers: []string{"claude"}, JSON: true}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "unauthorized: "+canary) {
		t.Fatalf("failure output = %q", out.String())
	}
}

func TestRedactIdentityDoesNotMutateInput(t *testing.T) {
	original := usage.Snapshot{Provider: "claude", Account: canary}
	res := &FetchResult{Snapshots: []usage.Snapshot{original}}
	redacted := redactIdentity(res, false)
	if res.Snapshots[0].Account != canary {
		t.Fatal("input snapshot was mutated")
	}
	if redacted.Snapshots[0].Account != "" {
		t.Fatal("redacted snapshot still contains account")
	}
}
