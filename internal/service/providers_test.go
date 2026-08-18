package service

import (
	"context"
	"encoding/json"
	"errors"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/WD-Mitchell/which-model/internal/config"
	"github.com/WD-Mitchell/which-model/internal/routing"
	"github.com/WD-Mitchell/which-model/internal/usage"
	"github.com/WD-Mitchell/which-model/internal/usage/cache"
)

// providersFloatPtr returns a pointer to v for snapshot windows.
func providersFloatPtr(v float64) *float64 { return &v }

// useTempUsageCache points the fixture's usage cache at a temp subdir under
// the fixture cache root (isolation: never the real OS usage cache) and
// returns that dir.
func useTempUsageCache(t *testing.T, svc *Services) string {
	t.Helper()
	dir := filepath.Join(svc.paths.CacheDir, "usage-cache")
	svc.usageCacheDir = dir
	return dir
}

// seedUsage writes a fresh, valid cache file for one provider via cache.Store.Write.
func seedUsage(t *testing.T, svc *Services, provider string, snap usage.Snapshot) {
	t.Helper()
	store := cache.Store{Dir: useTempUsageCache(t, svc)}
	if err := store.Write(provider, snap); err != nil {
		t.Fatalf("seed cache %q: %v", provider, err)
	}
}

// writeRawUsageCache writes a cacheFile-shaped JSON (snapshot + fetched_at)
// directly, e.g. for a failure-carrying snapshot (Store.Write refuses failures)
// or a stale snapshot (old fetched_at).
func writeRawUsageCache(t *testing.T, svc *Services, provider string, fetchedAt time.Time, snap usage.Snapshot) {
	t.Helper()
	dir := useTempUsageCache(t, svc)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir usage cache: %v", err)
	}
	cf := struct {
		Snapshot  usage.Snapshot `json:"snapshot"`
		FetchedAt time.Time      `json:"fetched_at"`
	}{Snapshot: snap, FetchedAt: fetchedAt}
	data, err := json.Marshal(cf)
	if err != nil {
		t.Fatalf("marshal cache: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, provider+".json"), data, 0o600); err != nil {
		t.Fatalf("write cache %q: %v", provider, err)
	}
}

// infoByID returns the ProviderInfo for id in list.
func infoByID(t *testing.T, list []ProviderInfo, id string) ProviderInfo {
	t.Helper()
	for _, info := range list {
		if info.ID == id {
			return info
		}
	}
	t.Fatalf("provider %q not in list %#v", id, list)
	return ProviderInfo{}
}

// assertConfigChanged asserts rec recorded exactly one event with the given
// section payload.
func assertConfigChanged(t *testing.T, rec *emitRecorder, section string) {
	t.Helper()
	events := rec.Events()
	if len(events) != 1 {
		t.Fatalf("events = %#v, want exactly 1", events)
	}
	if events[0].Event != EventConfigChanged {
		t.Fatalf("event = %q, want %q", events[0].Event, EventConfigChanged)
	}
	want := map[string]string{"section": section}
	if !reflect.DeepEqual(events[0].Payload, want) {
		t.Fatalf("payload = %#v, want %#v", events[0].Payload, want)
	}
}

// assertZeroEvents asserts rec recorded no events.
func assertZeroEvents(t *testing.T, rec *emitRecorder) {
	t.Helper()
	if events := rec.Events(); len(events) != 0 {
		t.Fatalf("events = %#v, want none", events)
	}
}

// providersCheckErr asserts err wraps sentinel and its message equals wantMsg.
func providersCheckErr(t *testing.T, err error, sentinel error, wantMsg string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("errors.Is(%v, %v) = false; err = %q", err, sentinel, err)
	}
	if got := err.Error(); got != wantMsg {
		t.Fatalf("error message = %q, want %q", got, wantMsg)
	}
}

// reloadConfig re-reads the fixture config from disk.
func reloadConfig(t *testing.T, svc *Services) *config.Config {
	t.Helper()
	cfg, err := config.Load(config.LoadOptions{Path: svc.paths.UserConfigFile, Getenv: func(string) string { return "" }})
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	return cfg
}

