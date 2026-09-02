package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	sdecimal "github.com/shopspring/decimal"

	"github.com/WD-Mitchell/which-model/internal/catalog"
	"github.com/WD-Mitchell/which-model/internal/catalog/csvstore"
	"github.com/WD-Mitchell/which-model/internal/catalog/identity"
	"github.com/WD-Mitchell/which-model/internal/catalog/score"
	"github.com/WD-Mitchell/which-model/internal/config"
	wdecimal "github.com/WD-Mitchell/which-model/internal/decimal"
	"github.com/WD-Mitchell/which-model/internal/pick"
	"github.com/WD-Mitchell/which-model/internal/routing"
)

// benchmarkNote is the fallback description written into every
// BenchmarkDetail. benchmarks.toml carries no per-benchmark metadata, so the
// field always holds this exact string (B05 SPEC §2.4; richer export metadata
// may populate it later without a contract change).
const benchmarkNote = "Carried in the model data export. No description recorded for this benchmark yet."

// modelKey identifies one (model, reasoning) row in a raw-benchmark map
// (mirrors the identity of one score CSV row, B05 SPEC §2.2).
type modelKey struct {
	model     string
	reasoning string
}

// CatalogService backs the Settings "Benchmark groups" page: the benchmark
// catalogue with per-benchmark coverage, the merged builtin+custom group
// list, and the custom-group mutations (B05 SPEC §1). Methods read the
// catalog caches under RLock; mutations follow the B02 single-writer
// discipline.
type CatalogService struct{ s *Services }

// Catalog exposes the benchmark-catalogue API on *Services.
func (s *Services) Catalog() *CatalogService { return &CatalogService{s: s} }

var (
	dec100     = sdecimal.NewFromInt(100)
	badSlugRun = regexp.MustCompile(`[^a-z0-9]+`)
)

// Benchmarks returns the full catalogue (B05 SPEC §2.1), sorted ascending.
func (c *CatalogService) Benchmarks(ctx context.Context) ([]string, error) {
	_ = ctx
	c.s.mu.RLock()
	defer c.s.mu.RUnlock()
	return c.s.catalogueLocked()
}

// BenchmarkDetail returns the fixed note, containing group slugs, and the
// tested rows with raw Value + Norm (value/max×100, Norm desc; SPEC §2.4).
func (c *CatalogService) BenchmarkDetail(ctx context.Context, name string) (BenchmarkDetail, error) {
	_ = ctx
	c.s.mu.RLock()
	defer c.s.mu.RUnlock()
	catalogue, err := c.s.catalogueLocked()
	if err != nil {
		return BenchmarkDetail{}, err
	}
	if !stringInSlice(catalogue, name) {
		return BenchmarkDetail{}, fmt.Errorf("%w: no benchmark %q", errNotFound, name)
	}

	return BenchmarkDetail{
		Name:   name,
		Note:   benchmarkNote,
		Groups: c.s.groupsForBenchmarkLocked(name),
		Rows:   c.s.benchmarkRowsLocked(name),
	}, nil
}

// ModelDetail returns every benchmark (model, reasoning) reports. Unknown or
// untested pairs return empty Rows (not not_found) so Settings can open any
// catalogue combo. Norm is value/max×100 against every tested model of that
// benchmark, matching BenchmarkDetail.
func (c *CatalogService) ModelDetail(ctx context.Context, model, reasoning string) (ModelScoreDetail, error) {
	_ = ctx
	c.s.mu.RLock()
	defer c.s.mu.RUnlock()
	cleaned := identity.CleanModelName(model)
	level := identity.CollapseReasoning(strings.TrimSpace(reasoning))
	out := ModelScoreDetail{Model: cleaned, Reasoning: level, Rows: []ModelBenchRow{}}
	if cleaned == "" || level == "" {
		return out, nil
	}
	catalogue, err := c.s.catalogueLocked()
	if err != nil {
		return ModelScoreDetail{}, err
	}
	key := modelKey{model: cleaned, reasoning: level}
	for _, name := range catalogue {
		rawMap := c.s.rawValues[name]
		value, ok := rawMap[key]
		if !ok {
			continue
		}
		max := sdecimal.Zero
		for _, candidate := range rawMap {
			if candidate.GreaterThan(max) {
				max = candidate
			}
		}
		norm := sdecimal.Zero
		if !max.IsZero() {
			norm = wdecimal.RoundHalfUp(value.Div(max).Mul(dec100), 0)
		}
		out.Rows = append(out.Rows, ModelBenchRow{
			Name:   name,
			Value:  value.InexactFloat64(),
			Norm:   norm.InexactFloat64(),
			Groups: c.s.groupsForBenchmarkLocked(name),
		})
	}
	sort.SliceStable(out.Rows, func(i, j int) bool {
		if out.Rows[i].Norm != out.Rows[j].Norm {
			return out.Rows[i].Norm > out.Rows[j].Norm
		}
		return out.Rows[i].Name < out.Rows[j].Name
	})
	return out, nil
}

