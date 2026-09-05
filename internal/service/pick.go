// Package service is the desktop app's Wails-free programmatic surface over
// the which-model engine (B00 SPEC). This file (B04-pick) implements the
// popover's ranking surface: Rank, RecordPick, and CatalogLine (B04 SPEC §1).
// It resolves a profile (saved or ephemeral overrides), builds the live
// availability set from the routes table and provider config, runs the
// engine's pick.Rank, and maps the result to the D00 RankResponse with a
// route key per candidate (SPEC §2).
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/WD-Mitchell/which-model/internal/catalog"
	"github.com/WD-Mitchell/which-model/internal/pick"
	"github.com/WD-Mitchell/which-model/internal/routing"
)

// tier1KeySet is the closed tier-1 axis vocabulary for ephemeral overrides
// (B04 SPEC §2.1).
var tier1KeySet = map[string]bool{"intelligence": true, "cost": true, "speed": true}

// Rank resolves the effective profile (SPEC §2.1), builds the availability
// set (SPEC §2.4), runs pick.Rank, and maps the top Holds candidates.
// Overrides non-nil => ephemeral: no persistence, no history, no event
// (D00 §2.2). Empty availability or *pick.NoCandidatesError =>
// RankResponse{Total: 0}, nil error (SPEC §2.5). Read-only; emits nothing.
func (s *Services) Rank(ctx context.Context, req RankRequest) (RankResponse, error) {
	_ = ctx
	s.mu.RLock()
	defer s.mu.RUnlock()

	profile, err := s.resolveProfile(req)
	if err != nil {
		return RankResponse{}, err
	}

	available := s.availableIdentities()
	if len(available) == 0 {
		return RankResponse{Candidates: []RankedModel{}, Total: 0}, nil
	}

	holds, err := s.effectiveHolds(req.Holds)
	if err != nil {
		return RankResponse{}, err
	}

	categories, err := s.profileCategories()
	if err != nil {
		return RankResponse{}, err
	}
	gui, err := s.cfg.LoadGUI()
	if err != nil {
		return RankResponse{}, err
	}
	result, err := pick.RankWithOptions(s.scores, profile, available, categories, pick.RankOptions{AllowIncomplete: gui.AllowIncompleteRecommendations})
	var noCand *pick.NoCandidatesError
	if errors.As(err, &noCand) {
		return RankResponse{Candidates: []RankedModel{}, Total: 0}, nil
	}
	var rankErr *pick.RankingError
	if errors.As(err, &rankErr) {
		return RankResponse{}, fmt.Errorf("%w: %v", errValidation, err)
	}
	if err != nil {
		return RankResponse{}, fmt.Errorf("pick: rank: %w", err)
	}

	all := make([]pick.ModelScore, 0, result.CandidateCount)
	all = append(all, result.Recommendation)
	all = append(all, result.Alternatives...)

	candidates := make([]RankedModel, 0, holds)
	for i, ms := range all {
		if i >= holds {
			break
		}
		cand := RankedModel{
			Rank:      i + 1,
			ModelName: ms.Model,
			Reasoning: ms.Reasoning,
			Score:     round2(ms.Total),
		}
		if row, ok := s.scoreRow(ms.Model, ms.Reasoning); ok {
			cand.Intelligence = tier1ScorePtr(row, pick.AxisIntelligence)
			cand.Cost = tier1ScorePtr(row, pick.AxisCost)
			cand.Speed = tier1ScorePtr(row, pick.AxisSpeed)
		}
		if route, ok := s.resolveRoute(ms.Model, ms.Reasoning); ok {
			cand.Provider = route.Provider
			cand.ModelID = route.ModelID
			cand.RouteKey = FormatRouteKey(route.Provider, route.ModelID, route.Reasoning)
		}
		candidates = append(candidates, cand)
	}
	return RankResponse{Candidates: candidates, Total: result.CandidateCount}, nil
}

// resolveProfile returns the effective engine profile: req.Overrides, if
// non-nil, is validated as an ephemeral profile (SPEC §2.1-2.2) and used for
// the duration of the call only; otherwise the profile is loaded by
// req.ProfileSlug (builtin from pick.Profiles, custom from [profiles.*]
// per B03's merge rule), with unknown slug -> errNotFound.
func (s *Services) resolveProfile(req RankRequest) (catalog.Profile, error) {
	if req.Overrides != nil {
		if err := s.validateOverrides(*req.Overrides); err != nil {
			return catalog.Profile{}, err
		}
		return catalogProfile(*req.Overrides)
	}
	return s.profileBySlug(req.ProfileSlug)
}

// profileBySlug resolves a saved profile: builtins first (pick.Profiles,
// which already carry percentage shares), then customs from [profiles.*]
// converted to engine profiles. Unknown slug -> errNotFound.
func (s *Services) profileBySlug(slug string) (catalog.Profile, error) {
	if bp, ok := pick.Profiles[slug]; ok {
		return bp, nil
	}
	customs, err := s.cfg.LoadProfiles(pick.CategoryNames)
	if err != nil {
		return catalog.Profile{}, err
	}
	cp, ok := customs[slug]
	if !ok {
		return catalog.Profile{}, fmt.Errorf("%w: profile %q not found", errNotFound, slug)
	}
	return catalogProfile(ProfileDetail{
		Slug:         slug,
		Name:         slug,
		CoreShare:    cp.CoreShare,
		Tier1Weights: cloneWeights(cp.Tier1),
		Tier2Weights: cloneWeights(cp.Tier2),
	})
}