func TestProvidersList_OrderAndUniverse(t *testing.T) {
	svc, _ := newTestServices(t,
		WithConfigTOML(`
[usage]
backend = "native"

[providers.codex]
enabled = true
priority = 1

[providers.claude]
enabled = true
priority = 2

[providers.extra]
source_preference = ["live"]
`),
		WithRoutes(tableWithExtraProviders()),
	)
	useTempUsageCache(t, svc)

	list, err := svc.Providers().List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	wantIDs := []string{"extra", "zeta", "codex", "claude"} // priorities: extra=0, zeta=0, codex=1, claude=2; 0-tie broken by id asc
	gotIDs := make([]string, 0, len(list))
	for i, info := range list {
		gotIDs = append(gotIDs, info.ID)
		if info.Priority != i+1 {
			t.Fatalf("provider %q Priority = %d, want %d", info.ID, info.Priority, i+1)
		}
	}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Fatalf("order = %#v, want %#v", gotIDs, wantIDs)
	}

	// config-only provider: RoutesTotal 0, default-deny off.
	extra := infoByID(t, list, "extra")
	if extra.Enabled {
		t.Fatal("config-only provider should be disabled by default")
	}
	if extra.RoutesTotal != 0 {
		t.Fatalf("extra RoutesTotal = %d, want 0", extra.RoutesTotal)
	}

	// table-only provider: default-deny off, RoutesTotal from table.
	zeta := infoByID(t, list, "zeta")
	if zeta.Enabled {
		t.Fatal("table-only provider should be disabled by default")
	}
	if zeta.RoutesTotal != 1 {
		t.Fatalf("zeta RoutesTotal = %d, want 1", zeta.RoutesTotal)
	}

	// providers present in both config and table are enabled.
	if !infoByID(t, list, "claude").Enabled || !infoByID(t, list, "codex").Enabled {
		t.Fatal("claude/codex should be enabled from config")
	}
}

func TestProvidersList_LimitsLine(t *testing.T) {
	t.Run("not enabled", func(t *testing.T) {
		// claude disabled but has a seeded cache: line "not enabled", usage
		// fields still populated from cache.
		svc, _ := newTestServices(t, WithConfigTOML(`
[usage]
backend = "native"

[providers.codex]
enabled = true
priority = 2

[providers.claude]
priority = 1
`))
		seedUsage(t, svc, "claude", seededSnapshot("claude"))
		info := infoByID(t, mustProviderList(t, svc), "claude")
		if info.LimitsLine != "not enabled" {
			t.Fatalf("LimitsLine = %q, want %q", info.LimitsLine, "not enabled")
		}
		if info.Session == nil || *info.Session != 42 {
			t.Fatalf("Session = %v, want 42", info.Session)
		}
		if info.Auth != "oauth" {
			t.Fatalf("Auth = %q, want oauth", info.Auth)
		}
	})

	t.Run("usage off", func(t *testing.T) {
		// backend off (default) => global usage disabled.
		svc, _ := newTestServices(t, WithConfigTOML(`
[providers.claude]
enabled = true
priority = 1
`))
		seedUsage(t, svc, "claude", seededSnapshot("claude"))
		info := infoByID(t, mustProviderList(t, svc), "claude")
		if info.LimitsLine != "usage off" {
			t.Fatalf("LimitsLine = %q, want %q", info.LimitsLine, "usage off")
		}
		if info.Session != nil || info.Credits != "" || info.Resets != "" || info.Auth != "" {
			t.Fatalf("usage fields not cleared: %#v", info)
		}
	})

	t.Run("no usage data", func(t *testing.T) {
		// usage on but no usable snapshot (missing cache file).
		svc, _ := newTestServices(t, WithConfigTOML(`
[usage]
backend = "native"

[providers.claude]
enabled = true
priority = 1
`))
		useTempUsageCache(t, svc) // empty dir => OfflineRead fallback failure
		info := infoByID(t, mustProviderList(t, svc), "claude")
		if info.LimitsLine != "no usage data" {
			t.Fatalf("LimitsLine = %q, want %q", info.LimitsLine, "no usage data")
		}
		if info.Session != nil || info.Auth != "" {
			t.Fatalf("usage fields not cleared: %#v", info)
		}
	})

	t.Run("failed cache", func(t *testing.T) {
		svc, _ := newTestServices(t, WithConfigTOML(`
[usage]
backend = "native"

[providers.claude]
enabled = true
priority = 1
`))
		writeRawUsageCache(t, svc, "claude", time.Now(), usage.Snapshot{
			Provider: "claude",
			Failure:  &usage.Failure{Code: "boom", Message: "boom"},
		})
		info := infoByID(t, mustProviderList(t, svc), "claude")
		if info.LimitsLine != "no usage data" {
			t.Fatalf("LimitsLine = %q, want %q", info.LimitsLine, "no usage data")
		}
	})

	t.Run("seeded", func(t *testing.T) {
		svc, _ := newTestServices(t, WithConfigTOML(`
[usage]
backend = "native"

[providers.claude]
enabled = true
priority = 1
`))
		seedUsage(t, svc, "claude", seededSnapshot("claude"))
		info := infoByID(t, mustProviderList(t, svc), "claude")
		if info.LimitsLine != "session 42% · weekly 18% · monthly 3 of 10 · 340 credits" {
			t.Fatalf("LimitsLine = %q", info.LimitsLine)
		}
		if info.Session == nil || *info.Session != 42 {
			t.Fatalf("Session = %v, want 42", info.Session)
		}
		if info.Weekly == nil || *info.Weekly != 18 {
			t.Fatalf("Weekly = %v, want 18", info.Weekly)
		}
		if info.Monthly != nil {
			t.Fatalf("Monthly = %v, want nil (uncomputable percent)", info.Monthly)
		}
		if info.Credits != "340 credits" {
			t.Fatalf("Credits = %q, want %q", info.Credits, "340 credits")
		}
		if info.Resets != "weekly 02:00 UTC" {
			t.Fatalf("Resets = %q, want %q", info.Resets, "weekly 02:00 UTC")
		}
		if info.Auth != "oauth" {
			t.Fatalf("Auth = %q, want oauth", info.Auth)
		}
	})
}

