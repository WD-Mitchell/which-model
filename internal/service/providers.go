package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/WD-Mitchell/which-model/internal/catalog/fetch/modelsdev"
	"github.com/WD-Mitchell/which-model/internal/catalog/identity"
	"github.com/WD-Mitchell/which-model/internal/config"
	"github.com/WD-Mitchell/which-model/internal/pick/band"
	"github.com/WD-Mitchell/which-model/internal/routing"
	"github.com/WD-Mitchell/which-model/internal/usage"
	"github.com/WD-Mitchell/which-model/internal/usage/cache"
	"github.com/WD-Mitchell/which-model/internal/usage/toggle"
)

const defaultLimitsTTL = 24 * time.Hour

// ProviderService is the providers facet of Services. Obtain it from
// (*Services).Providers so that all provider operations share the Services
// config, routes table, and lock.
type ProviderService struct{ s *Services }

// Providers returns the providers facet.
func (s *Services) Providers() *ProviderService { return &ProviderService{s: s} }

// providerUniverse returns the provider ids from the union of the routes table,
// configured providers, and every provider this binary ships support for, in
// raw-priority display order. Callers that already hold s.mu should use
// providerUniverseLocked.
func (s *Services) providerUniverse() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.providerUniverseLocked()
}

func (s *Services) providerUniverseLocked() []string {
	catalogue := loadModelsDevCatalogue(s.paths.CacheDir)
	return s.providerUniverseFromCatalogueLocked(catalogue)
}

func (s *Services) providerUniverseFromCatalogueLocked(catalogue []modelsdev.ProviderModel) []string {
	seen := make(map[string]struct{}, len(s.cfg.Providers)+len(s.routes.Routes))
	for id := range s.cfg.Providers {
		seen[id] = struct{}{}
	}
	// Every registered usage provider, so a provider the binary supports is
	// always listable — and therefore enableable — before it has any routes.
	//
	// Without this the universe is empty on a cold install: the route table only
	// exists after `which-model routes refresh` has run against an authenticated
	// CLI, so the Providers page had no rows and no way to get any, which read
	// as "you cannot add providers". Default-deny is unaffected: these appear
	// with Enabled=false until the user turns one on.
	//
	// Empty under `-tags nousage` (nothing registers), which is correct: a
	// binary with no usage providers compiled in should not advertise them.
	for _, id := range usage.IDs() {
		seen[id] = struct{}{}
	}
	for _, id := range discoverBackendProviderIDs(s.cfg.Usage.Backend) {
		seen[id] = struct{}{}
	}
	for _, route := range s.routes.Routes {
		seen[route.Provider] = struct{}{}
	}
	for _, model := range catalogue {
		if model.Provider != "" {
			seen[model.Provider] = struct{}{}
		}
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		pi := s.cfg.Providers[ids[i]].Priority
		pj := s.cfg.Providers[ids[j]].Priority
		if pi != pj {
			return pi < pj
		}
		return ids[i] < ids[j]
	})
	return ids
}

func loadModelsDevCatalogue(cacheDir string) []modelsdev.ProviderModel {
	data, err := os.ReadFile(filepath.Join(cacheDir, "catalog", "modelsdev_providers.json"))
	if err != nil {
		return nil
	}
	var catalogue []modelsdev.ProviderModel
	if err := json.Unmarshal(data, &catalogue); err != nil {
		return nil
	}
	return catalogue
}

// disabledRouteSet returns the disabled route keys for one provider. Stale
// entries are deliberately retained: they are inert until a matching route
// returns to the table.
func (s *Services) disabledRouteSet(id string) map[string]struct{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.disabledRouteSetLocked(id)
}

func (s *Services) disabledRouteSetLocked(id string) map[string]struct{} {
	set := make(map[string]struct{})
	disabled, err := s.cfg.LoadRoutesDisabled()
	if err != nil {
		return set
	}
	for _, route := range disabled[id] {
		set[route] = struct{}{}
	}
	return set
}

func (s *Services) isRouteDisabledLocked(provider, key string) bool {
	_, ok := s.disabledRouteSetLocked(provider)[key]
	return ok
}

// topReasoning returns the canonical highest reasoning level. The route table
// may retain the literal "default" spelling; it is compared as "high".
func topReasoning(levels []string) string {
	top := ""
	topRank := -1
	for _, level := range levels {
		canonical := identity.CollapseReasoning(level)
		rank, known := identity.EffortOrder[canonical]
		if !known {
			if top == "" {
				top = canonical
			}
			continue
		}
		if rank > topRank {
			topRank = rank
			top = canonical
		}
	}
	return top
}

