//go:build !nousage

package codexbar_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/WD-Mitchell/which-model/internal/config"
	"github.com/WD-Mitchell/which-model/internal/usage"
	"github.com/WD-Mitchell/which-model/internal/usage/fetch"
)

// The existing native CI job selects TestNativeCompanyCodexBarApproval*, so this
// exercises the actual approved child and managed cache on all three platforms.
func TestNativeCompanyCodexBarApprovalCache(t *testing.T) {
	output := os.Getenv("COMPANY_CODEXBAR_OUTPUT")
	if os.Getenv("COMPANY_CODEXBAR_TEST_DIR") == "" || output == "" {
		t.Skip("disposable native CI fixture only")
	}
	opts := fetch.Options{Backend: config.UsageBackendCodexBar, Enabled: map[string]bool{"antigravity": true},
		CacheDir: t.TempDir(), Source: usage.SourceCLI, MaxAge: 24 * time.Hour}
	read := func(opts fetch.Options, wantSource usage.Source, child bool) {
		t.Helper()
		got, warnings, err := fetch.FetchAll(context.Background(), []string{"antigravity"}, opts)
		if err != nil || len(warnings) != 0 || len(got) != 1 || got[0].Failure != nil ||
			!got[0].Stale || !got[0].UsageKnown || !got[0].FetchedAt.IsZero() || got[0].Source != wantSource {
			t.Fatalf("approved fetch/cache result: %+v warnings=%v err=%v", got, warnings, err)
		}
		_, err = os.Stat(output)
		if child {
			if err != nil {
				t.Fatal("expected approved child did not run")
			}
			if err := os.Remove(output); err != nil {
				t.Fatal(err)
			}
		} else if !os.IsNotExist(err) {
			t.Fatal("cache-only read launched a child")
		}
	}
	read(opts, usage.SourceLocal, true)
	opts.Offline = true
	read(opts, usage.SourceCache, false)
	opts.Offline = false
	opts.Source = usage.SourceCache
	read(opts, usage.SourceCache, false)
	opts.Source = usage.SourceCLI
	read(opts, usage.SourceLocal, true) // producer-stale cache requires new evidence
}