// seededSnapshot is a usage snapshot exercising §5: session/weekly percents,
// a monthly "x of y" fallback (synthetic => uncomputable percent), a credits
// window, a reset hint, and an oauth source.
func seededSnapshot(provider string) usage.Snapshot {
	return usage.Snapshot{
		Provider: provider,
		Source:   usage.SourceOAuth,
		Windows: []usage.Window{
			{ID: "session", Unit: usage.UnitPercent, UsedPercent: providersFloatPtr(42), UsageKnown: true},
			{ID: "weekly", Unit: usage.UnitPercent, UsedPercent: providersFloatPtr(18), ResetHint: "02:00 UTC", UsageKnown: true},
			{ID: "monthly", Unit: usage.UnitPercent, Synthetic: true, Used: providersFloatPtr(3), Limit: providersFloatPtr(10)},
			{ID: "credits", Unit: usage.UnitCredits, Remaining: providersFloatPtr(340)},
		},
	}
}

func mustProviderList(t *testing.T, svc *Services) []ProviderInfo {
	t.Helper()
	list, err := svc.Providers().List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	return list
}

func TestProvidersList_NoFetch(t *testing.T) {
	// Usage fields are populated exclusively from the offline cache: a stale
	// cache file (older than the default 24h TTL) still yields its values.
	svc, _ := newTestServices(t, WithConfigTOML(`
[usage]
backend = "native"

[providers.claude]
enabled = true
priority = 1
`))
	stale := seededSnapshot("claude")
	// drop the uncomputable synthetic window so Session is the only field we
	// assert, but keep stale by writing an old fetched_at.
	stale.Windows = []usage.Window{{ID: "session", Unit: usage.UnitPercent, UsedPercent: providersFloatPtr(42), UsageKnown: true}}
	writeRawUsageCache(t, svc, "claude", time.Now().Add(-48*time.Hour), stale)

	info := infoByID(t, mustProviderList(t, svc), "claude")
	if info.Session == nil || *info.Session != 42 {
		t.Fatalf("Session = %v, want 42 (stale cache must still populate)", info.Session)
	}
	if info.Auth != "oauth" {
		t.Fatalf("Auth = %q, want oauth", info.Auth)
	}
}

// TestProvidersTestNoFetchImport guards the CONTRACTS §8 compile-time
// guarantee: providers_test.go must not import internal/usage/fetch.
func TestProvidersTestNoFetchImport(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "providers_test.go", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse providers_test.go: %v", err)
	}
	for _, imp := range f.Imports {
		if imp.Path.Value == `"github.com/WD-Mitchell/which-model/internal/usage/fetch"` {
			t.Fatal("providers_test.go must not import internal/usage/fetch")
		}
	}
}