// List returns the provider universe in display order. Usage is read from the
// offline cache only; this method never invokes a provider descriptor or live
// fetch path.
func (p *ProviderService) List(ctx context.Context) ([]ProviderInfo, error) {
	_ = ctx
	p.s.mu.RLock()
	defer p.s.mu.RUnlock()
	return p.listLocked()
}

func (p *ProviderService) listLocked() ([]ProviderInfo, error) {
	catalogue := loadModelsDevCatalogue(p.s.paths.CacheDir)
	ids := p.s.providerUniverseFromCatalogueLocked(catalogue)
	catalogueModelIDs := make(map[string]map[string]struct{})
	for _, model := range catalogue {
		modelIDs := catalogueModelIDs[model.Provider]
		if modelIDs == nil {
			modelIDs = make(map[string]struct{})
			catalogueModelIDs[model.Provider] = modelIDs
		}
		modelIDs[model.ModelID] = struct{}{}
	}
	disabled, err := p.s.cfg.LoadRoutesDisabled()
	if err != nil {
		return nil, err
	}
	usageEnabled, _ := toggle.ResolveUsageEnabled(false, p.s.cfg)
	store := p.usageStoreLocked()
	out := make([]ProviderInfo, 0, len(ids))
	for index, id := range ids {
		provider := p.s.cfg.Providers[id]
		info := ProviderInfo{
			ID:       id,
			Enabled:  provider.Enabled,
			Priority: index + 1,
			Accounts: len(provider.Accounts),
			Builtin:  p.providerBuiltinLocked(id),
		}

		routesTotal := 0
		var modelIDs map[string]struct{}
		currentKeys := make(map[string]struct{})
		for _, route := range p.s.routes.Routes {
			if route.Provider != id {
				continue
			}
			routesTotal++
			if modelIDs == nil {
				modelIDs = make(map[string]struct{})
			}
			modelIDs[route.ModelID] = struct{}{}
			currentKeys[routeKey(route)] = struct{}{}
		}
		modelCount := len(modelIDs)
		for modelID := range catalogueModelIDs[routing.CatalogueSlugFor(id)] {
			if _, routed := modelIDs[modelID]; !routed {
				modelCount++
			}
		}
		routesOff := 0
		for _, key := range disabled[id] {
			if _, ok := currentKeys[key]; ok {
				routesOff++
			}
		}
		info.RoutesTotal = routesTotal
		info.Models = modelCount
		info.RoutesOn = routesTotal - routesOff
		if info.RoutesOn < 0 {
			info.RoutesOn = 0
		}

		ttl := provider.CacheTTL
		if ttl <= 0 {
			ttl = defaultLimitsTTL
		}
		snapshot := store.OfflineRead(id, ttl)
		populateProviderUsage(&info, snapshot)
		switch {
		case !provider.Enabled:
			info.LimitsLine = "not enabled"
		case !usageEnabled:
			clearProviderUsage(&info)
			info.LimitsLine = "usage off"
		case snapshot.Failure != nil:
			clearProviderUsage(&info)
			info.LimitsLine = "no usage data"
		default:
			info.LimitsLine = composeLimitsLine(&info, snapshot)
		}
		out = append(out, info)
	}
	return out, nil
}

func (p *ProviderService) usageStoreLocked() cache.Store {
	dir := p.s.usageCacheDir
	if dir == "" {
		if cacheDir, err := cache.CacheDir(); err == nil {
			dir = cacheDir
		} else {
			dir = filepath.Join(p.s.paths.CacheDir, "usage-cache")
		}
	}
	return cache.Store{Dir: dir}
}

func routeKey(route routing.Route) string {
	return route.ModelID + "@" + route.Reasoning
}

func populateProviderUsage(info *ProviderInfo, snapshot usage.Snapshot) {
	info.Auth = string(snapshot.Source)
	for _, window := range snapshot.Windows {
		var dst **int
		switch window.ID {
		case "session":
			dst = &info.Session
		case "weekly":
			dst = &info.Weekly
		case "monthly":
			dst = &info.Monthly
		default:
			continue
		}
		if *dst != nil {
			continue
		}
		percent, ok := band.WindowPercent(window)
		if !ok {
			continue
		}
		value := int(percent.Round(0).IntPart())
		*dst = &value
	}
}

func clearProviderUsage(info *ProviderInfo) {
	info.Session = nil
	info.Weekly = nil
	info.Monthly = nil
	info.Credits = ""
	info.Resets = ""
	info.Auth = ""
}

