// Tray icon + live top-pick score label + right-click menu (S02 SPEC §2.4/
// §2.6). setupTray creates the system tray with the template icon, sets the
// top-pick score label, wires an explicit left-click handler onto the popover
// and hands the right-click menu to traymenu.go. refresh is invoked by the
// bridge tap whenever pick:recorded | config:changed | catalog:changed arrive,
// recomputing the label and (if the profile list moved) the menu in place
// (S02 SPEC §2.4c).
package main

import (
	"context"
	"errors"
	"log"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/WD-Mitchell/which-model/internal/service"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// trayLabelProfile and trayLabelHolds pin the default landing profile the
// label mirrors (S02 SPEC Decisions "Label profile": balanced_implementation).
const (
	trayLabelProfile = "balanced_implementation"
	trayLabelHolds   = 1
)

// trayLabelGap is a U+2009 THIN SPACE between the glyph and the label on the
// platforms that still set a plain tray label (traytitle_other.go). AppKit
// butts an NSStatusItem's title straight up against its image; the mockup
// leaves 6px of air there (demo.dc.html line 45, `gap:6px`), and a full space
// is too wide in the menu bar. macOS no longer needs it: the item is one
// composed image, which owns its own spacing (traytitle_darwin.m).
const trayLabelGap = "\u2009"

// trayMenuFallbackDelay is when the right-click menu is installed if the
// ApplicationStarted event never arrives. Long enough that SystemTray.Run has
// certainly executed, short enough that the user has not reached for the mouse.
const trayMenuFallbackDelay = 2 * time.Second

var (
	// trayMu guards trayFallbackTimer, which tracks the fallback menu-start
	// timer so normal startup and shutdown can cancel it.
	trayMu            sync.Mutex
	trayFallbackTimer *time.Timer
)

// cancelTrayTimers stops the tracked menu fallback timer, if any.
func cancelTrayTimers() {
	trayMu.Lock()
	defer trayMu.Unlock()
	if trayFallbackTimer != nil {
		trayFallbackTimer.Stop()
		trayFallbackTimer = nil
	}
}

// errNoTopPick is returned by rankTopPick when ranking succeeded but produced
// no route — an empty catalog, or every provider disabled. The label is then
// empty: a lone em dash sitting in the menu bar reads as breakage, not as
// "nothing to show" (S02 SPEC §2.4b).
var errNoTopPick = errors.New("no ranked candidate")

// rankTopPick is the seam for the top pick shown in the menu bar (S02 SPEC
// §2.4c). It is a var so tests can substitute a deterministic pick without
// standing up a service. The slug is the profile to rank FOR — the menu bar
// names it on its own line now, so the label and the profile must come from
// one and the same request.
var rankTopPick = func(ctx context.Context, svc *service.Services, slug string) (service.RankedModel, error) {
	if svc == nil {
		return service.RankedModel{}, errNoTopPick
	}
	if slug == "" {
		slug = trayLabelProfile
	}
	resp, err := svc.Rank(ctx, service.RankRequest{
		ProfileSlug: slug,
		Holds:       trayLabelHolds,
	})
	if err != nil {
		return service.RankedModel{}, err
	}
	if len(resp.Candidates) == 0 {
		return service.RankedModel{}, errNoTopPick
	}
	return resp.Candidates[0], nil
}

// setupTray creates the tray, sets the template/monochrome icon and the
// initial label, wires the click paths, and returns the tray plus a refresh
// func that recomputes the label and menu (driven by the bridge tap).
func setupTray(app *application.App, svc *service.Services, pop *application.WebviewWindow) (tray *application.SystemTray, refresh func()) {
	tray = app.SystemTray.New()

	// macOS: template raster so AppKit tints it for the menu bar (light/dark).
	// Windows/Linux: the same monochrome PNG via SetIcon (S02 SPEC §2.4a). The
	// 2x raster is the one handed to AppKit: systemtray_darwin.m:160 resizes
	// whatever we pass to [[NSStatusBar systemStatusBar] thickness] == 22pt, so
	// the 44px raster maps 1:1 onto a retina menu bar instead of being
	// upscaled. See assets/gen_tray_icon.go.
	// Starting glyph. refresh replaces it with the top pick's provider mark as
	// soon as ranking resolves (setTrayIcon), and puts this one back whenever
	// the pick's provider has no mark.
	if runtime.GOOS == "darwin" {
		tray.SetTemplateIcon(trayIconTemplate2x)
	} else {
		tray.SetIcon(trayIconTemplate)
	}

	// Primary attach path: Wails positions the window under the icon, and the
	// attached window is what its right-click branch hides before showing the
	// menu (systemtray_darwin.go:115-120).
	tray.AttachWindow(pop)
	// Published for resizePopover: a content-driven height change on a visible
	// popover must re-anchor it under this tray (popover.go).
	popoverTrayRef = tray

	// Explicit left-click handler so the behaviour is ours rather than
	// applySmartDefaults' ToggleWindow — same show path as the hotkey, and it
	// records the show timestamp the blur grace period needs (popover.go).
	// Left click -> toggle the popover. Registered twice, deliberately:
	//
	//   * tray.OnClick is the portable path and is what actually runs on
	//     Windows and Linux.
	//   * installTrayButtonHandler is the macOS repair. Wails arms the status
	//     button's action but sets target/action on the NSStatusItem, using API
	//     deprecated in 10.14 and inert today, so OnClick NEVER fires on macOS.
	//     Measured: probes in both OnClick and OnDoubleClick logged nothing on a
	//     real click. See traybutton_darwin.m. It is a no-op off macOS.
	//
	// Both routes call the same toggle, and only one of them can ever fire on a
	// given platform, so there is no double-toggle.
	tray.OnClick(func() { togglePopoverAt(tray, pop) })
	installTrayButtonHandler(func() { togglePopoverAt(tray, pop) })

	// Right-click menu. Installed only once the app is running; see the
	// traymenu.go file comment for why that timing is load-bearing.
	menu := newTrayMenu(app, svc, tray, pop)
	// start marks the menu ready and redraws the title: Start is where the
	// first real profile selection appears, and the title names it.
	start := func() {
		trayMu.Lock()
		if trayFallbackTimer != nil {
			trayFallbackTimer.Stop()
			trayFallbackTimer = nil
		}
		trayMu.Unlock()
		menu.Start()
		if refresh != nil {
			refresh()
		}
	}

	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		start()
	})
	// Belt and braces: if ApplicationStarted never reaches us, the right-click
	// menu would silently not exist, which is the whole feature. Start is
	// idempotent (the second call finds the signature unchanged and returns),
	// and by trayMenuFallbackDelay the app is long past applySmartDefaults.
	trayMu.Lock()
	trayFallbackTimer = time.AfterFunc(trayMenuFallbackDelay, start)
	trayMu.Unlock()

	// Diagnostic harness (WM_SELFTEST=1 only, never in a shipped run). Drives
	// the same code paths a tray click drives, from a timer, so the show path
	// can be proven independently of AppKit click delivery — and so the
	// settings window's asset requests appear in the log without a click.
	if os.Getenv("WM_SELFTEST") == "1" {
		time.AfterFunc(3*time.Second, func() {
			log.Printf("selftest: popover visible before toggle = %v", pop.IsVisible())
			togglePopoverAt(tray, pop)
			time.AfterFunc(1500*time.Millisecond, func() {
				x, y := pop.Position()
				w, h := pop.Size()
				log.Printf("selftest: popover visible after toggle = %v pos=(%d,%d) size=%dx%d",
					pop.IsVisible(), x, y, w, h)
				bx, by, bw, bh, ok := statusButtonFrameCG()
				log.Printf("selftest: status button cg-frame ok=%v (%.0f,%.0f) %.0fx%.0f",
					ok, bx, by, bw, bh)
			})
		})
	}

	// Live two-line menu-bar title, recomputed on startup and on tap events.
	// The menu is refreshed FIRST: it owns the selected profile, and the title
	// names that profile, so reading it before the refresh would label the
	// startup title with the pre-selection fallback.
	refresh = func() {
		menu.Refresh()
		slug := menu.Selected()
		if slug == "" {
			slug = trayLabelProfile
		}
		// The popover wins once it has spoken (see trayPickFromPopover): it
		// knows the active profile and the live weight overrides, and it
		// re-pushes whenever its own ranking changes, so a config or catalog
		// event does not need — or want — a second opinion here.
		if top, bottom, provider, ok := popoverPick(); ok {
			setTrayTitleLines(tray, top, bottom, provider)
			return
		}
		top, bottom, provider := trayTitleLines(context.Background(), svc, slug)
		log.Printf("tray: title set to %q / %q (provider %q)", top, bottom, provider)
		setTrayTitleLines(tray, top, bottom, provider)
	}
	// The tray menu drives the profile the title names, so a quick-select has
	// to redraw it — nothing else fires on that path (no pick is recorded
	// until the user launches).
	menu.OnSelectionChanged(func() { refresh() })
	refresh()

	return tray, refresh
}