// Models returns one CatalogModel per distinct scores-CSV display name,
// sorted name ascending (B05 SPEC §2.14).
func (c *CatalogService) Models(ctx context.Context) ([]CatalogModel, error) {
	_ = ctx
	c.s.mu.RLock()
	defer c.s.mu.RUnlock()
	return c.s.catalogModelsLocked(), nil
}

func tier1ScorePtr(row catalog.ScoreRow, axis pick.Tier1Axis) *float64 {
	v, ok := row.Tier1[pick.Tier1ScoreColumn[axis]]
	if !ok {
		return nil
	}
	f := v.InexactFloat64()
	return &f
}

func (s *Services) catalogModelsLocked() []CatalogModel {
	type acc struct {
		name               string
		reasoning          map[string]struct{}
		topRank            int
		hasTop             bool
		intel, cost, speed *float64
		ids                map[string]struct{}
		providers          map[string]struct{}
	}
	byName := make(map[string]*acc, len(s.scores))
	for _, row := range s.scores {
		a := byName[row.Model]
		if a == nil {
			a = &acc{
				name:      row.Model,
				reasoning: map[string]struct{}{},
				ids:       map[string]struct{}{},
				providers: map[string]struct{}{},
				topRank:   -1,
			}
			byName[row.Model] = a
		}
		level := identity.CollapseReasoning(row.Reasoning)
		a.reasoning[level] = struct{}{}
		rank, known := identity.EffortOrder[level]
		if !known {
			rank = -1
		}
		if !a.hasTop || rank >= a.topRank {
			a.hasTop = true
			a.topRank = rank
			a.intel = tier1ScorePtr(row, pick.AxisIntelligence)
			a.cost = tier1ScorePtr(row, pick.AxisCost)
			a.speed = tier1ScorePtr(row, pick.AxisSpeed)
		}
	}
	for _, route := range s.routes.Routes {
		a := byName[route.Model]
		if a == nil {
			continue
		}
		if route.ModelID != "" {
			a.ids[route.ModelID] = struct{}{}
		}
		if route.Provider != "" {
			a.providers[route.Provider] = struct{}{}
		}
	}
	for _, pID := range s.providerUniverseLocked() {
		for _, pm := range s.Providers().providerModelsLocked(pID) {
			cleaned := identity.CleanModelName(pm.ModelName)
			if a := byName[cleaned]; a != nil {
				for _, lvl := range pm.Levels {
					level := identity.CollapseReasoning(lvl.Reasoning)
					if level != "" {
						a.reasoning[level] = struct{}{}
					}
				}
				if pm.ModelID != "" {
					a.ids[pm.ModelID] = struct{}{}
				}
				if pID != "" {
					a.providers[pID] = struct{}{}
				}
			}
		}
	}
	names := make([]string, 0, len(byName))
	for name := range byName {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]CatalogModel, 0, len(names))
	for _, name := range names {
		a := byName[name]
		levels := make([]string, 0, len(a.reasoning))
		for level := range a.reasoning {
			levels = append(levels, level)
		}
		sort.SliceStable(levels, func(i, j int) bool {
			return reasoningLess(levels[i], levels[j])
		})
		ids := make([]string, 0, len(a.ids))
		for id := range a.ids {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		modelID := ""
		if len(ids) > 0 {
			modelID = ids[0]
		}
		out = append(out, CatalogModel{
			ModelName:     name,
			ModelID:       modelID,
			Reasoning:     levels,
			Intelligence:  a.intel,
			Cost:          a.cost,
			Speed:         a.speed,
			ProviderCount: len(a.providers),
		})
	}
	return out
}

// Model returns the catalog card for one scores-CSV display name: identity
// plus every enabled provider that actually routes to it, with models.dev
// input/output prices when the cache lists them (B05 SPEC §2.15).
func (c *CatalogService) Model(ctx context.Context, name string) (CatalogModelDetail, error) {
	_ = ctx
	cleaned := identity.CleanModelName(name)
	c.s.mu.RLock()
	defer c.s.mu.RUnlock()
	return c.s.catalogModelLocked(cleaned)
}

