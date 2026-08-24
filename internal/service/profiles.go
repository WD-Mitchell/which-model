package service

import (
    "context"
    "fmt"
    "path/filepath"
    "regexp"
    "sort"
    "strings"
    "unicode"

    "github.com/shopspring/decimal"
    "github.com/WD-Mitchell/which-model/internal/catalog"
    "github.com/WD-Mitchell/which-model/internal/config"
    "github.com/WD-Mitchell/which-model/internal/pick"
)
type ProfileService struct { s *Services }

func (s *Services) Profiles() *ProfileService { return &ProfileService{s: s} }

var complexityScaleSlugs = []string{
    "simple_action_execution",
    "simple_implementation",
    "balanced_implementation",
    "research",
    "planning",
}

func validateComplexityScale(scale []string, profiles map[string]catalog.Profile) {
    for _, slug := range scale {
        if _, ok := profiles[slug]; !ok {
            panic(fmt.Sprintf("complexity scale profile %q is not a built-in profile", slug))
        }
    }
}

func init() { validateComplexityScale(complexityScaleSlugs, pick.Profiles) }

func (p *ProfileService) ComplexityScale() []string {
    return append([]string(nil), complexityScaleSlugs...)
}

// profileDisplayName turns a profile slug into the name the UI shows:
// "balanced_implementation" -> "Balanced Implementation".
//
// The engine's profile identifiers are snake_case and its Profile.Name is that
// same slug (internal/pick/profiles.go), so every surface — the popover, the
// settings list, the tray menu and the menu-bar title — was showing raw slugs.
// Title-casing here rather than in each front end keeps one spelling across
// all of them; the SLUG remains the id, and search still matches on it.
//
// Segments listed in profileNameAcronyms are upper-cased whole ("ui" -> "UI"),
// because title-casing an initialism reads as a typo.
func profileDisplayName(slug string) string {
    parts := strings.Split(slug, "_")
    out := make([]string, 0, len(parts))
    for _, part := range parts {
        if part == "" { continue }
        if up, ok := profileNameAcronyms[part]; ok { out = append(out, up); continue }
        r := []rune(part)
        out = append(out, string(unicode.ToUpper(r[0]))+string(r[1:]))
    }
    if len(out) == 0 { return slug }
    return strings.Join(out, " ")
}

// profileNameAcronyms are the slug segments that are initialisms, not words.
// "ui_ux" is the only built-in that needs it; the rest are for custom slugs.
var profileNameAcronyms = map[string]string{
    "ui": "UI", "ux": "UX", "api": "API", "ai": "AI", "ml": "ML", "sql": "SQL", "qa": "QA",
}

func (p *ProfileService) List(ctx context.Context) ([]ProfileSummary, error) {
    _ = ctx
    p.s.mu.RLock()
    defer p.s.mu.RUnlock()
    return p.listLocked()
}

func (p *ProfileService) listLocked() ([]ProfileSummary, error) {
    stats, _, err := AggregatePicks(filepath.Join(p.s.paths.StateDir, "pick", "history.jsonl"))
    if err != nil { return nil, err }
    builtins := make([]string, 0, len(pick.Profiles))
    for slug := range pick.Profiles { builtins = append(builtins, slug) }
    sort.Strings(builtins)
    out := make([]ProfileSummary, 0, len(builtins))
    for _, slug := range builtins {
        ep := pick.Profiles[slug]
        out = append(out, ProfileSummary{Slug: slug, Name: profileDisplayName(slug), Builtin: true, CoreShare: int(ep.Tier1Share.IntPart()), Tier1Weights: dtoWeights(ep.Tier1Weights), Tier2Weights: dtoWeights(ep.Tier2Weights), Picks: stats[slug].Picks, LastUsed: stats[slug].LastUsed})
    }
    customs, err := p.s.cfg.LoadProfiles(pick.CategoryNames)
    if err != nil { return nil, err }
    slugs := make([]string, 0, len(customs))
    for slug := range customs { if _, ok := pick.Profiles[slug]; !ok { slugs = append(slugs, slug) } }
    sort.Strings(slugs)
    for _, slug := range slugs {
        cp := customs[slug]
        out = append(out, ProfileSummary{Slug: slug, Name: profileDisplayName(slug), CoreShare: cp.CoreShare, Tier1Weights: cloneWeights(cp.Tier1), Tier2Weights: cloneWeights(cp.Tier2), Picks: stats[slug].Picks, LastUsed: stats[slug].LastUsed})
    }
    return out, nil
}

func cloneWeights(in map[string]int) map[string]int { out := make(map[string]int, len(in)); for k,v := range in { out[k]=v }; return out }

func (p *ProfileService) Get(ctx context.Context, slug string) (ProfileDetail, error) {
    list, err := p.List(ctx); if err != nil { return ProfileDetail{}, err }
    for _, d := range list { if d.Slug == slug { return d, nil } }
    return ProfileDetail{}, fmt.Errorf("%w: profile %q not found", errNotFound, slug)
}

