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

func usageTestConfigPath(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// validateSource accepts exactly the six canonical sources; the empty value
// is the auto fallback chain. An explicit "auto" is rejected like any other
// unknown value (F24 SPEC §2.4, D-1; TASKS T5 test 1).
func TestValidateSourceRejectsUnknown(t *testing.T) {
	for _, source := range []usage.Source{"bogus", "auto"} {
		err := validateSource(source)
		if err == nil {
			t.Fatalf("validateSource(%q) = nil, want error", source)
		}
		want := `invalid --source "` + string(source) + `"; valid: oauth, api, cli, web, local, cache`
		if err.Error() != want {
			t.Errorf("validateSource(%q) = %q, want %q", source, err.Error(), want)
		}
	}
}

func TestValidateSourceAcceptsEnumAndAuto(t *testing.T) {
	for _, source := range []usage.Source{"", usage.SourceOAuth, usage.SourceAPI, usage.SourceCLI, usage.SourceWeb, usage.SourceLocal, usage.SourceCache} {
		if err := validateSource(source); err != nil {
			t.Errorf("validateSource(%q) = %v, want nil", source, err)
		}
	}
}

// Per-provider membership: the forced source must be one the provider's
// credential chain can produce (claude is env/keychain/file → api only);
// cache is a universal view and the empty value is the auto chain
// (F24 SPEC §2.4; TASKS T5 tests 3-4).
func TestValidateProviderSourceMembership(t *testing.T) {
	err := validateProviderSource("claude", usage.SourceWeb)
	if err == nil {
		t.Fatal("validateProviderSource(claude, web) = nil, want error")
	}
	if want := `provider "claude" has no web source; valid sources: api`; err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
	for _, tc := range []struct {
		provider string
		source   usage.Source
	}{
		{"claude", usage.SourceAPI},
		{"claude", usage.SourceCache},
		{"claude", ""},
		{"copilot", usage.SourceCLI},
		{"copilot", usage.SourceAPI},
		{"copilot", usage.SourceOAuth},
	} {
		if err := validateProviderSource(tc.provider, tc.source); err != nil {
			t.Errorf("validateProviderSource(%q, %q) = %v, want nil", tc.provider, tc.source, err)
		}
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
	if err := RunUsage(UsageArgs{Providers: []string{"claude"}, Source: usage.SourceCache, ConfigPath: usageTestConfigPath(t, "[usage]\nbackend = \"native\"\n[providers.claude]\nenabled = true\n")}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if got.Source != usage.SourceCache {
		t.Fatalf("source = %q", got.Source)
	}
}

// An unknown forced source is an argument error before any fetch (F24 SPEC
// §3 exit-matrix row; TASKS T5 test 6).
func TestRunUsageRejectsUnknownSource(t *testing.T) {
	cfg := usageTestConfigPath(t, "[usage]\nbackend = \"native\"\n[providers.claude]\nenabled = true\n")
	err := RunUsage(UsageArgs{Providers: []string{"claude"}, Source: usage.Source("bogus"), ConfigPath: cfg}, nil, nil)
	if ExitCodeFor(err) != 2 || !strings.Contains(err.Error(), `invalid --source "bogus"`) {
		t.Fatalf("err = %v, exit = %d", err, ExitCodeFor(err))
	}
}

// A source the provider cannot produce is an argument error naming the
// provider and its valid sources (F24 SPEC §3 exit-matrix row).
func TestRunUsageRejectsUndeclaredProviderSource(t *testing.T) {
	cfg := usageTestConfigPath(t, "[usage]\nbackend = \"native\"\n[providers.claude]\nenabled = true\n")
	err := RunUsage(UsageArgs{Providers: []string{"claude"}, Source: usage.SourceWeb, ConfigPath: cfg}, nil, nil)
	if ExitCodeFor(err) != 2 || !strings.Contains(err.Error(), `provider "claude" has no web source`) {
		t.Fatalf("err = %v, exit = %d", err, ExitCodeFor(err))
	}
}
