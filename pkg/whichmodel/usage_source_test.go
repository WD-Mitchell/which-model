//go:build !nousage

package whichmodel

import (
	"context"
	"strings"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

func TestValidateSourceEnum(t *testing.T) {
	err := validateSource(usage.Source("bogus"))
	if err == nil || err.Error() != `invalid --source "bogus"; valid: oauth, api, cli, web, local, cache` {
		t.Fatalf("err = %v", err)
	}
}

func TestValidateSourceValid(t *testing.T) {
	for _, source := range []usage.Source{usage.SourceOAuth, usage.SourceCache} {
		if err := validateSource(source); err != nil {
			t.Fatalf("validateSource(%q) = %v", source, err)
		}
	}
}

func TestValidateProviderSource(t *testing.T) {
	if err := validateProviderSource("claude", usage.SourceWeb); err == nil || !strings.Contains(err.Error(), `provider "claude" has no web source`) || !strings.Contains(err.Error(), "valid sources:") {
		t.Fatalf("err = %v", err)
	}
	if err := validateProviderSource("claude", usage.SourceAPI); err != nil {
		t.Fatalf("declared source rejected: %v", err)
	}
}

func TestRunUsageSourceCachePassthrough(t *testing.T) {
	old := fetchAllFunc
	t.Cleanup(func() { fetchAllFunc = old })
	var got FetchAllOptions
	fetchAllFunc = func(ctx context.Context, opts FetchAllOptions) (*FetchResult, error) {
		got = opts
		return &FetchResult{Snapshots: []usage.Snapshot{{Provider: "claude"}}}, nil
	}
	var out, errOut strings.Builder
	if err := RunUsage(UsageArgs{Providers: []string{"claude"}, Source: usage.SourceCache}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if got.Source != usage.SourceCache {
		t.Fatalf("source = %q", got.Source)
	}
}

func TestRunUsageInvalidSource(t *testing.T) {
	var out, errOut strings.Builder
	err := RunUsage(UsageArgs{Providers: []string{"claude"}, Source: usage.Source("bogus")}, &out, &errOut)
	if ExitCodeFor(err) != 2 || !strings.Contains(err.Error(), "invalid --source") {
		t.Fatalf("err = %v, exit = %d", err, ExitCodeFor(err))
	}
}
