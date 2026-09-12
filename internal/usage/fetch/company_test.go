//go:build !nousage

package fetch

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/company"
	"github.com/WD-Mitchell/which-model/internal/config"
)

func TestCompanyFetchOptionsCannotGrantProvidersOrDelegation(t *testing.T) {
	old := readCompanyPolicy
	t.Cleanup(func() { readCompanyPolicy = old })
	p := company.Defaults()
	p.AllowedProviders = []string{"codex"}
	readCompanyPolicy = func() (company.Snapshot, error) { return company.Snapshot{Managed: true, Policy: &p}, nil }
	for _, opts := range []Options{
		{Enabled: map[string]bool{"forbidden": true}},
		{Backend: config.UsageBackendCodexBar, Enabled: map[string]bool{"codex": true}},
		{DisableManagedKeychain: true, Enabled: map[string]bool{"codex": true}},
	} {
		dir := t.TempDir()
		opts.CacheDir = dir
		_, _, err := FetchAll(context.Background(), []string{"codex", "forbidden"}, opts)
		var denied *company.Error
		if !errors.As(err, &denied) {
			t.Fatal("direct options bypassed policy", err)
		}
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) != 0 {
			t.Fatal("refused operation wrote cache", err)
		}
	}
}
