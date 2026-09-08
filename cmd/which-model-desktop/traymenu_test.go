package main

import (
	"testing"

	"github.com/WD-Mitchell/which-model/internal/service"
)

func summaries(pairs ...[2]string) []service.ProfileSummary {
	out := make([]service.ProfileSummary, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, service.ProfileSummary{Slug: p[0], Name: p[1]})
	}
	return out
}

func slugs(profiles []service.ProfileSummary) []string {
	out := make([]string, 0, len(profiles))
	for _, p := range profiles {
		out = append(out, p.Slug)
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Profiles are listed in complexity-scale order; anything off the scale (custom
// profiles) follows, alphabetically by display name.
func TestOrderTrayProfiles(t *testing.T) {
	scale := []string{"simple_action_execution", "simple_implementation", "balanced_implementation", "research", "planning"}
	// Deliberately alphabetical, i.e. the order ProfileService.List returns.
	in := summaries(
		[2]string{"balanced_implementation", "Balanced implementation"},
		[2]string{"my_zebra", "Zebra"},
		[2]string{"my_alpha", "Alpha"},
		[2]string{"planning", "Planning"},
		[2]string{"research", "Research"},
		[2]string{"simple_action_execution", "Simple action execution"},
		[2]string{"simple_implementation", "Simple implementation"},
	)

	want := []string{
		"simple_action_execution", "simple_implementation", "balanced_implementation",
		"research", "planning",
		"my_alpha", "my_zebra",
	}
	if got := slugs(orderTrayProfiles(in, scale)); !equal(got, want) {
		t.Fatalf("orderTrayProfiles() = %v, want %v", got, want)
	}
}

// With no scale (the service call failed) the menu must still be deterministic
// rather than randomly ordered, and must not drop anyone.
func TestOrderTrayProfilesNoScale(t *testing.T) {
	in := summaries([2]string{"b", "Beta"}, [2]string{"a", "Alpha"})
	if got := slugs(orderTrayProfiles(in, nil)); !equal(got, []string{"a", "b"}) {
		t.Fatalf("orderTrayProfiles(no scale) = %v, want [a b]", got)
	}
	if got := orderTrayProfiles(nil, nil); len(got) != 0 {
		t.Fatalf("orderTrayProfiles(nil) = %v, want empty", got)
	}
}

// Ties on display name fall back to slug so a rename collision cannot reorder
// the menu between rebuilds.
func TestOrderTrayProfilesNameTie(t *testing.T) {
	in := summaries([2]string{"zzz", "Same"}, [2]string{"aaa", "Same"})
	if got := slugs(orderTrayProfiles(in, nil)); !equal(got, []string{"aaa", "zzz"}) {
		t.Fatalf("orderTrayProfiles(tie) = %v, want [aaa zzz]", got)
	}
}

// The initially checked profile mirrors the popover's own seed
// (PopoverApp.tsx: `scale[1] ?? scale[0]`) so the two never disagree on launch.
func TestDefaultTraySelection(t *testing.T) {
	scale := []string{"simple_action_execution", "simple_implementation", "balanced_implementation"}
	profiles := summaries(
		[2]string{"simple_action_execution", "Simple action execution"},
		[2]string{"simple_implementation", "Simple implementation"},
		[2]string{"balanced_implementation", "Balanced implementation"},
	)
	if got := defaultTraySelection(profiles, scale); got != "simple_implementation" {
		t.Fatalf("defaultTraySelection() = %q, want simple_implementation", got)
	}
}

func TestDefaultTraySelectionFallbacks(t *testing.T) {
	custom := summaries([2]string{"custom", "Custom"})

	// Scale slugs the profile list does not contain (stale config) fall through
	// to the first listed profile.
	if got := defaultTraySelection(custom, []string{"gone_a", "gone_b"}); got != "custom" {
		t.Fatalf("defaultTraySelection(stale scale) = %q, want custom", got)
	}
	// A one-entry scale uses scale[0].
	one := summaries([2]string{"only", "Only"})
	if got := defaultTraySelection(one, []string{"only"}); got != "only" {
		t.Fatalf("defaultTraySelection(1-entry scale) = %q, want only", got)
	}
	// No profiles at all => no selection, and no panic.
	if got := defaultTraySelection(nil, []string{"a", "b"}); got != "" {
		t.Fatalf("defaultTraySelection(no profiles) = %q, want empty", got)
	}
}

// The signature decides when the native menu is rebuilt: renames and
// membership changes must move it, a pure reorder of identical content must
// not be mistaken for equality across a different order.
func TestTrayMenuSignature(t *testing.T) {
	base := summaries([2]string{"a", "Alpha"}, [2]string{"b", "Beta"})

	if trayMenuSignature(base, true) != trayMenuSignature(base, true) {
		t.Fatal("signature is not stable for identical input")
	}
	if trayMenuSignature(base, true) == trayMenuSignature(base, false) {
		t.Fatal("signature must distinguish the pre-selection first build")
	}
	renamed := summaries([2]string{"a", "Alpha 2"}, [2]string{"b", "Beta"})
	if trayMenuSignature(base, true) == trayMenuSignature(renamed, true) {
		t.Fatal("signature must change when a profile is renamed")
	}
	added := summaries([2]string{"a", "Alpha"}, [2]string{"b", "Beta"}, [2]string{"c", "Gamma"})
	if trayMenuSignature(base, true) == trayMenuSignature(added, true) {
		t.Fatal("signature must change when a profile is added")
	}
	reordered := summaries([2]string{"b", "Beta"}, [2]string{"a", "Alpha"})
	if trayMenuSignature(base, true) == trayMenuSignature(reordered, true) {
		t.Fatal("signature must change when the order changes")
	}
	// Slug/name boundaries must not be ambiguous: {"ab",""} and {"a","b"} are
	// different menus.
	if trayMenuSignature(summaries([2]string{"ab", ""}), false) ==
		trayMenuSignature(summaries([2]string{"a", "b"}), false) {
		t.Fatal("signature must not confuse slug/name boundaries")
	}
}

// Refresh must be inert before Start (the menu cannot be installed until the
// app is running) and must survive a nil service without panicking.
func TestTrayMenuRefreshBeforeStart(t *testing.T) {
	m := newTrayMenu(nil, nil, nil, nil)
	m.Refresh()
	if got := m.Selected(); got != "" {
		t.Fatalf("Selected() = %q before Start, want empty", got)
	}
}

func TestProfileMenuUsesOnlyOrderedDefaults(t *testing.T) {
	all := []service.ProfileSummary{{Slug: "review"}, {Slug: "content_editing"}, {Slug: "content_drafting"}}
	profile := service.UserProfile{UseCaseSlugs: []string{"content_drafting", "content_editing"}}
	got := profileMenuEntries(all, profile)
	if len(got) != 2 || got[0].Slug != "content_drafting" || got[1].Slug != "content_editing" {
		t.Fatalf("menu = %#v", got)
	}
}
