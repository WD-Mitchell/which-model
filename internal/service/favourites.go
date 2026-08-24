// Package service is the desktop app's Wails-free programmatic surface over
// the which-model engine (B00 SPEC). This file (B09-favourites) holds the
// pinned-routes facet: B00 SPEC §6.3 availability semantics (the same set
// B04/B06 use) and whole-file atomic config persistence per B00 SPEC §2.2.
package service

import (
	"context"
	"fmt"

	"github.com/WD-Mitchell/which-model/internal/config"
	"github.com/WD-Mitchell/which-model/internal/routing"
)

// FavouriteService is the favourites facet of Services; the host registers
// it as a Wails service. Zero value unusable; obtain via Favourites().
type FavouriteService struct{ s *Services }

// Favourites returns the favourites facet (shares the Services lock/config).
func (s *Services) Favourites() *FavouriteService {
	return &FavouriteService{s: s}
}

// pinnedRoutes reads [favourites].pins raw (bypassing validateFavourites so a
// hand-edited corrupt pin is surfaced by List, SPEC §2.7). Must be called
// with the Services lock held (R or W).
func (s *Services) pinnedRoutes() ([]string, error) {
	var fav config.FavouritesTOML
	if err := s.cfg.UnmarshalKey("favourites", &fav); err != nil {
		return nil, err
	}
	return fav.Pins, nil
}

// routesDisabled reads [routes.disabled] raw for the availability check.
// Must be called with the Services lock held.
func (s *Services) routesDisabled() config.RoutesDisabledTOML {
	var disabled config.RoutesDisabledTOML
	if err := s.cfg.UnmarshalKey("routes.disabled", &disabled); err != nil {
		return config.RoutesDisabledTOML{}
	}
	if disabled == nil {
		disabled = config.RoutesDisabledTOML{}
	}
	return disabled
}

// List returns [favourites].pins in stored order, each annotated per SPEC
// §2.2–2.4 (ModelName from the routes table, RouteLabel "provider ·
// reasoning" or "no provider · reasoning", InRange from the availability
// set). Ill-formed stored pins are surfaced, not dropped (SPEC §2.7).
func (f *FavouriteService) List(ctx context.Context) ([]Favourite, error) {
	s := f.s
	s.mu.RLock()
	defer s.mu.RUnlock()

	pins, err := s.pinnedRoutes()
	if err != nil {
		return nil, err
	}
	providers := s.cfg.Providers
	disabled := s.routesDisabled()
	routes := s.routes.Routes

	out := make([]Favourite, 0, len(pins))
	for _, key := range pins {
		out = append(out, resolveFavourite(key, routes, providers, disabled))
	}
	return out, nil
}

// Pin validates routeKey grammar and appends it to [favourites].pins.
// Already pinned -> nil, no write, no event. Otherwise persists atomically
// and emits config:changed {"section":"favourites"}. Bad grammar ->
// errValidation (§4). Out-of-range keys are accepted (SPEC §2.5).
func (f *FavouriteService) Pin(ctx context.Context, routeKey string) error {
	if _, _, _, err := ParseRouteKey(routeKey); err != nil {
		return fmt.Errorf("favourites: invalid route key %q: %w", routeKey, err)
	}
	changed, err := f.persistPins(func(pins []string) ([]string, bool) {
		for _, p := range pins {
			if p == routeKey {
				return pins, false // idempotent: already pinned
			}
		}
		return append(append([]string(nil), pins...), routeKey), true
	})
	if err != nil {
		return err
	}
	if changed {
		f.s.emit(EventConfigChanged, map[string]string{"section": "favourites"})
	}
	return nil
}

// Unpin validates routeKey grammar and removes it from [favourites].pins.
// Not pinned -> nil, no write, no event. Otherwise persists atomically and
// emits config:changed {"section":"favourites"}.
func (f *FavouriteService) Unpin(ctx context.Context, routeKey string) error {
	if _, _, _, err := ParseRouteKey(routeKey); err != nil {
		return fmt.Errorf("favourites: invalid route key %q: %w", routeKey, err)
	}
	changed, err := f.persistPins(func(pins []string) ([]string, bool) {
		for i, p := range pins {
			if p == routeKey {
				out := make([]string, 0, len(pins)-1)
				out = append(out, pins[:i]...)
				out = append(out, pins[i+1:]...)
				return out, true
			}
		}
		return pins, false // idempotent: not pinned
	})
	if err != nil {
		return err
	}
	if changed {
		f.s.emit(EventConfigChanged, map[string]string{"section": "favourites"})
	}
	return nil
}

