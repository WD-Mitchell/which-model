package service

import (
    "context"
    "errors"
    "testing"
)

func TestProfileListCanon(t *testing.T) {
    svc, _ := newTestServices(t)
    got, err := svc.Profiles().List(context.Background())
    if err != nil { t.Fatal(err) }
    if len(got) < 11 || got[0].Slug != "balanced_implementation" || got[10].Slug != "ui_ux" { t.Fatalf("unexpected built-in order/count: %d %#v", len(got), got) }
    for _, d := range got[:11] { if !d.Builtin { t.Errorf("%s not builtin", d.Slug) } }
}

func TestProfileSaveDuplicateDelete(t *testing.T) {
    svc, rec := newTestServices(t)
    p := svc.Profiles()
    d := ProfileDetail{Slug:"my_profile", Name:"my_profile", CoreShare:65,
        Tier1Weights: map[string]int{"intelligence":3,"cost":3,"speed":3},
        Tier2Weights: map[string]int{"research":3}}
    if err := p.Save(context.Background(), d); err != nil { t.Fatal(err) }
    got, err := p.Get(context.Background(), "my_profile"); if err != nil { t.Fatal(err) }
    if got.CoreShare != 65 || got.Tier1Weights["speed"] != 3 { t.Fatalf("saved profile mismatch: %#v", got) }
    dup, err := p.Duplicate(context.Background(), "my_profile"); if err != nil { t.Fatal(err) }
    if dup.Slug != "my_profile_copy" || dup.Builtin || dup.Picks != 0 { t.Fatalf("duplicate mismatch: %#v", dup) }
    if err := p.Delete(context.Background(), "my_profile"); err != nil { t.Fatal(err) }
    if _, err := p.Get(context.Background(), "my_profile"); !errors.Is(err, errNotFound) { t.Fatalf("Get deleted err=%v", err) }
    if len(rec.Events()) != 3 { t.Fatalf("events=%d want 3", len(rec.Events())) }
}

func TestProfileValidationAndScale(t *testing.T) {
    svc, _ := newTestServices(t); p := svc.Profiles()
    if err := p.Save(context.Background(), ProfileDetail{Slug:"planning", Name:"planning", CoreShare:60}); !errors.Is(err, errBuiltinReadonly) { t.Fatalf("builtin err=%v", err) }
    if err := p.Save(context.Background(), ProfileDetail{Slug:"bad-slug!", CoreShare:60}); !errors.Is(err, errValidation) { t.Fatalf("slug err=%v", err) }
    scale := p.ComplexityScale(); scale[0] = "x"; if p.ComplexityScale()[0] != "simple_action_execution" { t.Fatal("scale not copied") }
}

// The engine's profile names ARE its snake_case slugs, so every surface that
// named a profile — popover, settings list, tray menu, menu-bar title — showed
// "balanced_implementation". The DTO carries a display name instead; the slug
// stays the id.
func TestProfileDisplayName(t *testing.T) {
    cases := map[string]string{
        "balanced_implementation": "Balanced Implementation",
        "research":                "Research",
        "simple_action_execution": "Simple Action Execution",
        // Initialisms read as typos when title-cased.
        "ui_ux":     "UI UX",
        "my_api_v2": "My API V2",
        // Degenerate slugs must not produce an empty name.
        "":     "",
        "__":   "__",
        "x":    "X",
    }
    for slug, want := range cases {
        if got := profileDisplayName(slug); got != want {
            t.Errorf("profileDisplayName(%q) = %q, want %q", slug, got, want)
        }
    }
}

// The list must carry the display name for built-ins AND customs — a custom
// profile named after its slug would otherwise be the one row still shouting
// snake_case at the user.
func TestProfileListUsesDisplayNames(t *testing.T) {
    svc, _ := newTestServices(t)
    p := svc.Profiles()
    d := ProfileDetail{Slug:"my_custom_profile", Name:"my_custom_profile", CoreShare:65,
        Tier1Weights: map[string]int{"intelligence":3,"cost":3,"speed":3},
        Tier2Weights: map[string]int{"research":3}}
    if err := p.Save(context.Background(), d); err != nil { t.Fatal(err) }
    list, err := p.List(context.Background()); if err != nil { t.Fatal(err) }
    seen := map[string]string{}
    for _, s := range list { seen[s.Slug] = s.Name }
    if seen["balanced_implementation"] != "Balanced Implementation" {
        t.Errorf("built-in name = %q, want %q", seen["balanced_implementation"], "Balanced Implementation")
    }
    if seen["my_custom_profile"] != "My Custom Profile" {
        t.Errorf("custom name = %q, want %q", seen["my_custom_profile"], "My Custom Profile")
    }
}
