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
	"github.com/WD-Mitchell/which-model/internal/usage/credential"
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
	seedModelsDev(t, svc, `[
		{"Provider":"anthropic","ModelID":"claude-opus-5","Name":"Claude Opus 5"},
		{"Provider":"anthropic","ModelID":"claude-haiku-4","Name":"Claude Haiku 4"}
	]`)

	list, err := svc.Providers().List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	wantIDs := []string{"anthropic", "extra", "zeta", "codex", "claude"} // priority 0 ties are id ascending, then configured priorities 1 and 2
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

	if extra.Models != 0 {
		t.Fatalf("extra Models = %d, want 0", extra.Models)
	}
	if zeta.Models != 1 {
		t.Fatalf("zeta Models = %d, want 1", zeta.Models)
	}
	if anthropicModels := infoByID(t, list, "anthropic").Models; anthropicModels != 2 {
		t.Fatalf("anthropic Models = %d, want 2 catalogue model ids", anthropicModels)
	}
	if claudeModels := infoByID(t, list, "claude").Models; claudeModels != 3 {
		t.Fatalf("claude Models = %d, want 3 distinct catalogue and routed model ids", claudeModels)
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
func TestProviderSetEnabled_CatalogueOnly(t *testing.T) {
	svc, rec := newTestServices(t)
	useTempUsageCache(t, svc)
	seedModelsDev(t, svc, `[{"Provider":"alibaba","ModelID":"qwen3-max","Name":"Qwen3 Max"}]`)

	if err := svc.Providers().SetEnabled(context.Background(), "alibaba", true); err != nil {
		t.Fatalf("SetEnabled catalogue-only provider: %v", err)
	}
	assertConfigChanged(t, rec, "providers")
	if !reloadConfig(t, svc).Providers["alibaba"].Enabled {
		t.Fatal("alibaba.enabled should be true on disk")
	}
}

func TestProviderListIncludesCodexBarProviders(t *testing.T) {
	oldDiscover := discoverBackendProviderIDs
	t.Cleanup(func() { discoverBackendProviderIDs = oldDiscover })
	discoverBackendProviderIDs = func(backend config.UsageBackend) []string {
		if backend != config.UsageBackendCodexBar {
			t.Fatalf("backend = %q, want codexbar", backend)
		}
		return []string{"antigravity", "commandcode", "cursor"}
	}
	svc, _ := newTestServices(t, WithConfigTOML("[usage]\nbackend = \"codexbar\"\n"))
	useTempUsageCache(t, svc)

	list := mustProviderList(t, svc)
	got := make(map[string]bool, len(list))
	for _, provider := range list {
		got[provider.ID] = true
	}
	for _, id := range []string{"antigravity", "commandcode", "cursor"} {
		if !got[id] {
			t.Errorf("List missing CodexBar provider %q", id)
		}
	}
	if err := svc.Providers().SetEnabled(context.Background(), "antigravity", true); err != nil {
		t.Fatalf("SetEnabled CodexBar provider: %v", err)
	}
	if !reloadConfig(t, svc).Providers["antigravity"].Enabled {
		t.Fatal("antigravity.enabled should be true on disk")
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
		ID:             "claude",
		Builtin:        true,
		OAuthSupported: true,
		Accounts:       []ProviderAccountDTO{},
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

// seedModelsDev writes a models.dev providers catalogue cache for Detail's
// augmentation path (same file Addable reads).
func seedModelsDev(t *testing.T, svc *Services, rows string) {
	t.Helper()
	dir := filepath.Join(svc.paths.CacheDir, "catalog")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "modelsdev_providers.json"), []byte(rows), 0o600); err != nil {
		t.Fatal(err)
	}
}

// Detail shows the provider's FULL models.dev model list, not just the
// benchmarked models that produced routes: unrouted catalogue models append
// with nil Levels, and names come from models.dev (routes-only fallback when
// no catalogue cache exists).
func TestProviderDetail_IncludesCatalogueModels(t *testing.T) {
	svc, _ := newTestServices(t,
		WithConfigTOML("[routes.disabled]\nclaude = [\"claude-opus-5@max\"]\n"),
		WithRoutes(detailRoutes()),
	)
	seedModelsDev(t, svc, `[
		{"Provider":"anthropic","ModelID":"claude-eco","Name":"Claude Eco"},
		{"Provider":"anthropic","ModelID":"claude-haiku-4","Name":"Claude Haiku 4"},
		{"Provider":"anthropic","ModelID":"claude-opus-5","Name":"Claude Opus 5"},
		{"Provider":"openai","ModelID":"gpt-5.6","Name":"GPT-5.6"}
	]`)
	detail, err := svc.Providers().Detail(context.Background(), "claude")
	if err != nil {
		t.Fatalf("Detail: %v", err)
	}
	var gotIDs []string
	for _, m := range detail.Models {
		gotIDs = append(gotIDs, m.ModelID)
	}
	// Routed models keep their levels; the unrouted catalogue model
	// (claude-haiku-4) joins with a default level so it can be opened.
	// The openai row belongs to a different slug and must not leak in.
	wantIDs := []string{"claude-eco", "claude-haiku-4", "claude-opus-5", "claude-sonnet-4"}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Fatalf("Detail model ids = %v, want %v", gotIDs, wantIDs)
	}
	for _, m := range detail.Models {
		if m.ModelID == "claude-haiku-4" {
			if m.ModelName != "Claude Haiku 4" {
				t.Errorf("unrouted ModelName = %q, want models.dev name %q", m.ModelName, "Claude Haiku 4")
			}
			if len(m.Levels) != 1 || m.Levels[0].Reasoning != "default" {
				t.Errorf("unrouted Levels = %#v, want default", m.Levels)
			}
		}
	}
}