func TestProviderSetEnabled_Persists(t *testing.T) {
	svc, rec := newTestServices(t, WithConfigTOML(`
[providers.claude]
enabled = true
priority = 1
`))
	useTempUsageCache(t, svc)
	ctx := context.Background()

	// toggle off -> persists + emits.
	if err := svc.Providers().SetEnabled(ctx, "claude", false); err != nil {
		t.Fatalf("SetEnabled: %v", err)
	}
	assertConfigChanged(t, rec, "providers")
	rel := reloadConfig(t, svc)
	if rel.Providers["claude"].Enabled {
		t.Fatal("claude.enabled should be false on disk")
	}

	// idempotent on again.
	if err := svc.Providers().SetEnabled(ctx, "claude", true); err != nil {
		t.Fatalf("SetEnabled: %v", err)
	}
	if got := len(rec.Events()); got != 2 {
		t.Fatalf("events = %d, want 2", got)
	}
	rel = reloadConfig(t, svc)
	if !rel.Providers["claude"].Enabled {
		t.Fatal("claude.enabled should be true on disk")
	}
}

func TestProviderSetEnabled_Unknown(t *testing.T) {
	svc, rec := newTestServices(t)
	useTempUsageCache(t, svc)
	err := svc.Providers().SetEnabled(context.Background(), "nope", true)
	providersCheckErr(t, err, errNotFound, `not found: providers: unknown provider "nope"`)
	assertZeroEvents(t, rec)
	// no write happened.
	rel := reloadConfig(t, svc)
	if _, ok := rel.Providers["nope"]; ok {
		t.Fatal("unknown provider must not be written")
	}
}

func TestProvidersReorder_RoundTrip(t *testing.T) {
	svc, rec := newTestServices(t, WithConfigTOML(`
[providers.claude]
priority = 5

[providers.codex]
priority = 3
`))
	useTempUsageCache(t, svc)
	ctx := context.Background()

	if err := svc.Providers().Reorder(ctx, []string{"codex", "claude"}); err != nil {
		t.Fatalf("Reorder: %v", err)
	}
	assertConfigChanged(t, rec, "providers")

	list := mustProviderList(t, svc)
	gotIDs := make([]string, 0, len(list))
	for _, info := range list {
		gotIDs = append(gotIDs, info.ID)
	}
	if want := []string{"codex", "claude"}; !reflect.DeepEqual(gotIDs, want) {
		t.Fatalf("after reorder ids = %#v, want %#v", gotIDs, want)
	}

	rel := reloadConfig(t, svc)
	if rel.Providers["codex"].Priority != 1 || rel.Providers["claude"].Priority != 2 {
		t.Fatalf("on-disk priorities = codex:%d claude:%d, want 1,2", rel.Providers["codex"].Priority, rel.Providers["claude"].Priority)
	}

	// A second List after reload is identical.
	if !reflect.DeepEqual(mustProviderList(t, svc), list) {
		t.Fatal("second List differs after reload")
	}
}

func TestProvidersReorder_RejectsWrongSet(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name    string
		ordered []string
		wantMsg string
	}{
		{"duplicate", []string{"claude", "claude"}, `validation failed: providers: reorder list contains duplicate id "claude"`},
		{"unknown", []string{"claude", "bogus"}, `validation failed: providers: unknown provider "bogus"`},
		{"missing", []string{"claude"}, `validation failed: providers: reorder list must contain every provider exactly once (got 1, want 3)`},
		{"length", []string{"claude", "codex"}, `validation failed: providers: reorder list must contain every provider exactly once (got 2, want 3)`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, rec := newTestServices(t, WithConfigTOML("[providers.claude]\n[providers.codex]\n[providers.extra]\n"))
			useTempUsageCache(t, svc)
			err := svc.Providers().Reorder(ctx, tc.ordered)
			providersCheckErr(t, err, errValidation, tc.wantMsg)
			assertZeroEvents(t, rec)
			rel := reloadConfig(t, svc)
			if pri := rel.Providers["claude"].Priority; pri != 0 {
				t.Fatalf("claude.priority changed to %d, want untouched 0", pri)
			}
		})
	}
}