func composeLimitsLine(info *ProviderInfo, snapshot usage.Snapshot) string {
	segments := make([]string, 0, 4)
	for _, id := range []string{"session", "weekly", "monthly"} {
		window, ok := firstWindow(snapshot.Windows, id)
		if !ok {
			continue
		}
		percent, computable := band.WindowPercent(window)
		if computable {
			value := int(percent.Round(0).IntPart())
			segments = append(segments, fmt.Sprintf("%s %d%%", id, value))
			continue
		}
		if window.Used != nil && window.Limit != nil && *window.Limit > 0 {
			segments = append(segments, fmt.Sprintf("%s %d of %d", id, int(*window.Used), int(*window.Limit)))
		}
	}

	for _, window := range snapshot.Windows {
		if window.Remaining == nil {
			continue
		}
		if window.Unit != usage.UnitCredits && window.Unit != usage.UnitUSD {
			continue
		}
		info.Credits = fmt.Sprintf("%d credits", int(*window.Remaining))
		segments = append(segments, info.Credits)
		break
	}
	for _, id := range []string{"session", "weekly", "monthly"} {
		window, ok := firstWindow(snapshot.Windows, id)
		if ok && window.ResetHint != "" {
			info.Resets = id + " " + window.ResetHint
			break
		}
	}
	if len(segments) == 0 {
		return "no usage data"
	}
	return strings.Join(segments, " · ")
}

func firstWindow(windows []usage.Window, id string) (usage.Window, bool) {
	for _, window := range windows {
		if window.ID == id {
			return window, true
		}
	}
	return usage.Window{}, false
}

// Detail returns every model currently available from the provider: the
// routes table UNION the provider's full models.dev catalogue. Levels come
// from models.dev EffortLevels (or a single "default" when the catalogue
// declares none), plus any extra levels the routes table already carries.
// A missing scores row no longer hides the combo. The catalogue comes from
// the same cache file Addable reads; an absent or unreadable cache degrades
// to routes-only (never an error, never a fetch).
func (p *ProviderService) Detail(ctx context.Context, id string) (ProviderDetail, error) {
	_ = ctx
	p.s.mu.RLock()
	defer p.s.mu.RUnlock()
	if !p.providerKnownLocked(id) {
		return ProviderDetail{}, fmt.Errorf("%w: providers: unknown provider %q", errNotFound, id)
	}
	accounts := make([]ProviderAccountDTO, 0, len(p.s.cfg.Providers[id].Accounts))
	for _, account := range p.s.cfg.Providers[id].Accounts {
		accounts = append(accounts, ProviderAccountDTO{Name: account.Name, Kind: account.Kind, Ref: account.Ref})
	}
	return ProviderDetail{
		ID:             id,
		Accounts:       accounts,
		OAuthSupported: providerOAuthSupported(id),
		Builtin:        p.providerBuiltinLocked(id),
		Models:         p.providerModelsLocked(id),
	}, nil
}

type modelData struct {
	name   string
	levels []string
}

func addReasoningLevel(model *modelData, reasoning string) {
	collapsed := identity.CollapseReasoning(reasoning)
	for _, existing := range model.levels {
		if identity.CollapseReasoning(existing) == collapsed {
			return
		}
	}
	model.levels = append(model.levels, reasoning)
}

// providerModelsLocked lists every catalogue + routed model for id, with
// every reasoning level the provider currently exposes. Caller holds RLock
// or Lock.
func (p *ProviderService) providerModelsLocked(id string) []ProviderModel {
	return p.providerModelsFromMapLocked(id, p.modelsDevCatalogueLocked(id))
}

func (p *ProviderService) providerModelsFromMapLocked(id string, devCatalogue map[string]modelsDevEntry) []ProviderModel {
	disabled := p.s.disabledRouteSetLocked(id)
	models := make(map[string]*modelData)
	for _, route := range p.s.routes.Routes {
		if route.Provider != id {
			continue
		}
		model := models[route.ModelID]
		if model == nil {
			model = &modelData{name: route.Model}
			models[route.ModelID] = model
		} else if model.name == "" || (route.Model != "" && route.Model < model.name) {
			model.name = route.Model
		}
		addReasoningLevel(model, route.Reasoning)
	}
	for modelID, entry := range devCatalogue {
		model := models[modelID]
		if model == nil {
			model = &modelData{name: entry.name}
			models[modelID] = model
		} else if model.name == "" {
			model.name = entry.name
		}
		if len(entry.levels) > 0 {
			for _, level := range entry.levels {
				addReasoningLevel(model, level)
			}
			continue
		}
		if len(model.levels) == 0 {
			addReasoningLevel(model, "default")
		}
	}
	modelIDs := make([]string, 0, len(models))
	for modelID := range models {
		modelIDs = append(modelIDs, modelID)
	}
	sort.Strings(modelIDs)
	out := make([]ProviderModel, 0, len(modelIDs))
	for _, modelID := range modelIDs {
		model := models[modelID]
		sort.SliceStable(model.levels, func(i, j int) bool {
			return reasoningLess(model.levels[i], model.levels[j])
		})
		top := topReasoning(model.levels)
		levels := make([]RouteLevel, 0, len(model.levels))
		defaultSet := false
		for _, reasoning := range model.levels {
			canonical := identity.CollapseReasoning(reasoning)
			isDefault := !defaultSet && canonical == top
			if isDefault {
				defaultSet = true
			}
			_, off := disabled[modelID+"@"+reasoning]
			if !off {
				_, off = disabled[modelID+"@"+canonical]
			}
			levels = append(levels, RouteLevel{Reasoning: reasoning, Enabled: !off, Default: isDefault})
		}
		out = append(out, ProviderModel{ModelID: modelID, ModelName: model.name, Levels: levels})
	}
	return out
}

