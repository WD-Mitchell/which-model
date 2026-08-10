//go:build nousage

package fetch

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/WD-Mitchell/which-model/internal/usage"
	"github.com/WD-Mitchell/which-model/internal/usage/cache"
	"github.com/WD-Mitchell/which-model/internal/usage/credential"
)

func TestRegistryNouseage(t *testing.T) {
	if got := usage.Registry(); got != nil {
		t.Errorf("Registry() = %v, want nil", got)
	}
}

func TestLookupNouseage(t *testing.T) {
	if _, ok := usage.Lookup("claude"); ok {
		t.Error("Lookup(\"claude\") ok = true, want false")
	}
}

func TestRootFetchNouseage(t *testing.T) {
	_, err := usage.Fetch(context.Background(), []string{"claude"}, usage.Options{})
	if !errors.Is(err, usage.ErrUsageCompiledOut) {
		t.Errorf("Fetch err = %v, want errors.Is(err, ErrUsageCompiledOut)", err)
	}
}

func TestRootCacheDirNouseage(t *testing.T) {
	_, err := usage.CacheDir()
	if !errors.Is(err, usage.ErrUsageCompiledOut) {
		t.Errorf("usage.CacheDir err = %v, want errors.Is(err, ErrUsageCompiledOut)", err)
	}
}

func TestCacheCacheDirNouseage(t *testing.T) {
	_, err := cache.CacheDir()
	if !errors.Is(err, usage.ErrUsageCompiledOut) {
		t.Errorf("cache.CacheDir err = %v, want errors.Is(err, ErrUsageCompiledOut)", err)
	}
}

func TestFetchAllNouseage(t *testing.T) {
	snaps, warns, err := FetchAll(context.Background(), []string{"claude", "codex"}, Options{})
	if snaps != nil {
		t.Errorf("snapshots = %v, want nil", snaps)
	}
	if warns != nil {
		t.Errorf("warnings = %v, want nil", warns)
	}
	if !errors.Is(err, usage.ErrUsageCompiledOut) {
		t.Errorf("err = %v, want errors.Is(err, ErrUsageCompiledOut)", err)
	}
}

func TestFetchAllOptionsIgnoredNouseage(t *testing.T) {
	opts := Options{
		Refresh:      true,
		Offline:      true,
		MaxAge:       time.Hour,
		ShowIdentity: true,
		Enabled:      map[string]bool{"claude": true},
		Timeout:      10 * time.Second,
		MaxParallel:  4,
	}
	snaps, warns, err := FetchAll(context.Background(), []string{"claude"}, opts)
	if snaps != nil {
		t.Errorf("snapshots = %v, want nil", snaps)
	}
	if warns != nil {
		t.Errorf("warnings = %v, want nil", warns)
	}
	if !errors.Is(err, usage.ErrUsageCompiledOut) {
		t.Errorf("err = %v, want errors.Is(err, ErrUsageCompiledOut)", err)
	}
}

func TestWarningUsableNouseage(t *testing.T) {
	if got := (credential.Warning{Message: "boom"}).Message; got != "boom" {
		t.Errorf("Warning.Message = %q, want %q", got, "boom")
	}
}
