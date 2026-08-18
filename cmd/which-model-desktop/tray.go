// Tray icon + live top-pick score label (S02 SPEC §2.4). setupTray creates the
// system tray with a template icon, sets the current top-pick score label, and
// wires the popover via AttachWindow (fallback-ladder step 1, the primary
// path: Wails positions the window under the icon and toggles it on click,
// hiding it on focus loss). refresh is invoked by the bridge tap whenever
// pick:recorded | config:changed | catalog:changed arrive, recomputing the
// label in place (S02 SPEC §2.4c).
package main

import (
	"context"
	"log"
	"math"
	"runtime"
	"strconv"

	"github.com/WD-Mitchell/which-model/internal/service"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// trayLabelProfile and trayLabelHolds pin the default landing profile the
// label mirrors (S02 SPEC Decisions "Label profile": balanced_implementation,
// Holds 1).
const (
	trayLabelProfile = "balanced_implementation"
	trayLabelHolds   = 1
)

// emDash is the label shown when ranking fails or returns no route (S02 SPEC
// §2.4b).
const emDash = "—"

// errRankingUnavailable is the sentinel the rankTopScore seam returns while
// B04's svc.Pick.Rank is not landed (see rankTopScore below).
var errRankingUnavailable = &rankUnavailableError{}

type rankUnavailableError struct{}

func (*rankUnavailableError) Error() string {
	return "pick ranking unavailable: B04 not landed"
}

// rankTopScore is the seam for the top-pick score used by trayLabel. Per S02
// SPEC §2.4c it should rank
//
//	svc.Pick.Rank(ctx, service.RankRequest{ProfileSlug: trayLabelProfile, Holds: trayLabelHolds})
//
// and return resp.Candidates[0].Score. B04 (pick.go) is a later wave and is
// not present when this file is written, so the seam defaults to
// "ranking unavailable" → trayLabel renders "—" (SPEC §2.4c: rank failures
// are log-only, never a crash). When B04 lands, wire this to svc.Pick.Rank in
// main.bootstrap via:
//
//	rankTopScore = func(ctx context.Context, svc *service.Services) (float64, error) {
//	    resp, err := svc.Pick.Rank(ctx, service.RankRequest{
//	        ProfileSlug: trayLabelProfile, Holds: trayLabelHolds,
//	    })
//	    if err != nil {
//	        return 0, err
//	    }
//	    if len(resp.Candidates) == 0 {
//	        return 0, errRankingUnavailable // no route -> "—"
//	    }
//	    return resp.Candidates[0].Score, nil
//	}
var rankTopScore = func(_ context.Context, _ *service.Services) (float64, error) {
	return 0, errRankingUnavailable
}

// setupTray creates the tray, sets the template/monochrome icon and the
// initial label, and attaches the popover. It returns the tray and a refresh
// func that recomputes the label (driven by the bridge tap).
func setupTray(app *application.App, svc *service.Services, pop *application.WebviewWindow) (tray *application.SystemTray, refresh func()) {
	tray = app.SystemTray.New()

	// macOS: template raster so AppKit tints it for the menu bar (light/dark).
	// Windows/Linux: the same monochrome PNG via SetIcon (S02 SPEC §2.4a).
	if runtime.GOOS == "darwin" {
		tray.SetTemplateIcon(trayIconTemplate)
	} else {
		tray.SetIcon(trayIconTemplate)
	}

	// Live top-pick score label, recomputed on startup and on tap events.
	refresh = func() {
		label := trayLabel(context.Background(), svc)
		log.Printf("tray: label set to %q", label)
		tray.SetLabel(label)
	}
	refresh()

	// Primary attach path (fallback-ladder step 1): Wails positions the window
	// under the icon and toggles show/hide on click, hiding on focus loss.
	// No explicit OnClick is set — applySmartDefaults wires ToggleWindow.
	tray.AttachWindow(pop)

	return tray, refresh
}

// trayLabel ranks the default landing profile and formats the top candidate's
// score as an integer string, or "—" when ranking fails or returns no route
// (S02 SPEC §2.4b). It never returns an error: failures are logged and yield
// the em dash.
func trayLabel(ctx context.Context, svc *service.Services) string {
	score, err := rankTopScore(ctx, svc)
	if err != nil {
		log.Printf("tray: rank refresh failed: %v", err)
		return emDash
	}
	return strconv.Itoa(int(math.Round(score)))
}