func TestProviderDetail_NoCatalogueCacheIsRoutesOnly(t *testing.T) {
	svc, _ := newTestServices(t, WithRoutes(detailRoutes()))
	detail, err := svc.Providers().Detail(context.Background(), "claude")
	if err != nil {
		t.Fatalf("Detail: %v", err)
	}
	if len(detail.Models) != 3 {
		t.Fatalf("Detail models = %d, want 3 (routes only)", len(detail.Models))
	}
}

func TestProviderDetail_CatalogueEffortLevels(t *testing.T) {
	svc, _ := newTestServices(t, WithRoutes(detailRoutes()))
	seedModelsDev(t, svc, `[
		{"Provider":"anthropic","ModelID":"claude-haiku-4","Name":"Claude Haiku 4","EffortLevels":["low","medium","high"]},
		{"Provider":"anthropic","ModelID":"claude-opus-5","Name":"Claude Opus 5","EffortLevels":["low","high","max"]}
	]`)
	detail, err := svc.Providers().Detail(context.Background(), "claude")
	if err != nil {
		t.Fatalf("Detail: %v", err)
	}
	byID := map[string]ProviderModel{}
	for _, m := range detail.Models {
		byID[m.ModelID] = m
	}
	haiku := byID["claude-haiku-4"]
	if haiku.ModelID == "" {
		t.Fatal("missing claude-haiku-4")
	}
	got := make([]string, 0, len(haiku.Levels))
	for _, l := range haiku.Levels {
		got = append(got, l.Reasoning)
	}
	want := []string{"low", "medium", "high"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("haiku levels = %v, want %v (catalogue, not scores)", got, want)
	}
	opus := byID["claude-opus-5"]
	got = nil
	for _, l := range opus.Levels {
		got = append(got, l.Reasoning)
	}
	want = []string{"low", "high", "max"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("opus levels = %v, want %v", got, want)
	}
	if err := svc.Providers().SetRouteEnabled(context.Background(), "claude", "claude-haiku-4", "low", false); err != nil {
		t.Fatalf("SetRouteEnabled catalogue-only combo: %v", err)
	}
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

func TestProviderAdd(t *testing.T) {
	svc, rec := newTestServices(t, WithConfigTOML(`
[providers.claude]
enabled = true
priority = 1
`))
	useTempUsageCache(t, svc)
	ctx := context.Background()

	// A new id persists default-deny and shows up in the universe.
	if err := svc.Providers().Add(ctx, "myprovider"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	assertConfigChanged(t, rec, "providers")
	rel := reloadConfig(t, svc)
	if _, ok := rel.Providers["myprovider"]; !ok {
		t.Fatal("myprovider missing from config on disk")
	}
	if rel.Providers["myprovider"].Enabled {
		t.Error("a new provider must be default-deny (enabled=false)")
	}

	// Re-adding anything already in the universe is a conflict, not a reset.
	if err := svc.Providers().Add(ctx, "myprovider"); !errors.Is(err, errConflict) {
		t.Errorf("re-add err = %v, want errConflict", err)
	}
	if err := svc.Providers().Add(ctx, "claude"); !errors.Is(err, errConflict) {
		t.Errorf("add existing err = %v, want errConflict", err)
	}

	// Ids must be config-key safe.
	for _, bad := range []string{"", "  ", "Has Spaces", "UPPER!", "-", "a/b"} {
		if err := svc.Providers().Add(ctx, bad); !errors.Is(err, errValidation) {
			t.Errorf("Add(%q) err = %v, want errValidation", bad, err)
		}
	}
}

func TestProviderDeleteDuplicateAndAccounts(t *testing.T) {
	svc, rec := newTestServices(t, WithConfigTOML(`
[providers.claude]
enabled = true
priority = 1
`))
	useTempUsageCache(t, svc)
	ctx := context.Background()
	ps := svc.Providers()

	// A builtin ships a usage adapter and stays in the universe whatever the
	// config says, so deleting it would look like a no-op. Refuse instead.
	//
	// Nothing is builtin by default in this package's tests: descriptors come
	// from the blank imports in the binaries, not from internal/service. So the
	// case is set up explicitly, under an id no real adapter uses.
	const builtinID = "wm_test_builtin"
	registerTestProvider(t, builtinID)
	if err := ps.Delete(ctx, builtinID); !errors.Is(err, errValidation) {
		t.Errorf("Delete(builtin) err = %v, want errValidation", err)
	}

	if err := ps.Add(ctx, "myprov"); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Accounts: validated, then persisted as a set.
	bad := []ProviderAccountDTO{{Name: "Work", Kind: "smoke-signal"}}
	if err := ps.SetAccounts(ctx, "myprov", bad); !errors.Is(err, errValidation) {
		t.Errorf("SetAccounts(bad kind) err = %v, want errValidation", err)
	}
	if err := ps.SetAccounts(ctx, "myprov", []ProviderAccountDTO{{Name: " ", Kind: AccountKindToken}}); !errors.Is(err, errValidation) {
		t.Errorf("SetAccounts(blank name) err = %v, want errValidation", err)
	}
	dupes := []ProviderAccountDTO{{Name: "A", Kind: AccountKindToken}, {Name: "A", Kind: AccountKindOAuth}}
	if err := ps.SetAccounts(ctx, "myprov", dupes); !errors.Is(err, errConflict) {
		t.Errorf("SetAccounts(duplicate name) err = %v, want errConflict", err)
	}

	good := []ProviderAccountDTO{
		{Name: "Work", Kind: AccountKindOAuth, Ref: "~/.creds.json"},
		{Name: "Personal", Kind: AccountKindToken, Ref: "WM_TOKEN"},
	}
	if err := ps.SetAccounts(ctx, "myprov", good); err != nil {
		t.Fatalf("SetAccounts: %v", err)
	}
	detail, err := ps.Detail(ctx, "myprov")
	if err != nil {
		t.Fatalf("Detail: %v", err)
	}
	if len(detail.Accounts) != 2 || detail.Accounts[0].Name != "Work" || detail.Accounts[1].Ref != "WM_TOKEN" {
		t.Errorf("Detail accounts = %+v", detail.Accounts)
	}
	if detail.Builtin {
		t.Error("myprov must not report as builtin")
	}
	if detail.OAuthSupported {
		t.Error("custom provider must not advertise OAuth")
	}

	// Duplicate carries the accounts but never the enabled state.
	copyID, err := ps.Duplicate(ctx, "myprov")
	if err != nil {
		t.Fatalf("Duplicate: %v", err)
	}
	if copyID != "myprov_2" {
		t.Errorf("copy id = %q, want myprov_2", copyID)
	}
	rel := reloadConfig(t, svc)
	if rel.Providers[copyID].Enabled {
		t.Error("a duplicate must be default-deny")
	}
	if len(rel.Providers[copyID].Accounts) != 2 {
		t.Errorf("copy accounts = %+v", rel.Providers[copyID].Accounts)
	}

	// Delete removes a non-builtin outright.
	if err := ps.Delete(ctx, copyID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	rel = reloadConfig(t, svc)
	if _, still := rel.Providers[copyID]; still {
		t.Error("deleted provider still in config")
	}
	if len(rec.Events()) == 0 {
		t.Error("expected config:changed emissions")
	}
}

func TestProviderSetAccountsRemovesManagedCredentialWithLastManagedAccount(t *testing.T) {
	svc, rec := newTestServices(t, WithConfigTOML(`
[auth]
use_keychain = false

[providers.myprov]
enabled = true

[[providers.myprov.accounts]]
name = "Production"
kind = "token"
ref = "which-model"
`))
	ctx := context.Background()
	store := credential.ManagedStore{StateDir: svc.paths.StateDir, UseKeychain: false}
	if err := store.SaveAPIKey("myprov", "sk-managed-provider-test"); err != nil {
		t.Fatal(err)
	}

	if err := svc.Providers().SetAccounts(ctx, "myprov", nil); err != nil {
		t.Fatalf("SetAccounts(remove managed account): %v", err)
	}
	if _, _, err := store.Resolve(ctx, "myprov"); !errors.Is(err, credential.ErrNotFound) {
		t.Fatalf("managed credential still resolves after account removal: %v", err)
	}
	if accounts := reloadConfig(t, svc).Providers["myprov"].Accounts; len(accounts) != 0 {
		t.Fatalf("accounts after removal = %+v", accounts)
	}
	assertConfigChanged(t, rec, "providers")
}

func TestProviderSetAccountsDoesNotDeleteCursorOwnedCredential(t *testing.T) {
	svc, _ := newTestServices(t, WithConfigTOML(`
[auth]
use_keychain = false

[providers.cursor]
enabled = true

[[providers.cursor.accounts]]
name = "Team"
kind = "oauth"
ref = "cursor-agent"
`))
	ctx := context.Background()
	store := credential.ManagedStore{StateDir: svc.paths.StateDir, UseKeychain: false}
	if err := store.Save("cursor", "unrelated-managed-test-token"); err != nil {
		t.Fatal(err)
	}

	if err := svc.Providers().SetAccounts(ctx, "cursor", nil); err != nil {
		t.Fatalf("SetAccounts(remove Cursor account): %v", err)
	}
	managed, _, err := store.Resolve(ctx, "cursor")
	if err != nil || managed.Token != "unrelated-managed-test-token" {
		t.Fatalf("Cursor-owned removal changed managed credential: %#v, %v", managed, err)
	}
}

// registerTestProvider adds a usage descriptor so providerBuiltin sees an id as
// shipped. usage.Register panics on a duplicate id and the registry has no
// removal, so this registers at most once per test binary.
func registerTestProvider(t *testing.T, id string) {
	t.Helper()
	for _, existing := range usage.IDs() {
		if existing == id {
			return
		}
	}
	usage.Register(usage.Descriptor{ID: id, DisplayName: id, Kind: usage.KindSubscription, Tier: 1})
}

func TestProvidersListAuthenticationUsesConfiguredAccounts(t *testing.T) {
	for _, tc := range []struct {
		name     string
		accounts []config.ProviderAccount
		source   usage.Source
		want     string
	}{
		{"oauth overrides API transport", []config.ProviderAccount{{Kind: "oauth", Ref: "which-model"}}, usage.SourceAPI, "oauth"},
		{"oauth without usage", []config.ProviderAccount{{Kind: "oauth", Ref: "which-model"}}, "", "oauth"},
		{"API key overrides OAuth cache", []config.ProviderAccount{{Kind: "token", Ref: "which-model"}}, usage.SourceOAuth, "api"},
		{"cookie", []config.ProviderAccount{{Kind: "cookie", Ref: "external-cookie-reference"}}, usage.SourceAPI, "web"},
		{"mixed and duplicate methods", []config.ProviderAccount{{Kind: "token", Ref: "key"}, {Kind: "oauth", Ref: "one"}, {Kind: "oauth", Ref: "two"}}, usage.SourceAPI, "oauth + api"},
		{"unconfigured method uses usage source", nil, usage.SourceCLI, "cli"},
		{"empty reference is not configured auth", []config.ProviderAccount{{Kind: "oauth"}}, usage.SourceAPI, "api"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := newTestServices(t, WithConfigTOML("[usage]\nbackend = \"codexbar\"\n[providers.claude]\nenabled = true\n"))
			provider := svc.cfg.Providers["claude"]
			provider.Accounts = tc.accounts
			svc.cfg.Providers["claude"] = provider
			useTempUsageCache(t, svc)
			if tc.source != "" {
				snapshot := seededSnapshot("claude")
				snapshot.Source = tc.source
				seedUsage(t, svc, "claude", snapshot)
			}
			info := infoByID(t, mustProviderList(t, svc), "claude")
			if info.Auth != tc.want {
				t.Fatalf("Auth = %q, want %q", info.Auth, tc.want)
			}
			if tc.source != "" {
				store := cache.Store{Dir: svc.usageCacheDir}
				stored := store.OfflineRead("claude", time.Hour)
				if stored.Source != tc.source {
					t.Fatal("display authentication changed usage provenance")
				}
			}
		})
	}
}
