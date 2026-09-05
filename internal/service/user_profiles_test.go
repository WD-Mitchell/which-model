package service

import (
	"context"
	"github.com/WD-Mitchell/which-model/internal/config"
	"os"
	"reflect"
	"testing"
)

func TestUserProfilesHaveValidUseCases(t *testing.T) {
	svc, _ := newTestServices(t)
	profiles := svc.Profiles().UserProfiles()
	if len(profiles) != 3 {
		t.Fatalf("profiles = %#v", profiles)
	}
	for _, p := range profiles {
		seen := map[string]bool{}
		for _, slug := range p.UseCaseSlugs {
			if seen[slug] {
				t.Fatalf("duplicate %s", slug)
			}
			seen[slug] = true
			if _, err := svc.Profiles().Get(context.Background(), slug); err != nil {
				t.Fatal(err)
			}
		}
		if !seen[p.DefaultUseCase] {
			t.Fatalf("missing default in %s", p.Slug)
		}
	}
	profiles[0].UseCaseSlugs[0] = "broken"
	if svc.Profiles().UserProfiles()[0].UseCaseSlugs[0] == "broken" {
		t.Fatal("mutable shared slice")
	}
}

func TestUserProfileSelectionPersistsAndRejectsUnknown(t *testing.T) {
	svc, rec := newTestServices(t)
	ctx := context.Background()
	g, err := svc.Settings().Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if g.UserProfile != "software_engineering" {
		t.Fatalf("legacy default: %q", g.UserProfile)
	}
	g.UserProfile = "marketing"
	if err := svc.Settings().Set(ctx, g); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadFile(svc.paths.UserConfigFile)
	if err != nil {
		t.Fatal(err)
	}
	gui, err := cfg.LoadGUI()
	if err != nil || gui.UserProfile != "marketing" {
		t.Fatalf("persisted: %#v %v", gui, err)
	}
	before, _ := os.ReadFile(svc.paths.UserConfigFile)
	g.UserProfile = "missing"
	if err := svc.Settings().Set(ctx, g); err == nil {
		t.Fatal("unknown profile accepted")
	}
	after, _ := os.ReadFile(svc.paths.UserConfigFile)
	if !reflect.DeepEqual(before, after) || len(rec.Events()) != 1 {
		t.Fatal("failed save changed state")
	}
	g, _ = svc.Settings().Get(ctx)
	if g.UserProfile != "marketing" {
		t.Fatal("failed save changed selection")
	}
}

func TestUseCaseWithCustomBenchmarkGroup(t *testing.T) {
	svc, _ := newTestServices(t, WithConfigTOML("[groups.my_group]\nbenchmarks = [\"GPQA Diamond\"]\n"))
	ctx := context.Background()
	d := ProfileDetail{Slug: "custom_use_case", CoreShare: 60, Tier1Weights: map[string]int{"intelligence": 3, "cost": 3, "speed": 3}, Tier2Weights: map[string]int{"my_group": 5}}
	if err := svc.Profiles().Save(ctx, d); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Rank(ctx, RankRequest{ProfileSlug: d.Slug, Holds: 3}); err != nil {
		t.Fatal(err)
	}
	d.Tier2Weights = map[string]int{"unknown_group": 5}
	if err := svc.Profiles().Save(ctx, d); err == nil {
		t.Fatal("unknown group accepted")
	}
}

func TestExistingCustomUseCaseSurvivesNewPresetCollision(t *testing.T) {
	svc, _ := newTestServices(t, WithConfigTOML(`[profiles.content_drafting]
core_share = 80
[profiles.content_drafting.tier1]
intelligence = 5
cost = 1
speed = 1
[profiles.content_drafting.tier2]
research = 5
`))
	ctx := context.Background()
	d, err := svc.Profiles().Get(ctx, "content_drafting")
	if err != nil || d.Builtin || d.CoreShare != 80 {
		t.Fatalf("custom was shadowed: %#v, %v", d, err)
	}
	ep, err := svc.profileBySlug("content_drafting")
	if err != nil || ep.Tier1Share.IntPart() != 80 {
		t.Fatalf("rank resolves wrong weights: %#v %v", ep, err)
	}
	d.CoreShare = 75
	if err := svc.Profiles().Save(ctx, d); err != nil {
		t.Fatal(err)
	}
	if err := svc.Profiles().Delete(ctx, d.Slug); err != nil {
		t.Fatal(err)
	}
	d, err = svc.Profiles().Get(ctx, "content_drafting")
	if err != nil || !d.Builtin || d.CoreShare != 65 {
		t.Fatalf("preset not restored: %#v, %v", d, err)
	}
}