// trayModelLine names a ranked pick as "<model> (<reasoning>)" — what the user
// would actually launch, which the bare score never told them. Reasoning is
// part of the identity: the same model at two efforts is two different picks
// (S02 SPEC §2.4b, amended). Empty for a nameless pick, which would otherwise
// render as an empty bracket.
func trayModelLine(pick service.RankedModel) string {
	name := strings.TrimSpace(pick.ModelName)
	if name == "" {
		return ""
	}
	if reasoning := strings.TrimSpace(pick.Reasoning); reasoning != "" {
		name = name + " (" + reasoning + ")"
	}
	return name
}

// trayProfileName is the seam for the profile line — the display name of the
// profile the label ranks for, which is what the top line of the two-line
// menu-bar title shows. A var for the same reason rankTopPick is.
//
// It degrades to the slug: the built-ins' display names ARE their slugs today
// (internal/pick/profiles.go), and the popover shows the same string, so the
// two never disagree.
var trayProfileName = func(ctx context.Context, svc *service.Services, slug string) string {
	if slug == "" {
		slug = trayLabelProfile
	}
	if svc == nil {
		return slug
	}
	detail, err := svc.Profiles().Get(ctx, slug)
	if err != nil {
		log.Printf("tray: profile lookup for %q failed: %v", slug, err)
		return slug
	}
	if name := strings.TrimSpace(detail.Name); name != "" {
		return name
	}
	return slug
}