func (s *Services) catalogModelLocked(name string) (CatalogModelDetail, error) {
	list := s.catalogModelsLocked()
	var base *CatalogModel
	for i := range list {
		if list[i].ModelName == name {
			base = &list[i]
			break
		}
	}
	if base == nil {
		return CatalogModelDetail{}, fmt.Errorf("%w: no model %q", errNotFound, name)
	}
	allReasoning := make(map[string]struct{})
	for _, lvl := range base.Reasoning {
		allReasoning[lvl] = struct{}{}
	}
	type acc struct {
		provider, modelID string
		reasoning         map[string]struct{}
		keys              map[string]struct{}
	}
	byKey := map[string]*acc{}

	// 1. Scan enabled providers for all models and reasoning levels they offer matching this model.
	for _, pID := range s.providerUniverseLocked() {
		pcfg, ok := s.cfg.Providers[pID]
		if !ok || !pcfg.Enabled {
			continue
		}
		for _, pm := range s.Providers().providerModelsLocked(pID) {
			cleaned := identity.CleanModelName(pm.ModelName)
			if cleaned != base.ModelName && pm.ModelName != base.ModelName && (base.ModelID == "" || pm.ModelID != base.ModelID) {
				continue
			}
			join := pID + "|" + pm.ModelID
			a := byKey[join]
			if a == nil {
				a = &acc{
					provider:  pID,
					modelID:   pm.ModelID,
					reasoning: map[string]struct{}{},
					keys:      map[string]struct{}{},
				}
				byKey[join] = a
			}
			for _, lvl := range pm.Levels {
				level := identity.CollapseReasoning(lvl.Reasoning)
				if level != "" {
					a.reasoning[level] = struct{}{}
					allReasoning[level] = struct{}{}
					if pID != "" && pm.ModelID != "" {
						a.keys[FormatRouteKey(pID, pm.ModelID, level)] = struct{}{}
					}
				}
			}
		}
	}

	// 2. Also check any routes for this model from enabled providers.
	for _, route := range s.routes.Routes {
		if route.Model != base.ModelName {
			continue
		}
		pcfg, ok := s.cfg.Providers[route.Provider]
		if !ok || !pcfg.Enabled {
			continue
		}
		join := route.Provider + "|" + route.ModelID
		a := byKey[join]
		if a == nil {
			a = &acc{
				provider:  route.Provider,
				modelID:   route.ModelID,
				reasoning: map[string]struct{}{},
				keys:      map[string]struct{}{},
			}
			byKey[join] = a
		}
		level := identity.CollapseReasoning(route.Reasoning)
		if level != "" {
			a.reasoning[level] = struct{}{}
			allReasoning[level] = struct{}{}
			if route.Provider != "" && route.ModelID != "" {
				a.keys[FormatRouteKey(route.Provider, route.ModelID, level)] = struct{}{}
			}
		}
	}
	type costPair struct{ in, out *float64 }
	costs := map[string]costPair{}
	if cached, ok := readModelsDevCache(modelsDevCachePath(s.paths.CacheDir)); ok {
		for _, rec := range cached {
			if rec.InputCostUSDPerM != nil || rec.OutputCostUSDPerM != nil {
				pair := costPair{rec.InputCostUSDPerM, rec.OutputCostUSDPerM}
				costs[rec.Provider+"|"+rec.ModelID] = pair
				if rec.Name != "" {
					costs[rec.Provider+"|"+identity.CleanModelName(rec.Name)] = pair
				}
				if _, exists := costs[rec.ModelID]; !exists {
					costs[rec.ModelID] = pair
				}
				if rec.Name != "" {
					cleanedName := identity.CleanModelName(rec.Name)
					if _, exists := costs[cleanedName]; !exists {
						costs[cleanedName] = pair
					}
				}
			}
		}
	}
	joins := make([]string, 0, len(byKey))
	for join := range byKey {
		joins = append(joins, join)
	}
	sort.Strings(joins)
	providers := make([]CatalogModelProvider, 0, len(joins))
	for _, join := range joins {
		a := byKey[join]
		levels := make([]string, 0, len(a.reasoning))
		for level := range a.reasoning {
			levels = append(levels, level)
		}
		sort.SliceStable(levels, func(i, j int) bool {
			return reasoningLess(levels[i], levels[j])
		})
		routeKeys := make([]string, 0, len(a.keys))
		for rk := range a.keys {
			routeKeys = append(routeKeys, rk)
		}
		sort.Strings(routeKeys)
		slug := routing.CatalogueSlugFor(a.provider)
		pair, ok := costs[slug+"|"+a.modelID]
		if !ok {
			pair, ok = costs[a.provider+"|"+a.modelID]
		}
		if !ok {
			pair, ok = costs[slug+"|"+base.ModelName]
		}
		if !ok {
			pair, ok = costs[a.provider+"|"+base.ModelName]
		}
		if !ok {
			pair, ok = costs[a.modelID]
		}
		if !ok {
			pair = costs[base.ModelName]
		}
		providers = append(providers, CatalogModelProvider{
			Provider:          a.provider,
			ModelID:           a.modelID,
			Reasoning:         levels,
			RouteKeys:         routeKeys,
			InputCostUSDPerM:  pair.in,
			OutputCostUSDPerM: pair.out,
		})
	}
	reasoningList := make([]string, 0, len(allReasoning))
	for lvl := range allReasoning {
		reasoningList = append(reasoningList, lvl)
	}
	sort.SliceStable(reasoningList, func(i, j int) bool {
		return reasoningLess(reasoningList[i], reasoningList[j])
	})
	return CatalogModelDetail{
		ModelName:     base.ModelName,
		ModelID:       base.ModelID,
		Reasoning:     reasoningList,
		Intelligence:  base.Intelligence,
		Cost:          base.Cost,
		Speed:         base.Speed,
		ProviderCount: base.ProviderCount,
		Providers:     providers,
	}, nil
}