func (p *ProviderService) comboKnownLocked(provider, modelID, reasoning string) bool {
	want := identity.CollapseReasoning(reasoning)
	for _, model := range p.providerModelsLocked(provider) {
		if model.ModelID != modelID {
			continue
		}
		for _, level := range model.Levels {
			if identity.CollapseReasoning(level.Reasoning) == want {
				return true
			}
		}
	}
	return false
}

type modelsDevEntry struct {
	name   string
	levels []string
}

// modelsDevCatalogueLocked returns the provider's models.dev catalogue as
// ModelID → name + effort levels. Builtin ids map onto their catalogue slug
// (routing.CatalogueSlugFor); an added provider's id IS its slug. Absent or
// unreadable cache → nil.
func (p *ProviderService) modelsDevCatalogueLocked(id string) map[string]modelsDevEntry {
	catalogue := loadModelsDevCatalogue(p.s.paths.CacheDir)
	slug := routing.CatalogueSlugFor(id)
	out := make(map[string]modelsDevEntry)
	for _, m := range catalogue {
		if m.Provider == slug {
			out[m.ModelID] = modelsDevEntry{name: m.Name, levels: append([]string(nil), m.EffortLevels...)}
		}
	}
	return out
}

func reasoningLess(left, right string) bool {
	leftCanonical := identity.CollapseReasoning(left)
	rightCanonical := identity.CollapseReasoning(right)
	leftRank, leftKnown := identity.EffortOrder[leftCanonical]
	rightRank, rightKnown := identity.EffortOrder[rightCanonical]
	if leftKnown && rightKnown && leftRank != rightRank {
		return leftRank < rightRank
	}
	if leftKnown != rightKnown {
		return leftKnown
	}
	if leftCanonical != rightCanonical {
		return leftCanonical < rightCanonical
	}
	return left < right
}

func (p *ProviderService) providerKnownLocked(id string) bool {
	if _, ok := p.s.cfg.Providers[id]; ok {
		return true
	}
	for _, route := range p.s.routes.Routes {
		if route.Provider == id {
			return true
		}
	}
	for _, registered := range usage.IDs() {
		if registered == id {
			return true
		}
	}
	for _, backendID := range discoverBackendProviderIDs(p.s.cfg.Usage.Backend) {
		if backendID == id {
			return true
		}
	}
	// Keep catalogue-only providers writable as well as listable. Otherwise a
	// provider such as Alibaba appears in List but SetEnabled rejects it.
	for _, model := range loadModelsDevCatalogue(p.s.paths.CacheDir) {
		if model.Provider == id {
			return true
		}
	}
	return false
}

func (p *ProviderService) providerBuiltinLocked(id string) bool {
	if providerBuiltin(id) {
		return true
	}
	for _, backendID := range discoverBackendProviderIDs(p.s.cfg.Usage.Backend) {
		if backendID == id {
			return true
		}
	}
	return false
}

// SetEnabled persists providers.<id>.enabled and emits one providers change.
func (p *ProviderService) SetEnabled(ctx context.Context, id string, enabled bool) error {
	_ = ctx
	p.s.mu.RLock()
	known := p.providerKnownLocked(id)
	p.s.mu.RUnlock()
	if !known {
		return fmt.Errorf("%w: providers: unknown provider %q", errNotFound, id)
	}
	p.s.mu.Lock()
	if !p.providerKnownLocked(id) {
		p.s.mu.Unlock()
		return fmt.Errorf("%w: providers: unknown provider %q", errNotFound, id)
	}
	copyCfg, cleanup, err := cloneConfigForProviders(p.s.cfg)
	if err == nil {
		provider := copyCfg.Providers[id]
		provider.Enabled = enabled
		if copyCfg.Providers == nil {
			copyCfg.Providers = make(map[string]config.ProviderConfig)
		}
		copyCfg.Providers[id] = provider
	}
	if err == nil {
		err = p.persistConfigLocked(copyCfg)
	}
	if cleanup != nil {
		cleanup()
	}
	if err != nil {
		p.s.mu.Unlock()
		return err
	}
	p.s.mu.Unlock()
	p.s.emit(EventConfigChanged, map[string]string{"section": "providers"})
	return nil
}

