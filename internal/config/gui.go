package config

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Route-key grammar re-declared from specs/desktop/global CONTRACTS.md §1
// (kept in sync by convention with service.ParseRouteKey):
//
//	route_key = provider "/" model_id "@" reasoning
//	provider  = [a-z0-9_]+
//	model_id  = [A-Za-z0-9._-]+
//	reasoning = "minimal"|"low"|"medium"|"high"|"xhigh"|"max"|"default"
var (
	slugPattern     = regexp.MustCompile(`^[a-z0-9_]+$`)
	routeKeyPattern = regexp.MustCompile(`^[a-z0-9_]+/[A-Za-z0-9._-]+@(?:minimal|low|medium|high|xhigh|max|default)$`)
	// routePattern is a route key minus the provider segment, as stored under
	// [routes.disabled].<provider>.
	routePattern = regexp.MustCompile(`^[A-Za-z0-9._-]+@(?:minimal|low|medium|high|xhigh|max|default)$`)
)

// ProfileTOML mirrors one [profiles.<slug>] table. Weights are ints 1..5;
// zero-weight keys are never stored (SPEC §4 Decisions).
type ProfileTOML struct {
	CoreShare int            `toml:"core_share"` // 10..90, step 5
	Tier1     map[string]int `toml:"tier1"`      // keys exactly intelligence/cost/speed
	Tier2     map[string]int `toml:"tier2"`      // keys ⊆ categories ∪ [groups.*] slugs
}

type ProfilesTOML map[string]ProfileTOML

// HarnessTOML mirrors one [harnesses.<slug>] table (seeded by B07).
type HarnessTOML struct {
	Name      string   `toml:"name"`
	Command   string   `toml:"command"`   // template; token semantics are B07's
	Providers []string `toml:"providers"` // provider slugs
	Builtin   bool     `toml:"builtin"`
}

type HarnessesTOML map[string]HarnessTOML

// FavouritesTOML mirrors [favourites]. Pins are D00 §1 route keys.
type FavouritesTOML struct {
	Pins []string `toml:"pins"`
}

// RoutesDisabledTOML mirrors [routes.disabled]: provider -> "model_id@reasoning".
type RoutesDisabledTOML map[string][]string

// GroupTOML mirrors one [groups.<slug>] custom benchmark group.
type GroupTOML struct {
	Benchmarks []string `toml:"benchmarks"`
}

type GroupsTOML map[string]GroupTOML

// GUIConfig mirrors [gui]; field meanings and value sets are D00 GUISettings
// (which adds the transport-only ConfigPath — deliberately absent here).
type GUIConfig struct {
	Layout                  string `toml:"layout"`
	DefaultTab              string `toml:"default_tab"`
	WeightControl           string `toml:"weight_control"`
	Holds                   int    `toml:"holds"`
	Shortcut                string `toml:"shortcut"`
	ShowMenuBarIcon         bool   `toml:"show_menu_bar_icon"`
	LaunchAtLogin           bool   `toml:"launch_at_login"`
	CopyCommandInstead      bool   `toml:"copy_command_instead"`
	ClosePopoverAfterLaunch bool   `toml:"close_popover_after_launch"`
	AutoUpdate              bool   `toml:"auto_update"`
	AutoUpdateFrequency     string `toml:"auto_update_frequency"`
	MCPServer               bool   `toml:"mcp_server"`
	ClaudeMDHint            bool   `toml:"claude_md_hint"`
	ShellAlias              bool   `toml:"shell_alias"`
	CatalogRepo             string `toml:"catalog_repo"`
	UseLocalAA              bool   `toml:"use_local_aa"`
	BenchmarkCheckFrequency string `toml:"benchmark_check_frequency"`
	OnlyEnabledProviders    bool   `toml:"only_enabled_providers"`
}