// persistPins computes the next [favourites].pins via mutate (which reports
// whether the list actually differs), then persists atomically under the
// write lock per B00 SPEC §2.2: mutate -> MarshalTOML -> AtomicWriteFile.
// On any persist failure the in-memory [favourites] section is rolled back so
// "a failed write leaves in-memory state untouched" (B00 SPEC §2.2); the
// temp+rename writer already guarantees the on-disk file is untouched. A
// no-op (mutate reports unchanged) returns changed=false: no write, no event.
func (f *FavouriteService) persistPins(mutate func([]string) ([]string, bool)) (bool, error) {
	s := f.s
	s.mu.Lock()
	defer s.mu.Unlock()

	prev, err := s.pinnedRoutes()
	if err != nil {
		return false, err
	}
	next, changed := mutate(prev)
	if !changed {
		return false, nil
	}

	prevFav := config.FavouritesTOML{Pins: prev}
	if err := s.cfg.SetFavourites(config.FavouritesTOML{Pins: next}); err != nil {
		return false, err
	}
	data, err := s.cfg.MarshalTOML()
	if err != nil {
		s.cfg.SetFavourites(prevFav) // rollback in-memory
		return false, err
	}
	if err := config.AtomicWriteFile(s.paths.UserConfigFile, data); err != nil {
		s.cfg.SetFavourites(prevFav) // rollback in-memory
		return false, err
	}
	return true, nil
}

// resolveFavourite annotates one stored pin. Ill-formed pins are surfaced
// verbatim (SPEC §2.7) with RouteLabel "no provider · default" and InRange
// false. Well-formed pins are annotated per SPEC §2.2–2.4.
func resolveFavourite(key string, routes []routing.Route, providers map[string]config.ProviderConfig, disabled config.RoutesDisabledTOML) Favourite {
	provider, modelID, reasoning, err := ParseRouteKey(key)
	if err != nil {
		return Favourite{
			RouteKey:   key,
			ModelName:  key,
			RouteLabel: "no provider \u00b7 default",
			InRange:    false,
		}
	}

	inRange := routeInTable(routes, provider, modelID, reasoning) &&
		providers[provider].Enabled &&
		!disabledRoute(disabled, provider, modelID, reasoning)

	labelProvider := provider
	if !inRange {
		labelProvider = "no provider"
	}

	return Favourite{
		RouteKey:   key,
		ModelName:  resolveModelName(routes, provider, modelID, reasoning),
		RouteLabel: labelProvider + " \u00b7 " + reasoning,
		InRange:    inRange,
	}
}

// routeInTable reports whether the exact (provider, model_id, reasoning)
// triple exists in the routes table (B00 CONTRACTS §6.3).
func routeInTable(routes []routing.Route, provider, modelID, reasoning string) bool {
	for _, r := range routes {
		if r.Provider == provider && r.ModelID == modelID && r.Reasoning == reasoning {
			return true
		}
	}
	return false
}

// disabledRoute reports whether "model_id@reasoning" is listed under
// [routes.disabled].<provider> (B00 CONTRACTS §6.3).
func disabledRoute(disabled config.RoutesDisabledTOML, provider, modelID, reasoning string) bool {
	target := modelID + "@" + reasoning
	for _, route := range disabled[provider] {
		if route == target {
			return true
		}
	}
	return false
}

// resolveModelName returns the display name for a pin per SPEC §2.2: the exact
// route's Model; else the first route sharing the pin's model_id ordered by
// (provider asc, then table/stored order); else the model_id verbatim.
func resolveModelName(routes []routing.Route, provider, modelID, reasoning string) string {
	for _, r := range routes {
		if r.Provider == provider && r.ModelID == modelID && r.Reasoning == reasoning {
			return r.Model
		}
	}
	var best *routing.Route
	for i := range routes {
		r := &routes[i]
		if r.ModelID != modelID {
			continue
		}
		if best == nil || r.Provider < best.Provider {
			cp := *r
			best = &cp
		}
	}
	if best != nil {
		return best.Model
	}
	return modelID
}