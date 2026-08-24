package main

import (
	"testing"
	"time"
)

// shouldHideOnBlur is the whole of the hide-on-focus-loss rule that can be
// tested off-AppKit: the show timestamp, the clock and the grace window. The
// blur that arrives inside the grace window is the status-item mouse-down
// tracking loop stealing key (see popover.go), and hiding on it is exactly the
// bug where left-clicking the menu-bar icon looked like a no-op.
func TestShouldHideOnBlur(t *testing.T) {
	shown := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	const grace = 500 * time.Millisecond

	cases := []struct {
		name string
		now  time.Time
		want bool
	}{
		{name: "same instant as show", now: shown, want: false},
		{name: "inside grace", now: shown.Add(499 * time.Millisecond), want: false},
		{name: "exactly at grace", now: shown.Add(grace), want: true},
		{name: "well past grace", now: shown.Add(5 * time.Second), want: true},
		{name: "clock skewed backwards", now: shown.Add(-time.Second), want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldHideOnBlur(shown, tc.now, grace); got != tc.want {
				t.Fatalf("shouldHideOnBlur(%v) = %v, want %v", tc.now, got, tc.want)
			}
		})
	}
}

// A popover that was never shown from the tray has no grace to spend: any blur
// it reports is a real one, so it must be allowed to hide.
func TestShouldHideOnBlurNeverShown(t *testing.T) {
	if !shouldHideOnBlur(time.Time{}, time.Now(), popoverBlurGrace) {
		t.Fatal("shouldHideOnBlur(zero shownAt) = false, want true")
	}
}

// markPopoverShown must move the timestamp into the grace window, bump the
// generation (so a pending focus reclaim from an older show is abandoned) and
// re-arm the once-per-show reclaim.
func TestMarkPopoverShown(t *testing.T) {
	popoverMu.Lock()
	popoverShownAt, popoverShowGen, popoverReclaimed = time.Time{}, 7, true
	popoverMu.Unlock()

	markPopoverShown()

	popoverMu.Lock()
	shownAt, gen, reclaimed := popoverShownAt, popoverShowGen, popoverReclaimed
	popoverMu.Unlock()

	if shouldHideOnBlur(shownAt, time.Now(), popoverBlurGrace) {
		t.Fatal("blur immediately after markPopoverShown would hide the popover")
	}
	if gen != 8 {
		t.Fatalf("popoverShowGen = %d, want 8", gen)
	}
	if reclaimed {
		t.Fatal("popoverReclaimed = true, want the reclaim re-armed for this show")
	}
}

// The nil-window paths must be inert: main's second-instance callback and the
// hotkey both run before/after the popover exists in some failure modes.
func TestPopoverHelpersNilSafe(t *testing.T) {
	showPopover(nil)
	showPopoverAt(nil, nil)
	hidePopover(nil)
	togglePopover(nil)
	togglePopoverAt(nil, nil)
	onPopoverBlur(nil, time.Now())
}

// Content-driven popover height (S02 divergence: the design's panel is
// content-sized, so the window follows it). Bounds keep a measurement bug from
// producing a 1px or off-screen window.
func TestClampPopoverHeight(t *testing.T) {
	cases := []struct {
		name string
		in   int
		want int
	}{
		{"below min", 10, popoverMinHeight},
		{"zero (unmeasured)", 0, popoverMinHeight},
		{"negative", -40, popoverMinHeight},
		{"at min", popoverMinHeight, popoverMinHeight},
		{"landing view", 450, 450},
		{"at max", popoverMaxHeight, popoverMaxHeight},
		{"above max", 5000, popoverMaxHeight},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := clampPopoverHeight(tc.in); got != tc.want {
				t.Errorf("clampPopoverHeight(%d) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}
