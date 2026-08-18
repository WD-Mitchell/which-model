package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

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

// providerUniverse returns the provider ids from the union of the routes table
// and configured providers, in raw-priority display order. Callers that already
// hold s.mu should use providerUniverseLocked.
func (s *Services) providerUniverse() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.providerUniverseLocked()
}

func (s *Services) providerUniverseLocked() []string {
	seen := make(map[string]struct{}, len(s.cfg.Providers)+len(s.routes.Routes))
	for id := range s.cfg.Providers {
		seen[id] = struct{}{}
	}
	for _, route := range s.routes.Routes {
		seen[route.Provider] = struct{}{}
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
	ids := p.s.providerUniverseLocked()
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
		}

		routesTotal := 0
		currentKeys := make(map[string]struct{})
		for _, route := range p.s.routes.Routes {
			if route.Provider != id {
				continue
			}
			routesTotal++
			currentKeys[routeKey(route)] = struct{}{}
		}
		routesOff := 0
		for _, key := range disabled[id] {
			if _, ok := currentKeys[key]; ok {
				routesOff++
			}
		}
		info.RoutesTotal = routesTotal
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

// Detail returns the provider's models and only the reasoning levels present
// in the routes table.
func (p *ProviderService) Detail(ctx context.Context, id string) (ProviderDetail, error) {
	_ = ctx
	p.s.mu.RLock()
	defer p.s.mu.RUnlock()
	if !p.providerKnownLocked(id) {
		return ProviderDetail{}, fmt.Errorf("%w: providers: unknown provider %q", errNotFound, id)
	}
	disabled := p.s.disabledRouteSetLocked(id)
	type modelData struct {
		name   string
		levels []string
		seen   map[string]struct{}
	}
	models := make(map[string]*modelData)
	for _, route := range p.s.routes.Routes {
		if route.Provider != id {
			continue
		}
		model := models[route.ModelID]
		if model == nil {
			model = &modelData{name: route.Model, seen: make(map[string]struct{})}
			models[route.ModelID] = model
		} else if model.name == "" || (route.Model != "" && route.Model < model.name) {
			model.name = route.Model
		}
		if _, ok := model.seen[route.Reasoning]; !ok {
			model.seen[route.Reasoning] = struct{}{}
			model.levels = append(model.levels, route.Reasoning)
		}
	}
	modelIDs := make([]string, 0, len(models))
	for modelID := range models {
		modelIDs = append(modelIDs, modelID)
	}
	sort.Strings(modelIDs)
	out := ProviderDetail{ID: id, Models: make([]ProviderModel, 0, len(modelIDs))}
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
			levels = append(levels, RouteLevel{Reasoning: reasoning, Enabled: !off, Default: isDefault})
		}
		out.Models = append(out.Models, ProviderModel{ModelID: modelID, ModelName: model.name, Levels: levels})
	}
	return out, nil
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
		for _, candidate := range p.s.routes.Routes {
			if candidate.Provider == provider && candidate.ModelID == modelID && candidate.Reasoning == reasoning {
				route = true
				break
			}
		}
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
				for _, route := range p.s.routes.Routes {
					if route.Provider == provider {
						all = append(all, routeKey(route))
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

