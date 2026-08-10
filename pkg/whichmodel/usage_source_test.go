//go:build !nousage

package whichmodel

import (
	"context"
	"strings"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

func TestValidateSourcePassThrough(t *testing.T) {
	if err := validateSource(usage.Source("bogus")); err != nil {
		t.Fatalf("validateSource(bogus) = %v, want nil", err)
	}
}

func TestValidateSourceValid(t *testing.T) {
	for _, source := range []usage.Source{usage.SourceOAuth, usage.SourceCache} {
		if err := validateSource(source); err != nil {
			t.Fatalf("validateSource(%q) = %v", source, err)
		}
	}
}

func TestValidateProviderSourcePassThrough(t *testing.T) {
	if err := validateProviderSource("claude", usage.SourceWeb); err != nil {
		t.Fatalf("validateProviderSource() = %v, want nil", err)
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

func TestRunUsageSourcePassThrough(t *testing.T) {
	old := fetchAllFunc
	t.Cleanup(func() { fetchAllFunc = old })
	var got FetchAllOptions
	fetchAllFunc = func(ctx context.Context, opts FetchAllOptions) (*FetchResult, error) {
		got = opts
		return &FetchResult{Snapshots: []usage.Snapshot{{Provider: "claude"}}}, nil
	}
	var out, errOut strings.Builder
	if err := RunUsage(UsageArgs{Providers: []string{"claude"}, Source: usage.Source("bogus")}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if got.Source != usage.Source("bogus") {
		t.Fatalf("source = %q", got.Source)
	}
}