func (s *Services) groupsForBenchmarkLocked(name string) []string {
	var groupSlugs []string
	for _, eg := range s.benchConfig.EvidenceGroups {
		if stringInSlice(eg.Benchmarks, name) {
			groupSlugs = append(groupSlugs, eg.Category)
		}
	}
	customs, err := customGroupMap(s.cfg)
	if err != nil {
		return groupSlugs
	}
	for _, slug := range sortedKeys(customs) {
		if stringInSlice(customs[slug], name) {
			groupSlugs = append(groupSlugs, slug)
		}
	}
	return groupSlugs
}

// benchmarkRowsLocked builds the tested rows for one benchmark from the
// cached raw values: Value = raw, Norm = value/max×100 (half-up, integer
// valued), sorted Norm desc with model asc then reasoning asc tie-breaks.
// Caller holds at least RLock.
func (s *Services) benchmarkRowsLocked(name string) []BenchRow {
	raw := s.rawValues[name]
	if len(raw) == 0 {
		return nil
	}
	max := sdecimal.Zero
	for _, v := range raw {
		if v.GreaterThan(max) {
			max = v
		}
	}
	rows := make([]BenchRow, 0, len(raw))
	for key, v := range raw {
		norm := sdecimal.Zero
		if !max.IsZero() {
			norm = wdecimal.RoundHalfUp(v.Div(max).Mul(dec100), 0)
		}
		rows = append(rows, BenchRow{
			Model:     key.model,
			Reasoning: key.reasoning,
			Value:     v.InexactFloat64(),
			Norm:      norm.InexactFloat64(),
		})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Norm != rows[j].Norm {
			return rows[i].Norm > rows[j].Norm
		}
		if rows[i].Model != rows[j].Model {
			return rows[i].Model < rows[j].Model
		}
		return rows[i].Reasoning < rows[j].Reasoning
	})
	return rows
}

// Groups returns builtins (benchmarks.toml order) then customs (slug asc);
// InProfiles counts builtin+custom profiles weighting the slug (SPEC §2.5).
func (c *CatalogService) Groups(ctx context.Context) ([]GroupSummary, error) {
	_ = ctx
	c.s.mu.RLock()
	defer c.s.mu.RUnlock()
	customs, err := customGroupMap(c.s.cfg)
	if err != nil {
		return nil, err
	}
	profiles, err := c.s.cfg.LoadProfiles(pick.CategoryNames)
	if err != nil {
		return nil, err
	}
	out := make([]GroupSummary, 0, len(c.s.benchConfig.EvidenceGroups)+len(customs))
	for _, eg := range c.s.benchConfig.EvidenceGroups {
		out = append(out, GroupSummary{
			Slug:           eg.Category,
			Builtin:        true,
			BenchmarkCount: len(distinctStrings(eg.Benchmarks)),
			InProfiles:     inProfileCount(eg.Category, profiles),
		})
	}
	for _, slug := range sortedKeys(customs) {
		out = append(out, GroupSummary{
			Slug:           slug,
			BenchmarkCount: len(distinctStrings(customs[slug])),
			InProfiles:     inProfileCount(slug, profiles),
		})
	}
	return out, nil
}

// inProfileCount counts builtin (pick.Profiles) and custom profiles whose
// tier-2 weight for slug is > 0 (B05 SPEC §2.5).
func inProfileCount(slug string, customs config.ProfilesTOML) int {
	count := 0
	for _, bp := range pick.Profiles {
		if bp.Tier2Weights[slug].Sign() > 0 {
			count++
		}
	}
	for _, cp := range customs {
		if cp.Tier2[slug] > 0 {
			count++
		}
	}
	return count
}

// GroupDetail returns the full catalogue with On membership + coverage
// (SPEC §2.6).
func (c *CatalogService) GroupDetail(ctx context.Context, slug string) (GroupDetail, error) {
	_ = ctx
	c.s.mu.RLock()
	defer c.s.mu.RUnlock()
	return c.s.groupDetailLocked(slug)
}

