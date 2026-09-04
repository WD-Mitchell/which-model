package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestProfileListCanon(t *testing.T) {
	svc, _ := newTestServices(t)
	got, err := svc.Profiles().List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) < 11 || got[0].Slug != "balanced_implementation" || got[10].Slug != "ui_ux" {
		t.Fatalf("unexpected built-in order/count: %d %#v", len(got), got)
	}
	for _, d := range got[:11] {
		if !d.Builtin {
			t.Errorf("%s not builtin", d.Slug)
		}
	}
}

func TestProfileSaveDuplicateDelete(t *testing.T) {
	svc, rec := newTestServices(t)
	p := svc.Profiles()
	d := ProfileDetail{Slug: "my_profile", Name: "my_profile", CoreShare: 65,
		Tier1Weights: map[string]int{"intelligence": 3, "cost": 3, "speed": 3},
		Tier2Weights: map[string]int{"research": 3}}
	if err := p.Save(context.Background(), d); err != nil {
		t.Fatal(err)
	}
	got, err := p.Get(context.Background(), "my_profile")
	if err != nil {
		t.Fatal(err)
	}
	if got.CoreShare != 65 || got.Tier1Weights["speed"] != 3 {
		t.Fatalf("saved profile mismatch: %#v", got)
	}
	dup, err := p.Duplicate(context.Background(), "my_profile")
	if err != nil {
		t.Fatal(err)
	}
	if dup.Slug != "my_profile_copy" || dup.Builtin || dup.Picks != 0 {
		t.Fatalf("duplicate mismatch: %#v", dup)
	}
	if err := p.Delete(context.Background(), "my_profile"); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Get(context.Background(), "my_profile"); !errors.Is(err, errNotFound) {
		t.Fatalf("Get deleted err=%v", err)
	}
	if len(rec.Events()) != 3 {
		t.Fatalf("events=%d want 3", len(rec.Events()))
	}
}

func TestProfileValidationAndScale(t *testing.T) {
	svc, _ := newTestServices(t)
	p := svc.Profiles()
	if err := p.Save(context.Background(), ProfileDetail{Slug: "planning", Name: "planning", CoreShare: 60}); !errors.Is(err, errBuiltinReadonly) {
		t.Fatalf("builtin err=%v", err)
	}
	if err := p.Save(context.Background(), ProfileDetail{Slug: "bad-slug!", CoreShare: 60}); !errors.Is(err, errValidation) {
		t.Fatalf("slug err=%v", err)
	}
	scale := p.ComplexityScale()
	scale[0] = "x"
	if p.ComplexityScale()[0] != "simple_action_execution" {
		t.Fatal("scale not copied")
	}
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
		"":   "",
		"__": "__",
		"x":  "X",
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
	d := ProfileDetail{Slug: "my_custom_profile", Name: "my_custom_profile", CoreShare: 65,
		Tier1Weights: map[string]int{"intelligence": 3, "cost": 3, "speed": 3},
		Tier2Weights: map[string]int{"research": 3}}
	if err := p.Save(context.Background(), d); err != nil {
		t.Fatal(err)
	}
	list, err := p.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]string{}
	for _, s := range list {
		seen[s.Slug] = s.Name
	}
	if seen["balanced_implementation"] != "Balanced Implementation" {
		t.Errorf("built-in name = %q, want %q", seen["balanced_implementation"], "Balanced Implementation")
	}
	if seen["my_custom_profile"] != "My Custom Profile" {
		t.Errorf("custom name = %q, want %q", seen["my_custom_profile"], "My Custom Profile")
	}
}

func TestProfileCreateAtomicCollision(t *testing.T) {
	svc, rec := newTestServices(t)
	p := svc.Profiles()
	ctx := context.Background()
	d := ProfileDetail{Slug: "new_profile", Name: "New", CoreShare: 65, Tier1Weights: map[string]int{"intelligence": 3, "cost": 3, "speed": 3}, Tier2Weights: map[string]int{}}
	result := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() { result <- p.Create(ctx, d) }()
	}
	successes, conflicts := 0, 0
	for i := 0; i < 2; i++ {
		err := <-result
		if err == nil {
			successes++
		} else if errors.Is(err, errConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if successes != 1 || conflicts != 1 || len(rec.Events()) != 1 {
		t.Fatalf("success=%d conflict=%d events=%d", successes, conflicts, len(rec.Events()))
	}
	d.CoreShare = 70
	if err := p.Create(ctx, d); !errors.Is(err, errConflict) {
		t.Fatalf("collision=%v", err)
	}
	got, err := p.Get(ctx, d.Slug)
	if err != nil || got.CoreShare != 65 {
		t.Fatalf("overwritten=%+v %v", got, err)
	}
	if err = p.Save(ctx, d); err != nil {
		t.Fatal(err)
	}
	got, err = p.Get(ctx, d.Slug)
	if err != nil || got.CoreShare != 70 {
		t.Fatalf("save=%+v %v", got, err)
	}
	d.Slug = "planning"
	if err = p.Create(ctx, d); !errors.Is(err, errConflict) {
		t.Fatalf("builtin create=%v", err)
	}
}

func TestProfileCustomGroupSaveAndRank(t *testing.T) {
	svc, _ := newTestServices(t, WithConfigTOML(providersFixture+"[groups.custom_group]\nbenchmarks = [\"SWE-Bench Verified\"]\n"))
	ctx := context.Background()
	d := ProfileDetail{Slug: "custom_profile", Name: "Custom", CoreShare: 65, Tier1Weights: map[string]int{"intelligence": 3, "cost": 3, "speed": 3}, Tier2Weights: map[string]int{"custom_group": 5}}
	if err := svc.Profiles().Save(ctx, d); err != nil {
		t.Fatal(err)
	}
	for _, req := range []RankRequest{{ProfileSlug: d.Slug}, {ProfileSlug: d.Slug, Overrides: &d}} {
		if result, err := svc.Rank(ctx, req); err != nil {
			t.Fatal(err)
		} else if len(result.Candidates) == 0 {
			t.Fatal("custom-group rank did not reach scoring")
		}
	}
	d.Tier2Weights = map[string]int{"missing_group": 5}
	if err := svc.Profiles().Save(ctx, d); !errors.Is(err, errValidation) {
		t.Fatalf("unknown group=%v", err)
	}
	if _, err := svc.Rank(ctx, RankRequest{Overrides: &d}); !errors.Is(err, errValidation) {
		t.Fatalf("unknown preview=%v", err)
	}
}

func TestProfileRequestErrorsPrecedeStoredProfileDecode(t *testing.T) {
	svc, _ := newTestServices(t)
	path := filepath.Join(t.TempDir(), "bad.toml")
	if err := os.WriteFile(path, []byte("[profiles.broken]\ncore_share = 'bad'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := svc.cfg.DecodeFile(path); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		detail ProfileDetail
		want   error
	}{
		{ProfileDetail{Slug: "planning"}, errBuiltinReadonly},
		{ProfileDetail{Slug: "custom", Name: "Custom", CoreShare: 0}, errValidation},
	} {
		if err := svc.Profiles().Save(context.Background(), tc.detail); !errors.Is(err, tc.want) {
			t.Fatalf("got %v, want %v", err, tc.want)
		}
	}
}
