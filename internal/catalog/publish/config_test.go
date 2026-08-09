package publish

import (
	"reflect"
	"testing"
)

func TestPublishConfigZeroValue(t *testing.T) {
	var pc PublishConfig
	if pc.Enabled || pc.AutoMerge || pc.RunTests {
		t.Errorf("bool fields must be false in the zero value: %+v", pc)
	}
	for name, v := range map[string]string{
		"Schedule":      pc.Schedule,
		"Timezone":      pc.Timezone,
		"Mode":          pc.Mode,
		"MergeMethod":   pc.MergeMethod,
		"CommitMessage": pc.CommitMessage,
		"PRTitle":       pc.PRTitle,
		"RawCSVPath":    pc.RawCSVPath,
		"ScoresCSVPath": pc.ScoresCSVPath,
	} {
		if v != "" {
			t.Errorf("%s = %q, want empty in the zero value", name, v)
		}
	}
	if pc.Branches != nil || pc.PRLabels != nil {
		t.Errorf("slice fields must be nil in the zero value: %+v", pc)
	}
}

func TestPublishConfigRoundTrip(t *testing.T) {
	pc := PublishConfig{
		Enabled:       true,
		Schedule:      "15 8 * * MON",
		Timezone:      "America/New_York",
		Branches:      []string{"release", "canary"},
		Mode:          "direct-push",
		AutoMerge:     false,
		MergeMethod:   "rebase",
		CommitMessage: "chore: refresh",
		PRTitle:       "chore: refresh PR",
		PRLabels:      []string{"data"},
		RunTests:      false,
		RawCSVPath:    "raw.csv",
		ScoresCSVPath: "scores.csv",
	}
	if pc.Enabled != true || pc.Schedule != "15 8 * * MON" || pc.Timezone != "America/New_York" ||
		!reflect.DeepEqual(pc.Branches, []string{"release", "canary"}) || pc.Mode != "direct-push" ||
		pc.AutoMerge != false || pc.MergeMethod != "rebase" || pc.CommitMessage != "chore: refresh" ||
		pc.PRTitle != "chore: refresh PR" || !reflect.DeepEqual(pc.PRLabels, []string{"data"}) ||
		pc.RunTests != false || pc.RawCSVPath != "raw.csv" || pc.ScoresCSVPath != "scores.csv" {
		t.Errorf("struct literal fields did not round-trip: %+v", pc)
	}
}

func TestPublishDefaults(t *testing.T) {
	if DefaultSchedule != "0 6 * * *" {
		t.Errorf("DefaultSchedule = %q", DefaultSchedule)
	}
	if DefaultTimezone != "Europe/London" {
		t.Errorf("DefaultTimezone = %q", DefaultTimezone)
	}
	if DefaultMode != "pull-request" {
		t.Errorf("DefaultMode = %q", DefaultMode)
	}
	if DefaultMergeMethod != "squash" {
		t.Errorf("DefaultMergeMethod = %q", DefaultMergeMethod)
	}
	if DefaultCommitMessage != "chore(data): refresh available model scores" {
		t.Errorf("DefaultCommitMessage = %q", DefaultCommitMessage)
	}
	if DefaultPRTitle != "chore(data): refresh available model scores" {
		t.Errorf("DefaultPRTitle = %q", DefaultPRTitle)
	}
	if !reflect.DeepEqual(DefaultBranches, []string{"main"}) {
		t.Errorf("DefaultBranches = %v", DefaultBranches)
	}
	if !reflect.DeepEqual(DefaultPRLabels, []string{"data", "automated"}) {
		t.Errorf("DefaultPRLabels = %v", DefaultPRLabels)
	}
	pc := NewDefaults()
	if !pc.Enabled || !pc.AutoMerge || !pc.RunTests {
		t.Errorf("NewDefaults() bool defaults: %+v", pc)
	}
	if pc.Schedule != DefaultSchedule || pc.Timezone != DefaultTimezone ||
		pc.Mode != DefaultMode || pc.MergeMethod != DefaultMergeMethod ||
		pc.CommitMessage != DefaultCommitMessage || pc.PRTitle != DefaultPRTitle {
		t.Errorf("NewDefaults() string defaults: %+v", pc)
	}
	if !reflect.DeepEqual(pc.Branches, DefaultBranches) {
		t.Errorf("NewDefaults() Branches = %v", pc.Branches)
	}
	if !reflect.DeepEqual(pc.PRLabels, DefaultPRLabels) {
		t.Errorf("NewDefaults() PRLabels = %v", pc.PRLabels)
	}
}

func TestNewDefaultsNoSliceAliasing(t *testing.T) {
	pc := NewDefaults()
	pc.Branches[0] = "mutated"
	pc.PRLabels[0] = "mutated"
	if DefaultBranches[0] != "main" {
		t.Errorf("DefaultBranches aliased: %v", DefaultBranches)
	}
	if DefaultPRLabels[0] != "data" {
		t.Errorf("DefaultPRLabels aliased: %v", DefaultPRLabels)
	}
}