// groupDetailLocked resolves one group's detail; caller holds RLock or Lock.
func (s *Services) groupDetailLocked(slug string) (GroupDetail, error) {
	catalogue, err := s.catalogueLocked()
	if err != nil {
		return GroupDetail{}, err
	}
	var builtin bool
	var members []string
	for _, eg := range s.benchConfig.EvidenceGroups {
		if eg.Category == slug {
			builtin = true
			members = eg.Benchmarks
			break
		}
	}
	if !builtin {
		customs, err := customGroupMap(s.cfg)
		if err != nil {
			return GroupDetail{}, err
		}
		g, ok := customs[slug]
		if !ok {
			return GroupDetail{}, fmt.Errorf("%w: no group %q", errNotFound, slug)
		}
		members = g
	}
	benchmarks := make([]GroupBenchmark, 0, len(catalogue))
	for _, name := range catalogue {
		benchmarks = append(benchmarks, GroupBenchmark{
			Name:          name,
			On:            stringInSlice(members, name),
			Covered:       len(s.rawValues[name]),
			CoverageTotal: len(s.scores),
		})
	}
	return GroupDetail{Slug: slug, Builtin: builtin, Benchmarks: benchmarks}, nil
}

// SaveGroup replaces a custom group's member list, optionally renaming it
// (rewriting custom-profile tier2 keys in the same atomic write), then runs
// the re-derive pipeline (SPEC §2.8, §2.10, §2.12).
func (c *CatalogService) SaveGroup(ctx context.Context, slug string, benchmarks []string, renameTo string) error {
	c.s.mu.Lock()
	defer c.s.mu.Unlock()

	rename := sanitizeGroupSlug(renameTo)
	if err := c.s.validateGroupSaveLocked(slug, benchmarks, rename); err != nil {
		return err
	}
	if err := c.s.persistGroupSave(slug, benchmarks, rename); err != nil {
		return err
	}
	if err := c.s.rederive(ctx); err != nil {
		c.s.emit(EventConfigChanged, map[string]string{"section": "groups"})
		return err
	}
	c.s.emit(EventCatalogChanged, struct{}{})
	return nil
}

// DuplicateGroup copies any group (builtin or custom) to <slug>_copy[_N]
// and runs the pipeline (SPEC §2.9).
func (c *CatalogService) DuplicateGroup(ctx context.Context, slug string) (GroupDetail, error) {
	c.s.mu.Lock()
	defer c.s.mu.Unlock()

	var members []string
	builtin := c.s.isBuiltinGroup(slug)
	if builtin {
		for _, eg := range c.s.benchConfig.EvidenceGroups {
			if eg.Category == slug {
				members = eg.Benchmarks
				break
			}
		}
	} else {
		customs, err := customGroupMap(c.s.cfg)
		if err != nil {
			return GroupDetail{}, err
		}
		g, ok := customs[slug]
		if !ok {
			return GroupDetail{}, fmt.Errorf("%w: no group %q", errNotFound, slug)
		}
		members = g
	}

	base := slug + "_copy"
	candidate := base
	for i := 2; c.s.groupSlugExists(candidate); i++ {
		candidate = fmt.Sprintf("%s_%d", base, i)
	}
	cp, err := c.s.catalogCopy()
	if err != nil {
		return GroupDetail{}, err
	}
	if err := cp.SetGroup(candidate, config.GroupTOML{Benchmarks: members}); err != nil {
		return GroupDetail{}, err
	}
	if err := c.s.catalogCommit(cp); err != nil {
		return GroupDetail{}, err
	}
	if err := c.s.rederive(ctx); err != nil {
		c.s.emit(EventConfigChanged, map[string]string{"section": "groups"})
		return GroupDetail{}, err
	}
	c.s.emit(EventCatalogChanged, struct{}{})
	return c.s.groupDetailLocked(candidate)
}

// DeleteGroup removes a custom group and strips its tier2 key from custom
// profiles in the same write, then runs the pipeline (SPEC §2.9).
func (c *CatalogService) DeleteGroup(ctx context.Context, slug string) error {
	c.s.mu.Lock()
	defer c.s.mu.Unlock()

	if c.s.isBuiltinGroup(slug) {
		return fmt.Errorf("%w: group %q is built-in and read-only", errBuiltinReadonly, slug)
	}
	customs, err := customGroupMap(c.s.cfg)
	if err != nil {
		return err
	}
	if _, ok := customs[slug]; !ok {
		return fmt.Errorf("%w: no group %q", errNotFound, slug)
	}

	cp, err := c.s.catalogCopy()
	if err != nil {
		return err
	}
	if err := catalogStripTier2(cp, slug); err != nil {
		return err
	}
	cp.DeleteGroup(slug)
	if err := c.s.catalogCommit(cp); err != nil {
		return err
	}
	if err := c.s.rederive(ctx); err != nil {
		c.s.emit(EventConfigChanged, map[string]string{"section": "groups"})
		return err
	}
	c.s.emit(EventCatalogChanged, struct{}{})
	return nil
}

// ----- unexported helpers (B05 CONTRACTS §3) -----

// sanitizeGroupSlug: trim, lowercase, replace every [^a-z0-9]+ run with "_",
// strip surrounding "_". "" means "no rename requested" (SPEC §2.7).
func sanitizeGroupSlug(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	s = badSlugRun.ReplaceAllString(s, "_")
	return strings.Trim(s, "_")
}