var profileSlugRe = regexp.MustCompile(`^[a-z0-9_]+$`)

func (p *ProfileService) Save(ctx context.Context, d ProfileDetail) error {
    _ = ctx
    if !profileSlugRe.MatchString(d.Slug) { return fmt.Errorf("%w: profile slug %q must match [a-z0-9_]+", errValidation, d.Slug) }
    if _, ok := pick.Profiles[d.Slug]; ok { return fmt.Errorf("%w: profile %q is built-in and read-only", errBuiltinReadonly, d.Slug) }
    for slug, bp := range pick.Profiles { if d.Name == bp.Name && d.Slug != slug { return fmt.Errorf("%w: profile name %q conflicts with built-in profile %q", errConflict, d.Name, slug) } }
    if d.CoreShare < 10 || d.CoreShare > 90 || d.CoreShare%5 != 0 { return fmt.Errorf("%w: core_share %d must be between 10 and 90 in steps of 5", errValidation, d.CoreShare) }
    ep, err := engineProfile(d); if err != nil { return err }
    // engineProfile uses normalized shares; pick profiles use percentage shares.
    ep.Tier1Share = ep.Tier1Share.Mul(decimalHundred)
    ep.Tier2Share = ep.Tier2Share.Mul(decimalHundred)
    if err := pick.ValidateProfile(ep); err != nil { return fmt.Errorf("%w: %v", errValidation, err) }
    pt := config.ProfileTOML{CoreShare:d.CoreShare, Tier1: cloneWeights(d.Tier1Weights), Tier2: cloneWeights(d.Tier2Weights)}
    p.s.mu.Lock()
    prev, _ := p.s.cfg.LoadProfiles(pick.CategoryNames)
    if err := p.s.cfg.SetProfile(d.Slug, pt, pick.CategoryNames); err != nil { p.s.mu.Unlock(); return fmt.Errorf("%w: %v", errValidation, err) }
    data, err := p.s.cfg.MarshalTOML()
    if err != nil {
        restoreProfile(p.s.cfg, d.Slug, prev)
        p.s.mu.Unlock()
        return err
    }
    if err := config.AtomicWriteFile(p.s.paths.UserConfigFile, data); err != nil {
        restoreProfile(p.s.cfg, d.Slug, prev)
        p.s.mu.Unlock()
        return err
    }
    p.s.mu.Unlock()
    p.s.emit(EventConfigChanged, map[string]string{"section":"profiles"})
    return nil
}

func restoreProfile(cfg *config.Config, slug string, prev config.ProfilesTOML) {
    if old, ok := prev[slug]; ok {
        _ = cfg.SetProfile(slug, old, pick.CategoryNames)
    } else {
        cfg.DeleteProfile(slug)
    }
}

var decimalHundred = decimal.NewFromInt(100)

func (p *ProfileService) Duplicate(ctx context.Context, slug string) (ProfileDetail, error) {
    src, err := p.Get(ctx, slug); if err != nil { return ProfileDetail{}, err }
    existing, err := p.List(ctx); if err != nil { return ProfileDetail{}, err }
    used := map[string]bool{}; for _, d := range existing { used[d.Slug]=true }
    base := slug + "_copy"; candidate := base; for i:=2; used[candidate]; i++ { candidate = fmt.Sprintf("%s_%d", base, i) }
    src.Slug, src.Name, src.Builtin, src.Picks, src.LastUsed = candidate, candidate, false, 0, ""
    if err := p.Save(ctx, src); err != nil { return ProfileDetail{}, err }
    return src, nil
}

func (p *ProfileService) Delete(ctx context.Context, slug string) error {
    _ = ctx
    p.s.mu.Lock()
    customs, err := p.s.cfg.LoadProfiles(pick.CategoryNames)
    if err != nil { p.s.mu.Unlock(); return err }
    if _, ok := pick.Profiles[slug]; ok { p.s.mu.Unlock(); return fmt.Errorf("%w: profile %q is built-in and read-only", errBuiltinReadonly, slug) }
    old, ok := customs[slug]
    if !ok { p.s.mu.Unlock(); return fmt.Errorf("%w: profile %q not found", errNotFound, slug) }
    p.s.cfg.DeleteProfile(slug)
    data, err := p.s.cfg.MarshalTOML()
    if err != nil {
        _ = p.s.cfg.SetProfile(slug, old, pick.CategoryNames)
        p.s.mu.Unlock()
        return err
    }
    if err := config.AtomicWriteFile(p.s.paths.UserConfigFile, data); err != nil {
        _ = p.s.cfg.SetProfile(slug, old, pick.CategoryNames)
        p.s.mu.Unlock()
        return err
    }
    p.s.mu.Unlock()
    p.s.emit(EventConfigChanged, map[string]string{"section":"profiles"})
    return nil
}