// Add registers a custom provider id in config so it appears in the provider
// universe (default-deny: Enabled false) at the end of the priority order.
//
// This registers an ID ONLY — the binary has no usage adapter or route source
// for it, so its routes stay empty until the user declares them
// (`which-model routes add`). That is still useful: an enabled custom provider
// with user-declared routes participates in ranking. Built-in ids (already in
// the universe) are conflicts, not re-adds.
func (p *ProviderService) Add(ctx context.Context, id string) error {
	_ = ctx
	id = strings.ToLower(strings.TrimSpace(id))
	if id == "" || !providerIDPattern.MatchString(id) {
		return fmt.Errorf("%w: providers: id must be lowercase letters, digits, '-' or '_'", errValidation)
	}
	p.s.mu.Lock()
	for _, existing := range p.s.providerUniverseLocked() {
		if existing == id {
			p.s.mu.Unlock()
			return fmt.Errorf("%w: providers: %q already exists", errConflict, id)
		}
	}
	copyCfg, cleanup, err := cloneConfigForProviders(p.s.cfg)
	if err == nil {
		if copyCfg.Providers == nil {
			copyCfg.Providers = make(map[string]config.ProviderConfig, 1)
		}
		copyCfg.Providers[id] = config.ProviderConfig{
			Enabled:  false,
			Priority: len(p.s.providerUniverseLocked()) + 1,
		}
		err = p.persistConfigLocked(copyCfg)
	}
	if cleanup != nil {
		cleanup()
	}
	if err != nil {
		p.s.mu.Unlock()
		return err
	}
	p.s.mu.Unlock()
	p.s.emit(EventConfigChanged, map[string]string{"section": "providers"})
	return nil
}

// Addable returns the provider ids that can be added: every models.dev
// provider slug not already in the universe, sorted.
//
// The set is READ FROM THE CACHED CATALOGUE (<cache>/catalog/modelsdev_
// providers.json, written by `catalog refresh` / `routes refresh`), never
// fetched — the settings window must not block on the network. An absent or
// unreadable cache yields an empty list, which the UI shows as "refresh the
// catalogue first" rather than inviting a free-text guess.
//
// Slugs are the only ids worth offering: route production maps a provider id
// onto its models.dev catalogue entries by that exact slug, so an id from
// anywhere else can never acquire routes.
func (p *ProviderService) Addable(ctx context.Context) ([]string, error) {
	_ = ctx
	p.s.mu.RLock()
	existing := make(map[string]struct{})
	for _, id := range p.s.providerUniverseLocked() {
		existing[id] = struct{}{}
	}
	cachePath := filepath.Join(p.s.paths.CacheDir, "catalog", "modelsdev_providers.json")
	p.s.mu.RUnlock()

	data, err := os.ReadFile(cachePath)
	if err != nil {
		return []string{}, nil // no catalogue yet -> nothing to offer
	}
	var catalogue []modelsdev.ProviderModel
	if err := json.Unmarshal(data, &catalogue); err != nil {
		return []string{}, nil
	}

	seen := make(map[string]struct{})
	out := make([]string, 0, 16)
	for _, m := range catalogue {
		if m.Provider == "" {
			continue
		}
		if _, taken := existing[m.Provider]; taken {
			continue
		}
		if _, dup := seen[m.Provider]; dup {
			continue
		}
		seen[m.Provider] = struct{}{}
		out = append(out, m.Provider)
	}
	sort.Strings(out)
	return out, nil
}

// providerBuiltin reports whether id ships a native usage adapter in this
// binary. ProviderService.providerBuiltinLocked additionally includes the
// configured backend's provider roster because those rows are also permanent.
func providerBuiltin(id string) bool {
	for _, registered := range usage.IDs() {
		if registered == id {
			return true
		}
	}
	return false
}

