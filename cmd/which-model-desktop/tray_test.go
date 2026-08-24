package main

import (
	"context"
	"errors"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/service"
)

// trayLabel names the top pick behind a thin space — the model plus its
// reasoning level, which is what the user would actually launch. It must render
// NOTHING when ranking is unavailable: the old em-dash placeholder looked like
// a broken menu-bar item (S02 SPEC §2.4b).
func TestTrayLabel(t *testing.T) {
	orig := rankTopPick
	t.Cleanup(func() { rankTopPick = orig })

	cases := []struct {
		name string
		pick service.RankedModel
		err  error
		want string
	}{
		{
			name: "model and reasoning",
			pick: service.RankedModel{ModelName: "Claude Opus 5", Reasoning: "high", Score: 87.4},
			want: "Claude Opus 5 (high)",
		},
		{
			// Reasoning IS the identity here: two picks differing only by effort
			// must not render the same label.
			name: "same model, other effort",
			pick: service.RankedModel{ModelName: "Claude Opus 5", Reasoning: "low"},
			want: "Claude Opus 5 (low)",
		},
		{
			name: "no reasoning leaves the name bare",
			pick: service.RankedModel{ModelName: "GPT-5.6 Sol"},
			want: "GPT-5.6 Sol",
		},
		{
			// A route with no model name has nothing to show; an empty bracket
			// would read worse than no label at all.
			name: "nameless pick is blank",
			pick: service.RankedModel{Reasoning: "high"},
			want: "",
		},
		{name: "no candidate is blank", err: errNoTopPick, want: ""},
		{name: "rank error is blank", err: errors.New("catalog empty"), want: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rankTopPick = func(context.Context, *service.Services, string) (service.RankedModel, error) {
				return tc.pick, tc.err
			}
			_, got, _ := trayTitleLines(context.Background(), nil, trayLabelProfile)
			if got != tc.want {
				t.Fatalf("model line = %q, want %q", got, tc.want)
			}
		})
	}
}

// The default rankTopPick must not panic or dereference a nil service — the
// tray is built before the service is guaranteed usable on a cold install.
func TestRankTopPickNilService(t *testing.T) {
	if _, err := rankTopPick(context.Background(), nil, trayLabelProfile); !errors.Is(err, errNoTopPick) {
		t.Fatalf("rankTopPick(nil) error = %v, want errNoTopPick", err)
	}
}

// The menu-bar title is two lines: the profile ranked for, then the model it
// picked. Neither line carries spacing of its own: the composed menu-bar item
// owns its gaps (traytitle_darwin.m), and the single-line platforms add the
// thin space themselves (traytitle_other.go).
func TestTrayTitleLines(t *testing.T) {
	origRank, origName := rankTopPick, trayProfileName
	t.Cleanup(func() { rankTopPick, trayProfileName = origRank, origName })

	var askedFor string
	rankTopPick = func(_ context.Context, _ *service.Services, slug string) (service.RankedModel, error) {
		askedFor = slug
		return service.RankedModel{ModelName: "Claude Opus 5", Reasoning: "high", Provider: "claude"}, nil
	}
	trayProfileName = func(_ context.Context, _ *service.Services, slug string) string {
		return slug
	}

	top, bottom, provider := trayTitleLines(context.Background(), nil, "planning")
	if want := "planning"; top != want {
		t.Fatalf("top = %q, want %q", top, want)
	}
	if want := "Claude Opus 5 (high)"; bottom != want {
		t.Fatalf("bottom = %q, want %q", bottom, want)
	}
	// The icon is the pick's provider, so it comes from the same rank call.
	if provider != "claude" {
		t.Fatalf("provider = %q, want the ranked pick's provider", provider)
	}
	// The profile line and the model line must describe the SAME request, or
	// the menu bar names a profile that did not produce the model under it.
	if askedFor != "planning" {
		t.Fatalf("ranked for %q, want the profile the title names", askedFor)
	}
}