func TestProviderDetail_LevelsAndDefault(t *testing.T) {
	svc, _ := newTestServices(t,
		WithConfigTOML("[routes.disabled]\nclaude = [\"claude-opus-5@max\"]\n"),
		WithRoutes(detailRoutes()),
	)
	detail, err := svc.Providers().Detail(context.Background(), "claude")
	if err != nil {
		t.Fatalf("Detail: %v", err)
	}
	if detail.ID != "claude" {
		t.Fatalf("Detail.ID = %q", detail.ID)
	}
	want := ProviderDetail{
		ID: "claude",
		Models: []ProviderModel{
			{ModelID: "claude-eco", ModelName: "Claude Eco", Levels: []RouteLevel{
				// low then default (collapsed to high = the top present rung).
				{Reasoning: "low", Enabled: true, Default: false},
				{Reasoning: "default", Enabled: true, Default: true},
			}},
			{ModelID: "claude-opus-5", ModelName: "Claude Opus 5", Levels: []RouteLevel{
				{Reasoning: "high", Enabled: true, Default: false},
				{Reasoning: "max", Enabled: false, Default: true}, // max disabled via routes.disabled
			}},
			{ModelID: "claude-sonnet-4", ModelName: "Claude Sonnet 4", Levels: []RouteLevel{
				{Reasoning: "medium", Enabled: true, Default: true},
			}},
		},
	}
	if !reflect.DeepEqual(detail, want) {
		t.Fatalf("Detail = %#v, want %#v", detail, want)
	}
}

func TestProviderDetail_Unknown(t *testing.T) {
	svc, rec := newTestServices(t)
	useTempUsageCache(t, svc)
	_, err := svc.Providers().Detail(context.Background(), "nope")
	providersCheckErr(t, err, errNotFound, `not found: providers: unknown provider "nope"`)
	assertZeroEvents(t, rec)
}