// guiConfigTOML is the pointered decode mirror of GUIConfig: each key falls
// back to its DefaultGUIConfig value independently when unset.
type guiConfigTOML struct {
	Layout                  *string `toml:"layout"`
	DefaultTab              *string `toml:"default_tab"`
	WeightControl           *string `toml:"weight_control"`
	Holds                   *int    `toml:"holds"`
	Shortcut                *string `toml:"shortcut"`
	ShowMenuBarIcon         *bool   `toml:"show_menu_bar_icon"`
	LaunchAtLogin           *bool   `toml:"launch_at_login"`
	CopyCommandInstead      *bool   `toml:"copy_command_instead"`
	ClosePopoverAfterLaunch *bool   `toml:"close_popover_after_launch"`
	AutoUpdate              *bool   `toml:"auto_update"`
	AutoUpdateFrequency     *string `toml:"auto_update_frequency"`
	MCPServer               *bool   `toml:"mcp_server"`
	ClaudeMDHint            *bool   `toml:"claude_md_hint"`
	ShellAlias              *bool   `toml:"shell_alias"`
	CatalogRepo             *string `toml:"catalog_repo"`
	UseLocalAA              *bool   `toml:"use_local_aa"`
	BenchmarkCheckFrequency *string `toml:"benchmark_check_frequency"`
	OnlyEnabledProviders    *bool   `toml:"only_enabled_providers"`
}

// DefaultGUIConfig returns the [gui] per-key defaults (CONTRACTS §4).
func DefaultGUIConfig() GUIConfig {
	return GUIConfig{
		Layout: "carousel",
		// The popover opens on the profile picker; the complexity slider is the
		// other tab. Shipped default is profiles.
		DefaultTab:              "profiles",
		WeightControl:           "slider",
		Holds:                   5,
		Shortcut:                "alt+space",
		ShowMenuBarIcon:         true,
		LaunchAtLogin:           false,
		CopyCommandInstead:      false,
		ClosePopoverAfterLaunch: true,
		AutoUpdate:              true,
		AutoUpdateFrequency:     "daily",
		MCPServer:               false,
		ClaudeMDHint:            false,
		ShellAlias:              false,
		CatalogRepo:             DefaultCatalogRepo,
		UseLocalAA:              false,
		BenchmarkCheckFrequency: "6h",
		OnlyEnabledProviders:    false,
	}
}