// Delete removes a provider's config entry, its routes from the route table,
// and its disabled-route record. Builtins cannot be deleted.
//
// Routes go too: leaving them would keep the provider in the universe (it is
// unioned from the route table), so the row would reappear and the delete would
// look broken.
func (p *ProviderService) Delete(ctx context.Context, id string) error {
	_ = ctx
	p.s.mu.Lock()
	if p.providerBuiltinLocked(id) {
		p.s.mu.Unlock()
		return fmt.Errorf("%w: providers: %q ships with which-model or its configured usage backend and cannot be deleted; disable it instead", errValidation, id)
	}
	if !p.providerKnownLocked(id) {
		p.s.mu.Unlock()
		return fmt.Errorf("%w: providers: unknown provider %q", errNotFound, id)
	}
	copyCfg, cleanup, err := cloneConfigForProviders(p.s.cfg)
	if err == nil {
		delete(copyCfg.Providers, id)
		err = p.persistConfigLocked(copyCfg)
	}
	if cleanup != nil {
		cleanup()
	}
	if err != nil {
		p.s.mu.Unlock()
		return err
	}
	// Drop its routes so the provider actually leaves the universe.
	kept := make([]routing.Route, 0, len(p.s.routes.Routes))
	for _, route := range p.s.routes.Routes {
		if route.Provider != id {
			kept = append(kept, route)
		}
	}
	if len(kept) != len(p.s.routes.Routes) {
		p.s.routes.Routes = kept
		if err := p.s.saveRoutesLocked(); err != nil {
			p.s.mu.Unlock()
			return err
		}
	}
	p.s.mu.Unlock()
	p.s.emit(EventConfigChanged, map[string]string{"section": "providers"})
	return nil
}

// Duplicate copies a provider's config entry (accounts included) to a fresh id
// and returns it. The copy carries no routes — it is a second ACCOUNT of the
// same service, which is what a duplicate is for here.
func (p *ProviderService) Duplicate(ctx context.Context, id string) (string, error) {
	_ = ctx
	p.s.mu.Lock()
	if !p.providerKnownLocked(id) {
		p.s.mu.Unlock()
		return "", fmt.Errorf("%w: providers: unknown provider %q", errNotFound, id)
	}
	taken := make(map[string]struct{})
	for _, existing := range p.s.providerUniverseLocked() {
		taken[existing] = struct{}{}
	}
	next := ""
	for i := 2; i < 100; i++ {
		candidate := fmt.Sprintf("%s_%d", id, i)
		if _, clash := taken[candidate]; !clash {
			next = candidate
			break
		}
	}
	if next == "" {
		p.s.mu.Unlock()
		return "", fmt.Errorf("%w: providers: no free id for a copy of %q", errConflict, id)
	}

	source := p.s.cfg.Providers[id]
	copyCfg, cleanup, err := cloneConfigForProviders(p.s.cfg)
	if err == nil {
		if copyCfg.Providers == nil {
			copyCfg.Providers = make(map[string]config.ProviderConfig, 1)
		}
		dup := source
		dup.Enabled = false // default-deny, like any new provider
		dup.Priority = len(taken) + 1
		dup.Accounts = append([]config.ProviderAccount(nil), source.Accounts...)
		copyCfg.Providers[next] = dup
		err = p.persistConfigLocked(copyCfg)
	}
	if cleanup != nil {
		cleanup()
	}
	p.s.mu.Unlock()
	if err != nil {
		return "", err
	}
	p.s.emit(EventConfigChanged, map[string]string{"section": "providers"})
	return next, nil
}