func TestProviderRoutes_DisabledArithmetic(t *testing.T) {
	ctx := context.Background()

	t.Run("add off", func(t *testing.T) {
		svc, rec := newTestServices(t)
		useTempUsageCache(t, svc)
		if err := svc.Providers().SetRouteEnabled(ctx, "claude", "claude-opus-5", "max", false); err != nil {
			t.Fatalf("SetRouteEnabled: %v", err)
		}
		assertConfigChanged(t, rec, "routes")
		info := infoByID(t, mustProviderList(t, svc), "claude")
		if info.RoutesTotal != 3 || info.RoutesOn != 2 {
			t.Fatalf("claude RoutesTotal/RoutesOn = %d/%d, want 3/2", info.RoutesTotal, info.RoutesOn)
		}
		disabled, _ := reloadConfig(t, svc).LoadRoutesDisabled()
		if got := disabled["claude"]; !reflect.DeepEqual(got, []string{"claude-opus-5@max"}) {
			t.Fatalf("disabled.claude = %#v", got)
		}
	})

	t.Run("dedup on add", func(t *testing.T) {
		svc, rec := newTestServices(t)
		useTempUsageCache(t, svc)
		// add then re-add: no duplicate, still one event per mutation.
		if err := svc.Providers().SetRouteEnabled(ctx, "claude", "claude-sonnet-4", "medium", false); err != nil {
			t.Fatal(err)
		}
		if err := svc.Providers().SetRouteEnabled(ctx, "claude", "claude-sonnet-4", "medium", false); err != nil {
			t.Fatal(err)
		}
		if got := len(rec.Events()); got != 2 {
			t.Fatalf("events = %d, want 2", got)
		}
		disabled, _ := reloadConfig(t, svc).LoadRoutesDisabled()
		if got := disabled["claude"]; !reflect.DeepEqual(got, []string{"claude-sonnet-4@medium"}) {
			t.Fatalf("disabled.claude = %#v, want deduped", got)
		}
	})

	t.Run("remove entry re-enables", func(t *testing.T) {
		svc, _ := newTestServices(t, WithConfigTOML("[routes.disabled]\nclaude = [\"claude-opus-5@max\"]\n"))
		useTempUsageCache(t, svc)
		if err := svc.Providers().SetRouteEnabled(ctx, "claude", "claude-opus-5", "max", true); err != nil {
			t.Fatal(err)
		}
		info := infoByID(t, mustProviderList(t, svc), "claude")
		if info.RoutesOn != 3 {
			t.Fatalf("after re-enable RoutesOn = %d, want 3", info.RoutesOn)
		}
	})

	t.Run("stale entry survives", func(t *testing.T) {
		svc, _ := newTestServices(t, WithConfigTOML("[routes.disabled]\nclaude = [\"gpt-999@low\", \"claude-opus-5@high\"]\n"))
		useTempUsageCache(t, svc)
		// unmatched stale entry (gpt-999@low) subtracts nothing.
		info := infoByID(t, mustProviderList(t, svc), "claude")
		if info.RoutesTotal != 3 || info.RoutesOn != 2 {
			t.Fatalf("RoutesTotal/RoutesOn = %d/%d, want 3/2 (only opus-5@high matches)", info.RoutesTotal, info.RoutesOn)
		}
		// a write must preserve the unmatched stale entry.
		if err := svc.Providers().SetRouteEnabled(ctx, "claude", "claude-sonnet-4", "medium", false); err != nil {
			t.Fatal(err)
		}
		disabled, _ := reloadConfig(t, svc).LoadRoutesDisabled()
		if got := disabled["claude"]; !reflect.DeepEqual(got, []string{"claude-opus-5@high", "claude-sonnet-4@medium", "gpt-999@low"}) {
			t.Fatalf("disabled.claude = %#v, want stale gpt-999@low retained", got)
		}
	})

	t.Run("set all routes on", func(t *testing.T) {
		svc, rec := newTestServices(t, WithConfigTOML("[routes.disabled]\nclaude = [\"claude-opus-5@max\"]\ncodex = [\"gpt-5.6@high\"]\n"))
		useTempUsageCache(t, svc)
		if err := svc.Providers().SetAllRoutes(ctx, "claude", true); err != nil {
			t.Fatal(err)
		}
		assertConfigChanged(t, rec, "routes")
		disabled, _ := reloadConfig(t, svc).LoadRoutesDisabled()
		if _, ok := disabled["claude"]; ok {
			t.Fatalf("SetAllRoutes(true) should delete the key; got %#v", disabled["claude"])
		}
		if _, ok := disabled["codex"]; !ok {
			t.Fatal("other provider's disabled list must survive")
		}
	})

	t.Run("set all routes off", func(t *testing.T) {
		svc, _ := newTestServices(t)
		useTempUsageCache(t, svc)
		if err := svc.Providers().SetAllRoutes(ctx, "codex", false); err != nil {
			t.Fatal(err)
		}
		disabled, _ := reloadConfig(t, svc).LoadRoutesDisabled()
		if want := []string{"gpt-4.2@low", "gpt-5.6@high", "gpt-5.6@medium"}; !reflect.DeepEqual(disabled["codex"], want) {
			t.Fatalf("disabled.codex = %#v, want %#v (sorted full list)", disabled["codex"], want)
		}
	})

	t.Run("errors", func(t *testing.T) {
		svc, rec := newTestServices(t)
		useTempUsageCache(t, svc)

		// unknown provider -> not_found, zero events.
		err := svc.Providers().SetRouteEnabled(ctx, "nope", "x", "high", false)
		providersCheckErr(t, err, errNotFound, `not found: providers: unknown provider "nope"`)
		// unknown route triple -> not_found, zero events.
		err = svc.Providers().SetRouteEnabled(ctx, "claude", "no-such-model", "high", false)
		providersCheckErr(t, err, errNotFound, `not found: providers: no route claude/no-such-model@high`)
		// unknown provider for SetAllRoutes -> not_found.
		err = svc.Providers().SetAllRoutes(ctx, "nope", true)
		providersCheckErr(t, err, errNotFound, `not found: providers: unknown provider "nope"`)
		assertZeroEvents(t, rec)
	})
}

// withAddedRoutes returns base.Routes plus the appended routes.
func withAddedRoutes(base routing.Table, added ...routing.Route) routing.Table {
	return routing.Table{
		SchemaVersion: routing.TableSchemaVersion,
		Routes:        append(append([]routing.Route(nil), base.Routes...), added...),
	}
}

// tableWithExtraProviders returns the default table plus a table-only
// provider "zeta" (present in routes, absent from config).
func tableWithExtraProviders() routing.Table {
	return withAddedRoutes(defaultRoutes(),
		routing.Route{Provider: "zeta", ModelID: "zeta-model", Model: "Zeta", Reasoning: "low", Provenance: routing.ProvenanceProviderLive},
	)
}

// detailRoutes exercises levels/default logic including "default" collapsing
// to "high", present-level-only enumeration, and a disabled route.
func detailRoutes() routing.Table {
	return withAddedRoutes(defaultRoutes(),
		routing.Route{Provider: "claude", ModelID: "claude-eco", Model: "Claude Eco", Reasoning: "low", Provenance: routing.ProvenanceProviderLive},
		routing.Route{Provider: "claude", ModelID: "claude-eco", Model: "Claude Eco", Reasoning: "default", Provenance: routing.ProvenanceProviderLive},
	)
}