func invalidValue(key, format string, args ...any) error {
	return &ConfigError{Kind: KindInvalidValue, Key: key, Err: fmt.Errorf(format, args...)}
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// LoadGUI decodes [gui] with per-key defaults, then validates (§5 U1..U5).
func (c *Config) LoadGUI() (GUIConfig, error) {
	gui := DefaultGUIConfig()
	var mirror guiConfigTOML
	if err := c.UnmarshalKey("gui", &mirror); err != nil {
		return GUIConfig{}, err
	}
	if mirror.Layout != nil {
		gui.Layout = *mirror.Layout
	}
	if mirror.DefaultTab != nil {
		gui.DefaultTab = *mirror.DefaultTab
	}
	if mirror.WeightControl != nil {
		gui.WeightControl = *mirror.WeightControl
	}
	if mirror.Holds != nil {
		gui.Holds = *mirror.Holds
	}
	if mirror.Shortcut != nil {
		gui.Shortcut = *mirror.Shortcut
	}
	if mirror.ShowMenuBarIcon != nil {
		gui.ShowMenuBarIcon = *mirror.ShowMenuBarIcon
	}
	if mirror.LaunchAtLogin != nil {
		gui.LaunchAtLogin = *mirror.LaunchAtLogin
	}
	if mirror.CopyCommandInstead != nil {
		gui.CopyCommandInstead = *mirror.CopyCommandInstead
	}
	if mirror.ClosePopoverAfterLaunch != nil {
		gui.ClosePopoverAfterLaunch = *mirror.ClosePopoverAfterLaunch
	}
	if mirror.AutoUpdate != nil {
		gui.AutoUpdate = *mirror.AutoUpdate
	}
	if mirror.AutoUpdateFrequency != nil {
		gui.AutoUpdateFrequency = *mirror.AutoUpdateFrequency
	}
	if mirror.MCPServer != nil {
		gui.MCPServer = *mirror.MCPServer
	}
	if mirror.ClaudeMDHint != nil {
		gui.ClaudeMDHint = *mirror.ClaudeMDHint
	}
	if mirror.ShellAlias != nil {
		gui.ShellAlias = *mirror.ShellAlias
	}
	if mirror.CatalogRepo != nil {
		gui.CatalogRepo = *mirror.CatalogRepo
	}
	if mirror.UseLocalAA != nil {
		gui.UseLocalAA = *mirror.UseLocalAA
	}
	if mirror.BenchmarkCheckFrequency != nil {
		gui.BenchmarkCheckFrequency = *mirror.BenchmarkCheckFrequency
	}
	if strings.TrimSpace(gui.BenchmarkCheckFrequency) == "" {
		gui.BenchmarkCheckFrequency = "6h"
	}
	if mirror.OnlyEnabledProviders != nil {
		gui.OnlyEnabledProviders = *mirror.OnlyEnabledProviders
	}
	if strings.TrimSpace(gui.CatalogRepo) == "" {
		gui.CatalogRepo = DefaultCatalogRepo
	}
	if err := validateGUI(gui); err != nil {
		return GUIConfig{}, err
	}
	return gui, nil
}

// LoadProfiles decodes [profiles.*] and validates (§5 P1..P7). categories is
// the canonical tier2 vocabulary (callers pass pick.CategoryNames); it is
// unioned with the [groups.*] slugs internally.
func (c *Config) LoadProfiles(categories []string) (ProfilesTOML, error) {
	var profiles ProfilesTOML
	if err := c.UnmarshalKey("profiles", &profiles); err != nil {
		return nil, err
	}
	if profiles == nil {
		profiles = ProfilesTOML{}
	}
	allowed := c.tier2Vocabulary(categories)
	for _, slug := range sortedKeys(profiles) {
		if err := validateProfile(slug, profiles[slug], allowed); err != nil {
			return nil, err
		}
	}
	return profiles, nil
}

// LoadHarnesses decodes [harnesses.*] and validates (§5 H1..H4).
func (c *Config) LoadHarnesses() (HarnessesTOML, error) {
	var harnesses HarnessesTOML
	if err := c.UnmarshalKey("harnesses", &harnesses); err != nil {
		return nil, err
	}
	if harnesses == nil {
		harnesses = HarnessesTOML{}
	}
	for _, slug := range sortedKeys(harnesses) {
		if err := validateHarness(slug, harnesses[slug]); err != nil {
			return nil, err
		}
	}
	return harnesses, nil
}

// LoadFavourites decodes [favourites] and validates (§5 F1..F2).
func (c *Config) LoadFavourites() (FavouritesTOML, error) {
	var favourites FavouritesTOML
	if err := c.UnmarshalKey("favourites", &favourites); err != nil {
		return FavouritesTOML{}, err
	}
	if err := validateFavourites(favourites); err != nil {
		return FavouritesTOML{}, err
	}
	return favourites, nil
}

// LoadRoutesDisabled decodes [routes.disabled] and validates (§5 R1..R3).
func (c *Config) LoadRoutesDisabled() (RoutesDisabledTOML, error) {
	var disabled RoutesDisabledTOML
	if err := c.UnmarshalKey("routes.disabled", &disabled); err != nil {
		return nil, err
	}
	if disabled == nil {
		disabled = RoutesDisabledTOML{}
	}
	if err := validateRoutesDisabled(disabled); err != nil {
		return nil, err
	}
	return disabled, nil
}

// LoadGroups decodes [groups.*] and validates (§5 G1..G4).
func (c *Config) LoadGroups() (GroupsTOML, error) {
	var groups GroupsTOML
	if err := c.UnmarshalKey("groups", &groups); err != nil {
		return nil, err
	}
	if groups == nil {
		groups = GroupsTOML{}
	}
	for _, slug := range sortedKeys(groups) {
		if err := validateGroup(slug, groups[slug]); err != nil {
			return nil, err
		}
	}
	return groups, nil
}

// SetGUI validates then writes every [gui] key into the raw document
// (a saved config is self-describing; defaults apply only to absent keys).
func (c *Config) SetGUI(g GUIConfig) error {
	if strings.TrimSpace(g.CatalogRepo) == "" {
		g.CatalogRepo = DefaultCatalogRepo
	}
	if strings.TrimSpace(g.BenchmarkCheckFrequency) == "" {
		g.BenchmarkCheckFrequency = "6h"
	}
	if err := validateGUI(g); err != nil {
		return err
	}
	c.setRaw("gui", map[string]any{
		"layout":                     g.Layout,
		"default_tab":                g.DefaultTab,
		"weight_control":             g.WeightControl,
		"holds":                      int64(g.Holds),
		"shortcut":                   g.Shortcut,
		"show_menu_bar_icon":         g.ShowMenuBarIcon,
		"launch_at_login":            g.LaunchAtLogin,
		"copy_command_instead":       g.CopyCommandInstead,
		"close_popover_after_launch": g.ClosePopoverAfterLaunch,
		"auto_update":                g.AutoUpdate,
		"auto_update_frequency":      g.AutoUpdateFrequency,
		"mcp_server":                 g.MCPServer,
		"claude_md_hint":             g.ClaudeMDHint,
		"shell_alias":                g.ShellAlias,
		"catalog_repo":               g.CatalogRepo,
		"use_local_aa":               g.UseLocalAA,
		"benchmark_check_frequency":  g.BenchmarkCheckFrequency,
		"only_enabled_providers":     g.OnlyEnabledProviders,
	})
	return nil
}

// SetProfile validates then writes [profiles.<slug>] into the raw document.
func (c *Config) SetProfile(slug string, p ProfileTOML, categories []string) error {
	if err := validateProfile(slug, p, c.tier2Vocabulary(categories)); err != nil {
		return err
	}
	table := map[string]any{
		"core_share": int64(p.CoreShare),
		"tier1":      intTable(p.Tier1),
	}
	if len(p.Tier2) > 0 {
		table["tier2"] = intTable(p.Tier2)
	}
	c.setRaw("profiles."+slug, table)
	return nil
}

// SetHarness validates then writes [harnesses.<slug>] into the raw document.
func (c *Config) SetHarness(slug string, h HarnessTOML) error {
	if err := validateHarness(slug, h); err != nil {
		return err
	}
	c.setRaw("harnesses."+slug, map[string]any{
		"name":      h.Name,
		"command":   h.Command,
		"providers": stringList(h.Providers),
		"builtin":   h.Builtin,
	})
	return nil
}

// SetFavourites validates then writes [favourites] into the raw document.
func (c *Config) SetFavourites(f FavouritesTOML) error {
	if err := validateFavourites(f); err != nil {
		return err
	}
	c.setRaw("favourites", map[string]any{"pins": stringList(f.Pins)})
	return nil
}

// SetRoutesDisabled validates then writes [routes.disabled] into the raw
// document, preserving any other [routes] keys.
func (c *Config) SetRoutesDisabled(r RoutesDisabledTOML) error {
	if err := validateRoutesDisabled(r); err != nil {
		return err
	}
	disabled := make(map[string]any, len(r))
	for provider, routes := range r {
		disabled[provider] = stringList(routes)
	}
	c.setRaw("routes.disabled", disabled)
	return nil
}

// SetGroup validates then writes [groups.<slug>] into the raw document.
func (c *Config) SetGroup(slug string, g GroupTOML) error {
	if err := validateGroup(slug, g); err != nil {
		return err
	}
	c.setRaw("groups."+slug, map[string]any{"benchmarks": stringList(g.Benchmarks)})
	return nil
}

// DeleteProfile removes [profiles.<slug>]; idempotent, never errors.
func (c *Config) DeleteProfile(slug string) { c.deleteRawChild("profiles", slug) }

// DeleteHarness removes [harnesses.<slug>]; idempotent, never errors.
func (c *Config) DeleteHarness(slug string) { c.deleteRawChild("harnesses", slug) }

// DeleteGroup removes [groups.<slug>]; idempotent, never errors.
func (c *Config) DeleteGroup(slug string) { c.deleteRawChild("groups", slug) }

func (c *Config) setRaw(dotted string, value any) {
	if c.raw == nil {
		c.raw = make(map[string]any)
	}
	setKey(c.raw, dotted, value)
}

func (c *Config) deleteRawChild(section, slug string) {
	parent, ok := c.raw[section].(map[string]any)
	if !ok {
		return
	}
	delete(parent, slug)
	if len(parent) == 0 {
		delete(c.raw, section)
	}
}

// tier2Vocabulary unions the caller-supplied categories with the [groups.*]
// slugs present in the raw document (SPEC §2.4).
func (c *Config) tier2Vocabulary(categories []string) map[string]bool {
	allowed := make(map[string]bool, len(categories))
	for _, category := range categories {
		allowed[category] = true
	}
	if groups, ok := rawLookup(c.raw, "groups").(map[string]any); ok {
		for slug := range groups {
			allowed[slug] = true
		}
	}
	return allowed
}

func intTable(m map[string]int) map[string]any {
	table := make(map[string]any, len(m))
	for key, value := range m {
		table[key] = int64(value)
	}
	return table
}

func stringList(values []string) []any {
	list := make([]any, len(values))
	for i, value := range values {
		list[i] = value
	}
	return list
}

func validateProfile(slug string, p ProfileTOML, allowedTier2 map[string]bool) error {
	if !slugPattern.MatchString(slug) {
		return invalidValue("profiles."+slug, "slug must match [a-z0-9_]+")
	}
	if p.CoreShare < 10 || p.CoreShare > 90 {
		return invalidValue("profiles."+slug+".core_share", "must be between 10 and 90")
	}
	if p.CoreShare%5 != 0 {
		return invalidValue("profiles."+slug+".core_share", "must be a multiple of 5")
	}
	axes := []string{"intelligence", "cost", "speed"}
	if len(p.Tier1) != len(axes) {
		return invalidValue("profiles."+slug+".tier1", "keys must be exactly intelligence, cost, speed")
	}
	for _, axis := range axes {
		if _, ok := p.Tier1[axis]; !ok {
			return invalidValue("profiles."+slug+".tier1", "keys must be exactly intelligence, cost, speed")
		}
	}
	for _, axis := range axes {
		if value := p.Tier1[axis]; value < 1 || value > 5 {
			return invalidValue("profiles."+slug+".tier1."+axis, "must be between 1 and 5")
		}
	}
	for _, key := range sortedKeys(p.Tier2) {
		if !allowedTier2[key] {
			return invalidValue("profiles."+slug+".tier2."+key, "unknown tier2 category")
		}
		if value := p.Tier2[key]; value < 1 || value > 5 {
			return invalidValue("profiles."+slug+".tier2."+key, "must be between 1 and 5")
		}
	}
	return nil
}

func validateHarness(slug string, h HarnessTOML) error {
	if !slugPattern.MatchString(slug) {
		return invalidValue("harnesses."+slug, "slug must match [a-z0-9_]+")
	}
	if h.Name == "" {
		return invalidValue("harnesses."+slug+".name", "must not be empty")
	}
	if h.Command == "" {
		return invalidValue("harnesses."+slug+".command", "must not be empty")
	}
	for _, provider := range h.Providers {
		if !slugPattern.MatchString(provider) {
			return invalidValue("harnesses."+slug+".providers", "provider %q must match [a-z0-9_]+", provider)
		}
	}
	return nil
}

func validateFavourites(f FavouritesTOML) error {
	seen := make(map[string]bool, len(f.Pins))
	for _, pin := range f.Pins {
		if !routeKeyPattern.MatchString(pin) {
			return invalidValue("favourites.pins", "invalid route key %q", pin)
		}
		if seen[pin] {
			return invalidValue("favourites.pins", "duplicate pin %q", pin)
		}
		seen[pin] = true
	}
	return nil
}

func validateRoutesDisabled(r RoutesDisabledTOML) error {
	for _, provider := range sortedKeys(r) {
		if !slugPattern.MatchString(provider) {
			return invalidValue("routes.disabled", "provider %q must match [a-z0-9_]+", provider)
		}
		seen := make(map[string]bool, len(r[provider]))
		for _, route := range r[provider] {
			if !routePattern.MatchString(route) {
				return invalidValue("routes.disabled."+provider, "invalid route %q", route)
			}
			if seen[route] {
				return invalidValue("routes.disabled."+provider, "duplicate route %q", route)
			}
			seen[route] = true
		}
	}
	return nil
}

func validateGroup(slug string, g GroupTOML) error {
	if !slugPattern.MatchString(slug) {
		return invalidValue("groups."+slug, "slug must match [a-z0-9_]+")
	}
	if len(g.Benchmarks) == 0 {
		return invalidValue("groups."+slug+".benchmarks", "must not be empty")
	}
	seen := make(map[string]bool, len(g.Benchmarks))
	for _, benchmark := range g.Benchmarks {
		if benchmark == "" {
			return invalidValue("groups."+slug+".benchmarks", "benchmark name must not be empty")
		}
		if seen[benchmark] {
			return invalidValue("groups."+slug+".benchmarks", "duplicate benchmark %q", benchmark)
		}
		seen[benchmark] = true
	}
	return nil
}

// DefaultCatalogRepo is the GitHub repository desktop Settings pulls
// scores from unless the user picks another. Ref defaults to main.
const DefaultCatalogRepo = "WD-Mitchell/which-model"

const defaultCatalogRef = "main"

// ParseCatalogRepoSpec accepts "owner/repo", "owner/repo@ref", or a
// github.com URL. Empty spec is the shipped default.
func ParseCatalogRepoSpec(spec string) (owner, repo, ref string, err error) {
	s := strings.TrimSpace(spec)
	if s == "" {
		s = DefaultCatalogRepo
	}
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "http://")
	s = strings.TrimPrefix(s, "github.com/")
	s = strings.TrimSuffix(s, ".git")
	ref = defaultCatalogRef
	if i := strings.LastIndex(s, "@"); i >= 0 && !strings.Contains(s[i+1:], "/") {
		ref = s[i+1:]
		s = s[:i]
	}
	parts := strings.Split(s, "/")
	if len(parts) >= 4 && parts[2] == "tree" {
		ref = parts[3]
		parts = parts[:2]
	}
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", "", fmt.Errorf("want owner/repo")
	}
	owner, repo = parts[0], strings.TrimSuffix(parts[1], ".git")
	if !catalogRepoPartOK(owner) || !catalogRepoPartOK(repo) || ref == "" || strings.ContainsAny(ref, " /\\") {
		return "", "", "", fmt.Errorf("want owner/repo or owner/repo@ref")
	}
	return owner, repo, ref, nil
}

