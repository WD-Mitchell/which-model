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
