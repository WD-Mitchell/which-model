package whichmodel

import (
	"context"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/WD-Mitchell/which-model/internal/config"
	"github.com/WD-Mitchell/which-model/internal/usage"
	"github.com/WD-Mitchell/which-model/internal/usage/credential"
	"github.com/WD-Mitchell/which-model/internal/usage/fetch"
)

func TestPickCLIForwardsUsagePolicy(t *testing.T) {
	for _, backend := range []config.UsageBackend{config.UsageBackendOff, config.UsageBackendNative, config.UsageBackendCodexBar} {
		t.Run(string(backend), func(t *testing.T) {
			adapter := pickFetchAllFunc
			cfg, _ := pickPipelineSetup(t, pickTwoRoutes(), pickTwoScores(), nil, nil, nil)
			if err := os.WriteFile(cfg, []byte("[usage]\nbackend = \""+string(backend)+"\"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			setPickFetchAll(t, adapter)
			previous := pickUsageFetchAll
			t.Cleanup(func() { pickUsageFetchAll = previous; Global = GlobalFlags{} })
			calls := 0
			pickUsageFetchAll = func(_ context.Context, providers []string, opts fetch.Options) ([]usage.Snapshot, []credential.Warning, error) {
				calls++
				if opts.Backend != backend || !opts.Offline || !opts.Refresh || opts.MaxAge != 31*time.Second || opts.Timeout != 750*time.Millisecond {
					t.Fatalf("usage policy lost: %+v", opts)
				}
				if !reflect.DeepEqual(opts.Enabled, map[string]bool{"claude": true, "codex": true}) {
					t.Fatalf("enabled=%v", opts.Enabled)
				}
				return []usage.Snapshot{{Provider: "claude"}, {Provider: "codex"}}, nil, nil
			}
			code, _, stderr := captureExecuteFresh(t, []string{"pick", "--config", cfg, "--profile", "complex_implementation", "--offline", "--refresh-usage", "--max-age", "31s", "--timeout", "750ms", "--json"})
			if code != 0 || calls != 1 {
				t.Fatalf("code=%d calls=%d stderr=%s", code, calls, stderr)
			}
		})
	}
}