// catalogProfile converts a DTO ProfileDetail to the engine catalog.Profile.
// B02's engineProfile returns normalized (0..1) shares; B04 SPEC §2.3
// requires percentage shares (Tier1Share = CoreShare, Tier2Share =
// 100-CoreShare) because pick.Rank combines as tier·share÷100.
func catalogProfile(d ProfileDetail) (catalog.Profile, error) {
	ep, err := engineProfile(d)
	if err != nil {
		return catalog.Profile{}, err
	}
	ep.Tier1Share = ep.Tier1Share.Mul(decimalHundred)
	ep.Tier2Share = ep.Tier2Share.Mul(decimalHundred)
	return ep, nil
}

// validateOverrides enforces the SPEC §2.1 ephemeral-profile checks in order:
// CoreShare in 10..90 step 5; tier-1 keys subset of {intelligence,cost,speed}
// with at least one weight >= 1; every weight in 0..5; tier-2 keys subset of
// pick.CategoryNames ∪ custom group slugs. Slug/Name are ignored (ephemeral
// profiles have no identity). Failures -> errValidation.
func (s *Services) validateOverrides(d ProfileDetail) error {
	if d.CoreShare < 10 || d.CoreShare > 90 || d.CoreShare%5 != 0 {
		return fmt.Errorf("%w: core_share %d must be between 10 and 90 in steps of 5", errValidation, d.CoreShare)
	}
	atLeastOne := false
	for k := range d.Tier1Weights {
		if !tier1KeySet[k] {
			return fmt.Errorf("%w: tier1 key %q must be one of intelligence, cost, speed", errValidation, k)
		}
		if d.Tier1Weights[k] >= 1 {
			atLeastOne = true
		}
	}
	if !atLeastOne {
		return fmt.Errorf("%w: at least one tier1 weight must be >= 1", errValidation)
	}
	for k, v := range d.Tier1Weights {
		if v < 0 || v > 5 {
			return fmt.Errorf("%w: tier1 weight %q is %d, must be 0..5", errValidation, k, v)
		}
	}
	for k, v := range d.Tier2Weights {
		if v < 0 || v > 5 {
			return fmt.Errorf("%w: tier2 weight %q is %d, must be 0..5", errValidation, k, v)
		}
	}
	allowed, err := s.tier2AllowedSet()
	if err != nil {
		return err
	}
	var unknown []string
	for k := range d.Tier2Weights {
		if !allowed[k] {
			unknown = append(unknown, k)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return fmt.Errorf("%w: unknown tier2 categories: %s", errValidation, strings.Join(unknown, ", "))
	}
	return nil
}

// tier2AllowedSet is the tier-2 vocabulary for overrides: pick.CategoryNames
// unioned with the [groups.*] slugs (B05 custom benchmark groups).
func (s *Services) tier2AllowedSet() (map[string]bool, error) {
	allowed := make(map[string]bool, len(pick.CategoryNames))
	for _, name := range pick.CategoryNames {
		allowed[name] = true
	}
	groups, err := s.cfg.LoadGroups()
	if err != nil {
		return nil, err
	}
	for slug := range groups {
		allowed[slug] = true
	}
	return allowed, nil
}

// effectiveHolds returns the candidate cap: reqHolds 0 => [gui].holds,
// otherwise reqHolds must be one of {1, 3, 5} -> else errValidation
// (SPEC §2.8).
func (s *Services) effectiveHolds(reqHolds int) (int, error) {
	holds := reqHolds
	if holds == 0 {
		gui, err := s.cfg.LoadGUI()
		if err != nil {
			return 0, fmt.Errorf("%w: %v", errValidation, err)
		}
		holds = gui.Holds
	}
	if holds != 1 && holds != 3 && holds != 5 {
		return 0, fmt.Errorf("%w: holds %d must be 1, 3 or 5", errValidation, holds)
	}
	return holds, nil
}

// availableIdentities returns the availability set (B00 CONTRACTS §6.3,
// SPEC §2.4): routes table ∩ enabled providers − [routes.disabled], one
// Identity per route, deduplicated. Set semantics: order-independent.
func (s *Services) availableIdentities() []pick.Identity {
	seen := make(map[pick.Identity]bool)
	var out []pick.Identity
	for _, route := range s.routes.Routes {
		if !s.routeEnabled(route) {
			continue
		}
		id := pick.Identity{Model: route.Model, Reasoning: route.Reasoning}
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

// resolveRoute picks the highest-priority (lowest priority, ties by provider
// id ascending) enabled, non-disabled route for one catalog (model,
// reasoning). ok == false never occurs for a candidate that passed
// availability (SPEC §2.7).
func (s *Services) resolveRoute(model, reasoning string) (routing.Route, bool) {
	type candidate struct {
		route routing.Route
		prio  int
	}
	var matches []candidate
	for _, route := range s.routes.Routes {
		if route.Model != model || route.Reasoning != reasoning {
			continue
		}
		if !s.routeEnabled(route) {
			continue
		}
		prio := s.cfg.Providers[route.Provider].Priority
		matches = append(matches, candidate{route: route, prio: prio})
	}
	if len(matches) == 0 {
		return routing.Route{}, false
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].prio != matches[j].prio {
			return matches[i].prio < matches[j].prio
		}
		return matches[i].route.Provider < matches[j].route.Provider
	})
	return matches[0].route, true
}

func (s *Services) scoreRow(model, reasoning string) (catalog.ScoreRow, bool) {
	for _, row := range s.scores {
		if strings.EqualFold(row.Model, model) && strings.EqualFold(row.Reasoning, reasoning) {
			return row, true
		}
	}
	return catalog.ScoreRow{}, false
}

// routeEnabled reports whether a route is an availability member: its
// provider is enabled (providers.<id>.enabled == true; absent => disabled)
// AND its model_id@reasoning is not disabled under [routes.disabled].
func (s *Services) routeEnabled(route routing.Route) bool {
	provider, ok := s.cfg.Providers[route.Provider]
	if !ok || !provider.Enabled {
		return false
	}
	disabled, err := s.cfg.LoadRoutesDisabled()
	if err != nil {
		return true
	}
	for _, key := range disabled[route.Provider] {
		if key == route.ModelID+"@"+route.Reasoning {
			return false
		}
	}
	return true
}

// RecordPick appends one history line to <StateDir>/pick/history.jsonl and
// emits pick:recorded{profile_slug, route_key} (SPEC §2.10). routeKey must
// match the D00 route-key grammar; profileSlug must resolve (builtin or
// [profiles.*]). Write failure -> io_error, no event.
func (s *Services) RecordPick(ctx context.Context, profileSlug, routeKey string) error {
	_ = ctx
	if _, _, _, err := ParseRouteKey(routeKey); err != nil {
		return err
	}
	s.mu.RLock()
	_, err := s.profileBySlug(profileSlug)
	s.mu.RUnlock()
	if err != nil {
		return err
	}

	evidence, err := json.Marshal(guiEvidence{
		Profile:            profileSlug,
		ScoreInputs:        map[string]any{},
		RouteProvenance:    "user_declared",
		ExcludedCandidates: []any{},
	})
	if err != nil {
		return fmt.Errorf("pick: record evidence: %w", err)
	}
	entry := PickHistoryEntry{
		ULID:          ulid.Make().String(),
		TS:            time.Now().UTC().Format(time.RFC3339),
		Profile:       profileSlug,
		Strategy:      "gui",
		CandidateID:   routeKey,
		FinalScore:    0,
		ExcludedCount: 0,
		Evidence:      evidence,
	}
	path := filepath.Join(s.paths.StateDir, "pick", "history.jsonl")
	if err := AppendPick(path, entry); err != nil {
		return fmt.Errorf("pick: record: %w", err)
	}
	s.emit(EventPickRecorded, map[string]string{"profile_slug": profileSlug, "route_key": routeKey})
	return nil
}

// guiEvidence is the aggregation-grade evidence object RecordPick writes
// (B04 CONTRACTS §4): only the four always-serialized fields; band/
// snapshot_age_seconds/confidence/last_verified are omitted.
type guiEvidence struct {
	Profile            string         `json:"profile"`
	ScoreInputs        map[string]any `json:"score_inputs"`
	RouteProvenance    string         `json:"route_provenance"`
	ExcludedCandidates []any          `json:"excluded_candidates"`
}

// CatalogLine returns the popover catalog summary (SPEC §2.11): Models =
// distinct (model, reasoning) scores rows; ProvidersOn = enabled provider
// count; Harnesses = [harnesses.*] entries when present else the 4 builtin
// seeds. Read-only: never seeds harnesses, never emits.
func (s *Services) CatalogLine(ctx context.Context) (CatalogSummary, error) {
	_ = ctx
	s.mu.RLock()
	defer s.mu.RUnlock()
	providersOn := 0
	for _, p := range s.cfg.Providers {
		if p.Enabled {
			providersOn++
		}
	}
	harnesses := len(harnessSeeds)
	if hs, err := s.cfg.LoadHarnesses(); err == nil && len(hs) > 0 {
		harnesses = len(hs)
	}
	return CatalogSummary{
		Models:      len(s.scores),
		ProvidersOn: providersOn,
		Harnesses:   harnesses,
	}, nil
}

// profileCategories reads the current configuration under the caller's lock.
func (s *Services) profileCategories() ([]string, error) {
	allowed, err := s.tier2AllowedSet()
	if err != nil {
		return nil, err
	}
	categories := make([]string, 0, len(allowed))
	for name := range allowed {
		categories = append(categories, name)
	}
	sort.Strings(categories)
	return categories, nil
}
