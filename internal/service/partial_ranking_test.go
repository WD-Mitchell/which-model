package service

import (
	"context"
	"strings"
	"testing"
)

func TestIncompleteRecommendationSettingAppliesImmediately(t *testing.T) {
	// Beta has real intelligence/cost scores but no speed, like Astra's AA row.
	scores := strings.Replace(fixtureScoresCSV, "beta,high,85,0,70,75,", "beta,high,85,0,70,,", 1)
	svc, _ := fixtureServices(t, WithScoresCSV(scores))
	ctx := context.Background()
	settings, err := svc.Settings().Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if settings.AllowIncompleteRecommendations {
		t.Fatal("new installs must default to complete scores")
	}
	for _, allow := range []bool{false, true, false} {
		settings.AllowIncompleteRecommendations = allow
		if err := svc.Settings().Set(ctx, settings); err != nil {
			t.Fatal(err)
		}
		got, err := svc.Settings().Get(ctx)
		if err != nil || got.AllowIncompleteRecommendations != allow {
			t.Fatalf("saved setting = %+v, %v", got, err)
		}
		result, err := svc.Rank(ctx, RankRequest{ProfileSlug: "test_profile", Holds: 5})
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, candidate := range result.Candidates {
			if candidate.ModelName == "beta" {
				found = true
				if candidate.Intelligence == nil || candidate.Cost == nil || candidate.Speed != nil {
					t.Fatalf("partial measurements were lost or fabricated: %+v", candidate)
				}
			}
		}
		if found != allow {
			t.Fatalf("allow=%v, beta present=%v", allow, found)
		}
		models, err := svc.Catalog().Models(ctx)
		if err != nil {
			t.Fatal(err)
		}
		for _, model := range models {
			if model.ModelName == "beta" && (model.Intelligence == nil || model.Speed != nil) {
				t.Fatalf("recommendation setting changed catalog data: %+v", model)
			}
		}
	}
}