func catalogRepoPartOK(s string) bool {
	if s == "" || s == "." || s == ".." {
		return false
	}
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			continue
		}
		return false
	}
	return true
}

func validateGUI(g GUIConfig) error {
	switch g.Layout {
	case "carousel", "list":
	default:
		return invalidValue("gui.layout", `must be "carousel" or "list"`)
	}
	switch g.DefaultTab {
	case "profiles", "sliders":
	default:
		return invalidValue("gui.default_tab", `must be "profiles" or "sliders"`)
	}
	switch g.WeightControl {
	case "step", "bar", "slider":
	default:
		return invalidValue("gui.weight_control", `must be "step", "bar" or "slider"`)
	}
	switch g.Holds {
	case 3, 5, 10:
	default:
		return invalidValue("gui.holds", "must be 3, 5 or 10")
	}
	switch g.Shortcut {
	case "alt+space", "ctrl+space", "cmd+shift+m":
	default:
		return invalidValue("gui.shortcut", `must be "alt+space", "ctrl+space" or "cmd+shift+m"`)
	}
	switch g.AutoUpdateFrequency {
	case "hourly", "daily", "weekly", "monthly":
	default:
		return invalidValue("gui.auto_update_frequency", `must be "hourly", "daily", "weekly" or "monthly"`)
	}
	switch g.BenchmarkCheckFrequency {
	case "15m", "1h", "3h", "6h", "12h", "24h", "weekly":
	default:
		return invalidValue("gui.benchmark_check_frequency", `must be "15m", "1h", "3h", "6h", "12h", "24h" or "weekly"`)
	}
	if _, _, _, err := ParseCatalogRepoSpec(g.CatalogRepo); err != nil {
		return invalidValue("gui.catalog_repo", "%s", err.Error())
	}
	return nil
}
