//go:build !darwin || ios

// Non-macOS menu-bar title. Windows and Linux tray backends render a single
// line of text beside the icon — there is no two-line title to draw — so the
// two lines are joined into one and handed to Wails' own SetLabel.
package main

import (
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// traySingleLineSep separates the profile from the model when both lines have
// to share one. An en dash with spaces, which reads as a break rather than as
// part of either name.
const traySingleLineSep = " – "

// setTrayTitleLines sets the tray label to "<profile> – <model>", dropping
// either half when it is empty.
//
// The provider is accepted and ignored: the provider mark is drawn into the
// menu-bar item on macOS (traytitle_darwin.m), which these backends have no
// equivalent of — they take a raster icon, set once at startup.
func setTrayTitleLines(tray *application.SystemTray, top, bottom, _ string) {
	if tray == nil {
		return
	}
	tray.SetLabel(trayLabelGap + traySingleLine(top, bottom))
}

// traySingleLine joins the two title lines for platforms with one line to
// spend. Blank when both halves are blank, so the icon stands alone rather
// than trailing a stray separator.
func traySingleLine(top, bottom string) string {
	top = strings.TrimSpace(top)
	bottom = strings.TrimSpace(bottom)
	switch {
	case top == "":
		return bottom
	case bottom == "":
		return top
	}
	return top + traySingleLineSep + bottom
}