// rawBenchmarkValues parses the scores CSV's raw benchmark columns
// ("benchmark: <name>", the non-_score member of each pair) into
// benchmark -> (model,reasoning) -> raw value (SPEC §2.2).
func rawBenchmarkValues(scoresCSV []byte) (map[string]map[modelKey]sdecimal.Decimal, error) {
	data, err := stripLeadingProvenance(scoresCSV)
	if err != nil {
		return nil, err
	}
	reader := csv.NewReader(bytes.NewReader(data))
	reader.FieldsPerRecord = -1
	header, err := reader.Read()
	if err != nil {
		return nil, err
	}
	colIndex := make(map[string]int, len(header))
	var rawCols []string
	for i, name := range header {
		colIndex[name] = i
		if strings.HasPrefix(name, csvstore.BenchmarkColumnPrefix) && !strings.HasSuffix(name, "_score") {
			rawCols = append(rawCols, name)
		}
	}
	cell := func(fields []string, column string) string {
		idx, ok := colIndex[column]
		if !ok || idx >= len(fields) {
			return ""
		}
		return strings.TrimSpace(fields[idx])
	}

	out := make(map[string]map[modelKey]sdecimal.Decimal)
	for {
		fields, err := reader.Read()
		if err != nil {
			break // io.EOF (malformed middle rows are tolerated: raw cells are optional)
		}
		model := identity.CleanModelName(cell(fields, "model"))
		reasoning := identity.CollapseReasoning(cell(fields, "reasoning"))
		if model == "" || reasoning == "" {
			continue
		}
		key := modelKey{model: model, reasoning: reasoning}
		for _, column := range rawCols {
			raw := cell(fields, column)
			if raw == "" {
				continue
			}
			value, err := wdecimal.Parse(raw)
			if err != nil {
				return nil, fmt.Errorf("scores CSV row %s/%s %s must be numeric", model, reasoning, column)
			}
			name := strings.TrimPrefix(column, csvstore.BenchmarkColumnPrefix)
			if out[name] == nil {
				out[name] = make(map[modelKey]sdecimal.Decimal)
			}
			out[name][key] = value
		}
	}
	return out, nil
}

// stripLeadingProvenance drops a single leading "# ..." provenance line that
// Derive emits before the header (F06), so the raw parse sees only CSV rows.
func stripLeadingProvenance(data []byte) ([]byte, error) {
	if len(data) == 0 || data[0] != '#' {
		return data, nil
	}
	if idx := bytes.IndexByte(data, '\n'); idx >= 0 {
		return data[idx+1:], nil
	}
	return nil, fmt.Errorf("scores CSV contains only a provenance line")
}