// SetAccounts replaces a provider's account list wholesale — one atomic write
// covers add, rename, re-kind and remove, so the UI never has to sequence
// several calls and half-apply on failure.
func (p *ProviderService) SetAccounts(ctx context.Context, id string, accounts []ProviderAccountDTO) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(accounts))
	for _, account := range accounts {
		name := strings.TrimSpace(account.Name)
		if name == "" {
			return fmt.Errorf("%w: providers: an account needs a name", errValidation)
		}
		switch account.Kind {
		case AccountKindOAuth, AccountKindCookie, AccountKindToken:
		default:
			return fmt.Errorf("%w: providers: account %q has unknown kind %q (oauth, cookie or token)", errValidation, name, account.Kind)
		}
		if _, dup := seen[name]; dup {
			return fmt.Errorf("%w: providers: duplicate account name %q", errConflict, name)
		}
		seen[name] = struct{}{}
	}

	p.s.mu.Lock()
	if !p.providerKnownLocked(id) {
		p.s.mu.Unlock()
		return fmt.Errorf("%w: providers: unknown provider %q", errNotFound, id)
	}
	previousAccounts := p.s.cfg.Providers[id].Accounts
	copyCfg, cleanup, err := cloneConfigForProviders(p.s.cfg)
	if err == nil {
		if copyCfg.Providers == nil {
			copyCfg.Providers = make(map[string]config.ProviderConfig, 1)
		}
		provider := copyCfg.Providers[id]
		provider.Accounts = make([]config.ProviderAccount, 0, len(accounts))
		for _, account := range accounts {
			provider.Accounts = append(provider.Accounts, config.ProviderAccount{
				Name: strings.TrimSpace(account.Name),
				Kind: account.Kind,
				Ref:  strings.TrimSpace(account.Ref),
			})
		}
		copyCfg.Providers[id] = provider
	}

	var rollbackCredential func() error
	if err == nil &&
		hasProviderAccountRef(previousAccounts, managedOAuthRef) &&
		!hasProviderAccountRef(copyCfg.Providers[id].Accounts, managedOAuthRef) {
		store, storeErr := p.s.managedStoreLocked()
		if storeErr != nil {
			err = storeErr
		} else {
			rollbackCredential, err = removeManagedCredential(ctx, store, id)
		}
	}
	if err == nil {
		err = p.persistConfigLocked(copyCfg)
	}
	if err != nil && rollbackCredential != nil {
		if rollbackErr := rollbackCredential(); rollbackErr != nil {
			err = fmt.Errorf("%w; managed credential restoration failed: %w", err, rollbackErr)
		}
	}
	if cleanup != nil {
		cleanup()
	}
	p.s.mu.Unlock()
	if err != nil {
		return err
	}
	p.s.emit(EventConfigChanged, map[string]string{"section": "providers"})
	return nil
}

func hasProviderAccountRef(accounts []config.ProviderAccount, ref string) bool {
	for _, account := range accounts {
		if strings.TrimSpace(account.Ref) == ref {
			return true
		}
	}
	return false
}

// providerIDPattern bounds custom provider ids to config-key-safe slugs.
var providerIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// Reorder rewrites every provider priority to its 1-based display position.
func (p *ProviderService) Reorder(ctx context.Context, orderedIDs []string) error {
	_ = ctx
	p.s.mu.RLock()
	universe := p.s.providerUniverseLocked()
	p.s.mu.RUnlock()
	if err := validateProviderReorder(orderedIDs, universe); err != nil {
		return err
	}
	p.s.mu.Lock()
	// The routes table can only change under this lock; validate again before
	// constructing the copy so a concurrent catalog reload cannot reorder a
	// different universe.
	universe = p.s.providerUniverseLocked()
	if err := validateProviderReorder(orderedIDs, universe); err != nil {
		p.s.mu.Unlock()
		return err
	}
	copyCfg, cleanup, err := cloneConfigForProviders(p.s.cfg)
	if err == nil {
		if copyCfg.Providers == nil {
			copyCfg.Providers = make(map[string]config.ProviderConfig, len(orderedIDs))
		}
		for index, id := range orderedIDs {
			provider := copyCfg.Providers[id]
			provider.Priority = index + 1
			copyCfg.Providers[id] = provider
		}
		err = p.persistConfigLocked(copyCfg)
	}
	if cleanup != nil {
		cleanup()
	}
	if err != nil {
		p.s.mu.Unlock()
		return err
	}
	p.s.mu.Unlock()
	p.s.emit(EventConfigChanged, map[string]string{"section": "providers"})
	return nil
}

func validateProviderReorder(orderedIDs, universe []string) error {
	seen := make(map[string]struct{}, len(orderedIDs))
	for _, id := range orderedIDs {
		if _, ok := seen[id]; ok {
			return fmt.Errorf("%w: providers: reorder list contains duplicate id %q", errValidation, id)
		}
		seen[id] = struct{}{}
	}
	known := make(map[string]struct{}, len(universe))
	for _, id := range universe {
		known[id] = struct{}{}
	}
	for _, id := range orderedIDs {
		if _, ok := known[id]; !ok {
			return fmt.Errorf("%w: providers: unknown provider %q", errValidation, id)
		}
	}
	if len(orderedIDs) != len(universe) {
		return fmt.Errorf("%w: providers: reorder list must contain every provider exactly once (got %d, want %d)", errValidation, len(orderedIDs), len(universe))
	}
	return nil
}