// An unrankable state blanks BOTH lines. A profile name stranded over nothing
// reads as a broken menu-bar item — the same reason the em-dash placeholder
// was dropped (S02 SPEC §2.4b).
func TestTrayTitleLinesBlankTogether(t *testing.T) {
	origRank, origName := rankTopPick, trayProfileName
	t.Cleanup(func() { rankTopPick, trayProfileName = origRank, origName })

	rankTopPick = func(context.Context, *service.Services, string) (service.RankedModel, error) {
		return service.RankedModel{}, errNoTopPick
	}
	trayProfileName = func(context.Context, *service.Services, string) string { return "planning" }

	top, bottom, provider := trayTitleLines(context.Background(), nil, "planning")
	if top != "" || bottom != "" || provider != "" {
		t.Fatalf("trayTitleLines() = %q / %q / %q, want all blank", top, bottom, provider)
	}
}

// The profile-name seam must degrade to the slug rather than to an empty
// string: an unnamed top line would leave the model line looking mislabelled,
// and the built-ins' display names ARE their slugs today.
func TestTrayProfileNameFallsBackToSlug(t *testing.T) {
	if got := trayProfileName(context.Background(), nil, "planning"); got != "planning" {
		t.Fatalf("trayProfileName(nil svc) = %q, want the slug", got)
	}
	if got := trayProfileName(context.Background(), nil, ""); got != trayLabelProfile {
		t.Fatalf("trayProfileName(no slug) = %q, want %q", got, trayLabelProfile)
	}
}

// The gap must stay a thin space where it is still used — the one-line tray
// label on Windows and Linux, whose backends butt the text against the icon
// just as AppKit did. A normal space pushes it visibly off; none crowds it.
func TestTrayLabelGapIsThinSpace(t *testing.T) {
	if trayLabelGap != "\u2009" {
		t.Fatalf("trayLabelGap = %q, want U+2009 THIN SPACE", trayLabelGap)
	}
}

// trayLabelHolds must be a value service.effectiveHolds accepts, or every
// refresh silently blanks the label (internal/service/pick.go:224).
func TestTrayLabelHoldsIsLegal(t *testing.T) {
	switch trayLabelHolds {
	case 3, 5, 10:
	default:
		t.Fatalf("trayLabelHolds = %d, must be 3, 5 or 10", trayLabelHolds)
	}
}

// The popover owns the menu bar once it has pushed a pick. The host's own
// ranking cannot see the popover's active profile or its ephemeral weight
// overrides, so a later catalog or config event must not recompute the title
// from a different profile — the popover re-pushes when its own ranking moves.
func TestPopoverPickWins(t *testing.T) {
	trayPickMu.Lock()
	origSet, origTop, origBottom, origProvider :=
		trayPickFromPopover, trayPickTop, trayPickBottom, trayPickProviderID
	trayPickFromPopover, trayPickTop, trayPickBottom, trayPickProviderID = false, "", "", ""
	trayPickMu.Unlock()
	t.Cleanup(func() {
		trayPickMu.Lock()
		trayPickFromPopover, trayPickTop, trayPickBottom, trayPickProviderID =
			origSet, origTop, origBottom, origProvider
		trayPickMu.Unlock()
	})

	if _, _, _, ok := popoverPick(); ok {
		t.Fatal("popoverPick() reports a pick before the popover pushed one")
	}

	setTrayPickFromUI("Planning", "Claude Opus 5", "high", "claude")

	top, bottom, provider, ok := popoverPick()
	if !ok {
		t.Fatal("popoverPick() = !ok after a push")
	}
	// The model line is composed the same way the host composes its own, so
	// the two sources cannot disagree on spelling.
	if top != "Planning" || bottom != "Claude Opus 5 (high)" || provider != "claude" {
		t.Fatalf("popoverPick() = %q / %q / %q", top, bottom, provider)
	}

	// A pick with no reasoning must not render an empty bracket.
	setTrayPickFromUI("Research", "GPT-5.6 Sol", "", "codex")
	if _, bottom, _, _ := popoverPick(); bottom != "GPT-5.6 Sol" {
		t.Fatalf("bottom = %q, want the bare model name", bottom)
	}
}