// mergedBenchmarksTOML appends the custom groups to the builtin document as
// [benchmark_groups.<slug>] tables plus their slugs on
// benchmark_selection.groups (SPEC §2.10b; CONTRACTS §4).
func mergedBenchmarksTOML(builtin []byte, customs map[string][]string) ([]byte, error) {
	var doc struct {
		Selection struct {
			Groups     []string `toml:"groups"`
			Benchmarks []string `toml:"benchmarks"`
		} `toml:"benchmark_selection"`
		Groups map[string]struct {
			Benchmarks []string `toml:"benchmarks"`
		} `toml:"benchmark_groups"`
		Aliases map[string]string `toml:"benchmark_aliases"`
	}
	if _, err := toml.Decode(string(builtin), &doc); err != nil {
		return nil, err
	}
	if doc.Groups == nil {
		doc.Groups = make(map[string]struct {
			Benchmarks []string `toml:"benchmarks"`
		})
	}
	for _, slug := range sortedKeys(customs) {
		doc.Selection.Groups = append(doc.Selection.Groups, slug)
		doc.Groups[slug] = struct {
			Benchmarks []string `toml:"benchmarks"`
		}{Benchmarks: append([]string(nil), customs[slug]...)}
	}
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(doc); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// overlayCustomCategories sets Categories[slug] per SPEC §2.11: for each
// custom group and each row, the mean of the member benchmarks' derived
// scores present in row.Benchmarks (dedup by identity.BenchmarkKey, minimum
// 1 populated evidence, RoundHalfUp 0dp); absent when no member scored.
func overlayCustomCategories(rows []catalog.ScoreRow, customs map[string][]string) {
	for i := range rows {
		row := &rows[i]
		evidence := make(map[string]sdecimal.Decimal, len(row.Benchmarks))
		for name, v := range row.Benchmarks {
			evidence[identity.BenchmarkKey(name)] = v
		}
		for _, slug := range sortedKeys(customs) {
			var values []sdecimal.Decimal
			seen := make(map[string]bool)
			for _, name := range customs[slug] {
				key := identity.BenchmarkKey(name)
				if seen[key] {
					continue
				}
				seen[key] = true
				if v, ok := evidence[key]; ok {
					values = append(values, v)
				}
			}
			if len(values) == 0 {
				continue
			}
			sum := sdecimal.Zero
			for _, v := range values {
				sum = sum.Add(v)
			}
			mean := wdecimal.RoundHalfUp(
				sum.Div(sdecimal.NewFromInt(int64(len(values)))), 0)
			if row.Categories == nil {
				row.Categories = make(map[string]sdecimal.Decimal)
			}
			row.Categories[slug] = mean
		}
	}
}

// applyCatalogOverlay applies the custom-group category overlay to rows from
// the custom groups read out of cfg (SPEC §2.11). Called by reloadCatalog
// after every catalog (re)load; caller holds the write lock.
func applyCatalogOverlay(rows []catalog.ScoreRow, cfg *config.Config) error {
	customs, err := customGroupMap(cfg)
	if err != nil {
		return err
	}
	overlayCustomCategories(rows, customs)
	return nil
}

// rederive runs SPEC §2.10 (a)–(e) and returns the failing path on error.
// The caller holds the write lock; a failure here leaves the catalog caches
// untouched (the group persist that preceded it stays persisted, §2.12).
func (s *Services) rederive(ctx context.Context) error {
	rawPath := filepath.Join(s.paths.CacheDir, "catalog", "available_model_raw_values.csv")
	rawCSV, err := os.ReadFile(rawPath)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return fmt.Errorf("%s: %w", rawPath, ctxErr)
		}
		return fmt.Errorf("%s: %w", rawPath, err)
	}

	benchmarksPath := filepath.Join(s.paths.CacheDir, "catalog", "benchmarks.toml")
	if p := s.catalogBenchmarkPath(); p != "" {
		benchmarksPath = p
	}
	builtin, err := os.ReadFile(benchmarksPath)
	if err != nil {
		return fmt.Errorf("%s: %w", benchmarksPath, err)
	}

	customs, err := customGroupMap(s.cfg)
	if err != nil {
		return err
	}
	merged, err := mergedBenchmarksTOML(builtin, customs)
	if err != nil {
		return err
	}
	derived, err := score.Derive(rawCSV, merged, score.DefaultNormalizer(), score.DefaultAggregator())
	if err != nil {
		return err
	}

	scoresPath := filepath.Join(s.paths.CacheDir, "catalog", "available_model_scores.csv")
	if err := csvstore.WriteAtomicBytes(scoresPath, derived); err != nil {
		return fmt.Errorf("%s: %w", scoresPath, err)
	}

	if ctx.Err() != nil {
		return ctx.Err()
	}
	scores, err := score.ParseScoresCSV(derived)
	if err != nil {
		return err
	}
	rawValues, err := rawBenchmarkValues(derived)
	if err != nil {
		return err
	}
	s.scores = scores
	s.rawValues = rawValues
	overlayCustomCategories(s.scores, customs)
	return nil
}

// ----- config copy/mutate/persist/swap (B02 SPEC §2.6) -----

// validateGroupSaveLocked enforces the fixed SaveGroup check order (SPEC
// §2.8). rename is the already-sanitised rename target ("" = no rename).
func (s *Services) validateGroupSaveLocked(slug string, benchmarks []string, rename string) error {
	if s.isBuiltinGroup(slug) {
		return fmt.Errorf("%w: group %q is built-in and read-only", errBuiltinReadonly, slug)
	}
	customs, err := customGroupMap(s.cfg)
	if err != nil {
		return err
	}
	if _, ok := customs[slug]; !ok {
		return fmt.Errorf("%w: no group %q", errNotFound, slug)
	}
	if catalogue, err := s.catalogueLocked(); err != nil {
		return err
	} else {
		in := make(map[string]bool, len(catalogue))
		for _, name := range catalogue {
			in[name] = true
		}
		for _, b := range benchmarks {
			if !in[b] {
				return fmt.Errorf("%w: unknown benchmark %q in group %q", errValidation, b, slug)
			}
		}
	}
	if rename != "" && rename != slug {
		if s.groupSlugExists(rename) {
			return fmt.Errorf("%w: group %q already exists", errConflict, rename)
		}
	}
	return nil
}

// persistGroupSave writes the group's member list (under slug or the renamed
// slug) and, on rename, rewrites the tier2 weight key slug -> rename in every
// custom profile carrying it, in one atomic config write (SPEC §2.8). On any
// failure nothing is persisted and no event is emitted.
func (s *Services) persistGroupSave(slug string, benchmarks []string, rename string) error {
	cp, err := s.catalogCopy()
	if err != nil {
		return err
	}
	if rename == "" || rename == slug {
		if err := cp.SetGroup(slug, config.GroupTOML{Benchmarks: benchmarks}); err != nil {
			return err
		}
	} else {
		if err := cp.SetGroup(rename, config.GroupTOML{Benchmarks: benchmarks}); err != nil {
			return err
		}
		if err := catalogRewriteTier2(cp, slug, rename); err != nil {
			return err
		}
		cp.DeleteGroup(slug)
	}
	return s.catalogCommit(cp)
}