// SetRouteEnabled toggles one route's disabled-list entry.
func (p *ProviderService) SetRouteEnabled(ctx context.Context, provider, modelID, reasoning string, enabled bool) error {
	_ = ctx
	p.s.mu.RLock()
	known, route := p.providerKnownLocked(provider), false
	if known {
		route = p.comboKnownLocked(provider, modelID, reasoning)
	}
	p.s.mu.RUnlock()
	if !known {
		return fmt.Errorf("%w: providers: unknown provider %q", errNotFound, provider)
	}
	if !route {
		return fmt.Errorf("%w: providers: no route %s/%s@%s", errNotFound, provider, modelID, reasoning)
	}

	p.s.mu.Lock()
	if !p.providerKnownLocked(provider) {
		p.s.mu.Unlock()
		return fmt.Errorf("%w: providers: unknown provider %q", errNotFound, provider)
	}
	copyCfg, cleanup, err := cloneConfigForProviders(p.s.cfg)
	if err == nil {
		disabled, loadErr := copyCfg.LoadRoutesDisabled()
		if loadErr != nil {
			err = loadErr
		} else {
			if disabled == nil {
				disabled = config.RoutesDisabledTOML{}
			}
			key := modelID + "@" + reasoning
			entries := append([]string(nil), disabled[provider]...)
			if enabled {
				entries = removeString(entries, key)
			} else {
				entries = append(entries, key)
			}
			disabled[provider] = normalizeStrings(entries)
			normalizeDisabledMap(disabled)
			err = copyCfg.SetRoutesDisabled(disabled)
		}
	}
	if err == nil {
		err = p.persistConfigLocked(copyCfg)
	}
	if cleanup != nil {
		cleanup()
	}
	if err != nil {
		p.s.mu.Unlock()
		return err
	}
	p.s.mu.Unlock()
	p.s.emit(EventConfigChanged, map[string]string{"section": "routes"})
	return nil
}

// SetAllRoutes enables or disables every current route for a provider.
func (p *ProviderService) SetAllRoutes(ctx context.Context, provider string, enabled bool) error {
	_ = ctx
	p.s.mu.RLock()
	known := p.providerKnownLocked(provider)
	p.s.mu.RUnlock()
	if !known {
		return fmt.Errorf("%w: providers: unknown provider %q", errNotFound, provider)
	}
	p.s.mu.Lock()
	if !p.providerKnownLocked(provider) {
		p.s.mu.Unlock()
		return fmt.Errorf("%w: providers: unknown provider %q", errNotFound, provider)
	}
	copyCfg, cleanup, err := cloneConfigForProviders(p.s.cfg)
	if err == nil {
		disabled, loadErr := copyCfg.LoadRoutesDisabled()
		if loadErr != nil {
			err = loadErr
		} else {
			if disabled == nil {
				disabled = config.RoutesDisabledTOML{}
			}
			if enabled {
				delete(disabled, provider)
			} else {
				all := make([]string, 0)
				for _, model := range p.providerModelsLocked(provider) {
					for _, level := range model.Levels {
						all = append(all, model.ModelID+"@"+level.Reasoning)
					}
				}
				disabled[provider] = normalizeStrings(all)
			}
			normalizeDisabledMap(disabled)
			err = copyCfg.SetRoutesDisabled(disabled)
		}
	}
	if err == nil {
		err = p.persistConfigLocked(copyCfg)
	}
	if cleanup != nil {
		cleanup()
	}
	if err != nil {
		p.s.mu.Unlock()
		return err
	}
	p.s.mu.Unlock()
	p.s.emit(EventConfigChanged, map[string]string{"section": "routes"})
	return nil
}

func normalizeDisabledMap(disabled config.RoutesDisabledTOML) {
	for provider, entries := range disabled {
		normalized := normalizeStrings(entries)
		if len(normalized) == 0 {
			delete(disabled, provider)
		} else {
			disabled[provider] = normalized
		}
	}
}

func normalizeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	sort.Strings(values)
	out := values[:0]
	for _, value := range values {
		if len(out) == 0 || out[len(out)-1] != value {
			out = append(out, value)
		}
	}
	return out
}

func removeString(values []string, target string) []string {
	out := values[:0]
	for _, value := range values {
		if value != target {
			out = append(out, value)
		}
	}
	return out
}

func (p *ProviderService) persistConfigLocked(next *config.Config) error {
	data, err := next.MarshalTOML()
	if err != nil {
		return err
	}
	if err := config.AtomicWriteFile(p.s.paths.UserConfigFile, data); err != nil {
		return err
	}
	p.s.cfg = next
	return nil
}

func cloneConfigForProviders(src *config.Config) (*config.Config, func(), error) {
	data, err := src.MarshalTOML()
	if err != nil {
		return nil, nil, err
	}
	file, err := os.CreateTemp("", "which-model-providers-")
	if err != nil {
		return nil, nil, err
	}
	name := file.Name()
	cleanup := func() { _ = os.Remove(name) }
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		cleanup()
		return nil, nil, err
	}
	if err := file.Close(); err != nil {
		cleanup()
		return nil, nil, err
	}
	copyCfg, err := config.LoadFile(name)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	return copyCfg, cleanup, nil
}