var (
	// trayPickMu guards the popover-supplied pick below.
	trayPickMu sync.Mutex
	// trayPickFromPopover is true once the popover has pushed a pick. From then
	// on it is the menu bar's source of truth: the host's own ranking cannot
	// see the popover's active profile or its ephemeral weight overrides, so
	// recomputing on the next config event would silently revert the title to
	// a different profile's pick.
	trayPickFromPopover                             bool
	trayPickTop, trayPickBottom, trayPickProviderID string
)

// setTrayPickFromUI records and draws the pick the popover is showing
// (WindowService.SetTrayPick). Safe before the status item exists: the draw
// path retries (traytitle_darwin.go).
func setTrayPickFromUI(profileName, modelName, reasoning, provider string) {
	top := strings.TrimSpace(profileName)
	bottom := trayModelLine(service.RankedModel{ModelName: modelName, Reasoning: reasoning})
	provider = strings.TrimSpace(provider)

	trayPickMu.Lock()
	trayPickFromPopover = true
	trayPickTop, trayPickBottom, trayPickProviderID = top, bottom, provider
	trayPickMu.Unlock()

	log.Printf("tray: popover pick %q / %q (provider %q)", top, bottom, provider)
	setTrayTitleLines(popoverTrayRef, top, bottom, provider)
}

// popoverPick returns the last pick the popover pushed, and whether there is
// one at all.
func popoverPick() (top, bottom, provider string, ok bool) {
	trayPickMu.Lock()
	defer trayPickMu.Unlock()
	return trayPickTop, trayPickBottom, trayPickProviderID, trayPickFromPopover
}

// trayTitleLines is everything the menu-bar item shows for one profile: the
// profile name on top, the model it picked below, and the provider whose mark
// becomes the icon. One rank call feeds all three — they must describe the same
// pick, and refresh runs on every pick, config and catalog event.
//
// Both lines carry the thin-space gap so neither butts up against the icon. An
// unrankable state blanks everything rather than leaving a profile name
// stranded over nothing — a lone name reads as a broken item, the same reason
// the em-dash placeholder was dropped (S02 SPEC §2.4b) — and an empty provider
// leaves the app glyph in place.
func trayTitleLines(
	ctx context.Context,
	svc *service.Services,
	slug string,
) (top, bottom, provider string) {
	pick, err := rankTopPick(ctx, svc, slug)
	if err != nil {
		log.Printf("tray: rank refresh failed: %v", err)
		return "", "", ""
	}
	bottom = trayModelLine(pick)
	if bottom == "" {
		return "", "", ""
	}
	return trayProfileName(ctx, svc, slug), bottom, strings.TrimSpace(pick.Provider)
}