// catalogRewriteTier2 moves the tier2 weight key oldSlug -> newSlug in every
// custom profile carrying oldSlug (SPEC §2.8 rename).
func catalogRewriteTier2(cp *config.Config, oldSlug, newSlug string) error {
	profiles, err := cp.LoadProfiles(pick.CategoryNames)
	if err != nil {
		return err
	}
	for slug, p := range profiles {
		v, ok := p.Tier2[oldSlug]
		if !ok {
			continue
		}
		p.Tier2[newSlug] = v
		delete(p.Tier2, oldSlug)
		if err := cp.SetProfile(slug, p, pick.CategoryNames); err != nil {
			return err
		}
	}
	return nil
}

// catalogStripTier2 removes the tier2 key slug from every custom profile
// carrying it (SPEC §2.9 delete).
func catalogStripTier2(cp *config.Config, slug string) error {
	profiles, err := cp.LoadProfiles(pick.CategoryNames)
	if err != nil {
		return err
	}
	for pslug, p := range profiles {
		if _, ok := p.Tier2[slug]; !ok {
			continue
		}
		delete(p.Tier2, slug)
		if err := cp.SetProfile(pslug, p, pick.CategoryNames); err != nil {
			return err
		}
	}
	return nil
}

// catalogCopy deep-copies the in-memory config by a Marshal -> Load round-trip
// so a mutation can be built and validated before any durable write.
func (s *Services) catalogCopy() (*config.Config, error) {
	data, err := s.cfg.MarshalTOML()
	if err != nil {
		return nil, err
	}
	tmp := filepath.Join(s.paths.ConfigDir, ".catalog-cfg-copy.toml")
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return nil, err
	}
	cp, err := config.LoadFile(tmp)
	_ = os.Remove(tmp)
	if err != nil {
		return nil, err
	}
	return cp, nil
}

// catalogCommit atomically writes cp and swaps it into memory. On failure
// in-memory state is untouched (B02 SPEC §2.6).
func (s *Services) catalogCommit(cp *config.Config) error {
	data, err := cp.MarshalTOML()
	if err != nil {
		return err
	}
	if err := config.AtomicWriteFile(s.paths.UserConfigFile, data); err != nil {
		return err
	}
	s.cfg = cp
	return nil
}

// ----- small shared helpers -----

// catalogueLocked returns the union of config-listed, custom-group, and
// score-row benchmark names, deduplicated and sorted ascending (SPEC §2.1).
// Caller holds at least RLock.
func (s *Services) catalogueLocked() ([]string, error) {
	set := make(map[string]bool)
	for name := range s.benchConfig.CanonicalBenchmarks {
		set[name] = true
	}
	customs, err := customGroupMap(s.cfg)
	if err != nil {
		return nil, err
	}
	for _, members := range customs {
		for _, b := range members {
			set[b] = true
		}
	}
	for _, row := range s.scores {
		for b := range row.Benchmarks {
			set[b] = true
		}
	}
	names := make([]string, 0, len(set))
	for n := range set {
		names = append(names, n)
	}
	sort.Strings(names)
	return names, nil
}

// customGroupMap decodes [groups.*] into slug -> member list. Caller holds
// the lock appropriate for s.cfg reads.
func customGroupMap(cfg *config.Config) (map[string][]string, error) {
	groups, err := cfg.LoadGroups()
	if err != nil {
		return nil, err
	}
	out := make(map[string][]string, len(groups))
	for slug, g := range groups {
		out[slug] = append([]string(nil), g.Benchmarks...)
	}
	return out, nil
}

// isBuiltinGroup reports whether slug is a built-in group (benchmarks.toml
// benchmark_selection.groups). Caller holds the lock.
func (s *Services) isBuiltinGroup(slug string) bool {
	for _, eg := range s.benchConfig.EvidenceGroups {
		if eg.Category == slug {
			return true
		}
	}
	return false
}

// groupSlugExists reports whether slug names any group (builtin or custom).
// Caller holds the lock.
func (s *Services) groupSlugExists(slug string) bool {
	if s.isBuiltinGroup(slug) {
		return true
	}
	customs, err := customGroupMap(s.cfg)
	if err != nil {
		return false
	}
	_, ok := customs[slug]
	return ok
}

func stringInSlice(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// sortedKeys returns m's keys sorted ascending.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// distinctStrings returns the unique elements of list in first-occurrence
// order.
func distinctStrings(list []string) []string {
	seen := make(map[string]bool, len(list))
	out := make([]string, 0, len(list))
	for _, s := range list {
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}
