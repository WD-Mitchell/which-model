// S04 — service binding wrappers. One thin struct per EngineHost group
// (D00 CONTRACTS §5) so Wails derives binding module names that map 1:1 onto
// host groups. Each method is a single delegation to the service layer — no
// business logic here. Bound errors are *service.ErrorDTO values (which
// implement error and JSON-serialise to {code, message}), so every rejection
// is ErrorDTO-shaped.
package main

import (
	"context"

	"github.com/WD-Mitchell/which-model/internal/service"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// ctx is the binding context for service calls. Long-running service methods
// (usage fetch, derive) take a context for cancellation; Wails-bound methods
// have no per-call context, so background() is used (the browser host drives
// these synchronously).
var ctx = context.Background()

// ProfilesAPI — EngineHost.profiles.
type ProfilesAPI struct{ svc *service.Services }

func (a *ProfilesAPI) List() ([]service.ProfileSummary, error) {
	return a.svc.Profiles().List(ctx)
}
func (a *ProfilesAPI) Get(slug string) (service.ProfileDetail, error) {
	return a.svc.Profiles().Get(ctx, slug)
}
func (a *ProfilesAPI) Save(p service.ProfileDetail) error {
	return a.svc.Profiles().Save(ctx, p)
}
func (a *ProfilesAPI) Duplicate(slug string) (service.ProfileDetail, error) {
	return a.svc.Profiles().Duplicate(ctx, slug)
}
func (a *ProfilesAPI) Delete(slug string) error {
	return a.svc.Profiles().Delete(ctx, slug)
}
func (a *ProfilesAPI) ComplexityScale() ([]string, error) {
	return a.svc.Profiles().ComplexityScale(), nil
}

// PickAPI — EngineHost.pick.
type PickAPI struct{ svc *service.Services }

func (a *PickAPI) Rank(req service.RankRequest) (service.RankResponse, error) {
	return a.svc.Rank(ctx, req)
}
func (a *PickAPI) RecordPick(profileSlug, routeKey string) error {
	return a.svc.RecordPick(ctx, profileSlug, routeKey)
}
func (a *PickAPI) CatalogLine() (service.CatalogSummary, error) {
	return a.svc.CatalogLine(ctx)
}

// CatalogAPI — EngineHost.catalog.
type CatalogAPI struct{ svc *service.Services }

func (a *CatalogAPI) Benchmarks() ([]string, error) {
	return a.svc.Catalog().Benchmarks(ctx)
}
func (a *CatalogAPI) BenchmarkDetail(name string) (service.BenchmarkDetail, error) {
	return a.svc.Catalog().BenchmarkDetail(ctx, name)
}
func (a *CatalogAPI) Groups() ([]service.GroupSummary, error) {
	return a.svc.Catalog().Groups(ctx)
}
func (a *CatalogAPI) GroupDetail(slug string) (service.GroupDetail, error) {
	return a.svc.Catalog().GroupDetail(ctx, slug)
}
func (a *CatalogAPI) SaveGroup(slug string, benchmarks []string, renameTo string) error {
	return a.svc.Catalog().SaveGroup(ctx, slug, benchmarks, renameTo)
}
func (a *CatalogAPI) DuplicateGroup(slug string) (service.GroupDetail, error) {
	return a.svc.Catalog().DuplicateGroup(ctx, slug)
}
func (a *CatalogAPI) DeleteGroup(slug string) error {
	return a.svc.Catalog().DeleteGroup(ctx, slug)
}

// ProvidersAPI — EngineHost.providers.
type ProvidersAPI struct{ svc *service.Services }

func (a *ProvidersAPI) List() ([]service.ProviderInfo, error) {
	return a.svc.Providers().List(ctx)
}
func (a *ProvidersAPI) Add(id string) error {
	return a.svc.Providers().Add(ctx, id)
}
func (a *ProvidersAPI) Addable() ([]string, error) {
	return a.svc.Providers().Addable(ctx)
}
func (a *ProvidersAPI) Delete(id string) error {
	return a.svc.Providers().Delete(ctx, id)
}
func (a *ProvidersAPI) Duplicate(id string) (string, error) {
	return a.svc.Providers().Duplicate(ctx, id)
}
func (a *ProvidersAPI) SetAccounts(id string, accounts []service.ProviderAccountDTO) error {
	return a.svc.Providers().SetAccounts(ctx, id, accounts)
}
func (a *ProvidersAPI) SetEnabled(id string, on bool) error {
	return a.svc.Providers().SetEnabled(ctx, id, on)
}
func (a *ProvidersAPI) Reorder(orderedIds []string) error {
	return a.svc.Providers().Reorder(ctx, orderedIds)
}
func (a *ProvidersAPI) Detail(id string) (service.ProviderDetail, error) {
	return a.svc.Providers().Detail(ctx, id)
}
func (a *ProvidersAPI) SetRouteEnabled(id, modelId, reasoning string, on bool) error {
	return a.svc.Providers().SetRouteEnabled(ctx, id, modelId, reasoning, on)
}
func (a *ProvidersAPI) SetAllRoutes(id string, on bool) error {
	return a.svc.Providers().SetAllRoutes(ctx, id, on)
}
func (a *ProvidersAPI) RefreshRoutes() error {
	return a.svc.Providers().RefreshRoutes(ctx)
}

// HarnessesAPI — EngineHost.harnesses.
type HarnessesAPI struct{ svc *service.Services }

func (a *HarnessesAPI) List() ([]service.HarnessInfo, error) {
	return a.svc.Harnesses().List(ctx)
}
func (a *HarnessesAPI) Save(h service.HarnessInfo) error {
	return a.svc.Harnesses().Save(ctx, h)
}
func (a *HarnessesAPI) Delete(slug string) error {
	return a.svc.Harnesses().Delete(ctx, slug)
}
func (a *HarnessesAPI) SetProvider(slug, provider string, on bool) error {
	return a.svc.Harnesses().SetProvider(ctx, slug, provider, on)
}
func (a *HarnessesAPI) SetAllProviders(slug string, on bool) error {
	return a.svc.Harnesses().SetAllProviders(ctx, slug, on)
}
func (a *HarnessesAPI) Launch(slug, routeKey, profileSlug string) (service.LaunchResult, error) {
	return a.svc.Harnesses().Launch(ctx, slug, routeKey, profileSlug)
}

// UsageAPI — EngineHost.usage.
type UsageAPI struct{ svc *service.Services }

func (a *UsageAPI) Snapshots(force bool) ([]service.UsageDTO, error) {
	return a.svc.Snapshots(ctx, force)
}
func (a *UsageAPI) SetMode(mode string) error {
	return a.svc.SetMode(ctx, mode)
}
func (a *UsageAPI) SetBackend(backend string) error {
	return a.svc.SetBackend(ctx, backend)
}
func (a *UsageAPI) Mode() (service.UsageMode, error) {
	return a.svc.Mode(ctx)
}

// FavouritesAPI — EngineHost.favourites.
type FavouritesAPI struct{ svc *service.Services }

func (a *FavouritesAPI) List() ([]service.Favourite, error) {
	return a.svc.Favourites().List(ctx)
}
func (a *FavouritesAPI) Pin(routeKey string) error {
	return a.svc.Favourites().Pin(ctx, routeKey)
}
func (a *FavouritesAPI) Unpin(routeKey string) error {
	return a.svc.Favourites().Unpin(ctx, routeKey)
}

// SettingsAPI — EngineHost.settings.
type SettingsAPI struct{ svc *service.Services }

func (a *SettingsAPI) Get() (service.GUISettings, error) {
	return a.svc.Settings().Get(ctx)
}
func (a *SettingsAPI) Set(s service.GUISettings) error {
	return a.svc.Settings().Set(ctx, s)
}
func (a *SettingsAPI) ShellSnippets() (service.ShellSnippets, error) {
	return a.svc.Settings().ShellSnippets(ctx)
}

// SignInAPI — EngineHost.signin. The interactive device-flow sign-in
// (copilot OAuth): Start returns the verification URL + user code, Confirm
// polls until approved and saves the credential, Cancel abandons.
type SignInAPI struct{ svc *service.Services }

func (a *SignInAPI) Start(provider string) (service.SignInStart, error) {
	return a.svc.SignIn().Start(ctx, provider)
}
func (a *SignInAPI) Confirm(provider string) error {
	return a.svc.SignIn().Confirm(ctx, provider)
}
func (a *SignInAPI) Cancel(provider string) error {
	return a.svc.SignIn().Cancel(provider)
}

// registerServices builds the application.Service entries for every engine
// facet. The host WindowService is registered separately (via
// app.RegisterService) once the popover window exists (S04 SPEC §2.2).
func registerServices(svc *service.Services) []application.Service {
	return []application.Service{
		application.NewService(&ProfilesAPI{svc: svc}),
		application.NewService(&PickAPI{svc: svc}),
		application.NewService(&CatalogAPI{svc: svc}),
		application.NewService(&ProvidersAPI{svc: svc}),
		application.NewService(&HarnessesAPI{svc: svc}),
		application.NewService(&UsageAPI{svc: svc}),
		application.NewService(&SignInAPI{svc: svc}),
		application.NewService(&FavouritesAPI{svc: svc}),
		application.NewService(&SettingsAPI{svc: svc}),
	}
}

// registerWindowService registers the host WindowService once the app and
// popover window both exist (S04 SPEC §2.2).
func registerWindowService(app *application.App, popover *application.WebviewWindow) {
	if app == nil {
		return
	}
	app.RegisterService(application.NewService(newWindowService(app, popover)))
}
