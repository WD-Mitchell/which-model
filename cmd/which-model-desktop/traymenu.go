// Tray right-click menu (S02 SPEC §2.6). A native NSMenu hung off the system
// tray, and now the app's ONLY app-level menu: it carries what the popover's
// hamburger used to (Custom weights…, Settings…, Quit) plus a Quick select
// submenu of every profile, Refresh data and Check for updates. The
// popover's hamburger was removed rather than duplicated — one menu, in the
// place macOS users right-click for it.
//
// # WHY THE MENU IS INSTALLED ONLY AFTER ApplicationStarted
//
// Wails decides the tray's click wiring once, inside SystemTray.Run:
//
//	systemtray.go:143  func (s *SystemTray) applySmartDefaults() {
//	                       ...
//	                       if s.rightClickHandler == nil && hasMenu {
//	                           s.rightClickHandler = s.ShowMenu
//	                       }
//
// and the macOS click path only takes the good route while that handler is nil:
//
//	systemtray_darwin.go:114  case rightButtonDown:
//	                              if systemTray.parent.rightClickHandler == nil {
//	                                  // Hide the attached window before the menu appears.
//	                                  ...
//	                                  return 1
//
// Returning 1 from systrayPreClickCallback makes the NSEvent monitor installed
// in systemTrayNew assign statusItem.menu *before* the button processes the
// mouse-down, so AppKit runs its own menu tracking: correct button highlight,
// popover hidden first, no app activation. The ShowMenu fallback instead
// dispatch_asyncs a synthesized mouse-down into a button that is already inside
// a tracking loop.
//
// So: SetMenu must not happen before the tray has run (menu == nil at
// applySmartDefaults time keeps rightClickHandler nil), and it is safe from
// ApplicationStarted onwards because SystemTray.Run is scheduled from
// App.Run's pendingRun loop, which completes before NSApp finishes launching.
// The LEFT branch is unaffected: it only shortcuts to the menu when BOTH
// clickHandler and attachedWindow are unset, and tray.go sets both.
package main

import (
	"context"
	"log"
	"sort"
	"strings"
	"sync"

	"github.com/WD-Mitchell/which-model/internal/service"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// trayProfileEvent is the app-level channel the tray uses to tell the popover
// which profile the user quick-selected. It is deliberately NOT a member of
// the closed D00 §3 EngineEvent enum — nothing in the engine emits it, and
// packages/core must keep mirroring only engine events.
const trayProfileEvent = "tray:profile"

// traySettingsLabel is the last item's label; the ellipsis is the single
// U+2026 character macOS menus use, not three periods.
const traySettingsLabel = "Settings…"

// trayViewEvent tells the popover which view to open. Like trayProfileEvent it
// is app-level, not an engine EngineEvent. No menu item emits it today —
// "Custom weights…" was removed once the Sliders tab became the way in — but
// the channel stays: the popover still listens, and dev/browser mode drives it
// as a DOM event (apps/desktop/src/lib/trayEvents.ts).
const trayViewEvent = "tray:view"

// Menu labels. Ellipses are the single U+2026 character macOS menus use.
const (
	trayQuickSelectLabel = "Quick select"
	trayRefreshLabel     = "Refresh data"
	trayUpdateLabel      = "Check for updates…"
	trayQuitLabel        = "Quit which-model"
)

// trayMenu owns the tray's menu and its lifecycle. The menu is rebuilt only
// when the profile list actually changes (see signature below); a plain
// selection change just moves the native checkmark, which avoids churning
// NSMenu objects on every pick:recorded.
type trayMenu struct {
	app  *application.App
	svc  *service.Services
	tray *application.SystemTray
	pop  *application.WebviewWindow

	mu          sync.Mutex
	ready       bool   // ApplicationStarted has fired
	sig         string // signature of the installed menu
	selected    string // selected use-case slug
	userProfile string // saved work profile represented by the menu
	// onSelection is called after a quick-select changes the selected profile.
	// setupTray uses it to redraw the menu-bar title, which names that profile.
	onSelection func()
}

// OnSelectionChanged registers the callback run after a quick-select. Called
// once, during setup, before the menu can be clicked.
func (m *trayMenu) OnSelectionChanged(fn func()) {
	m.mu.Lock()
	m.onSelection = fn
	m.mu.Unlock()
}

// newTrayMenu builds the (not yet installed) menu owner.
func newTrayMenu(
	app *application.App,
	svc *service.Services,
	tray *application.SystemTray,
	pop *application.WebviewWindow,
) *trayMenu {
	return &trayMenu{app: app, svc: svc, tray: tray, pop: pop}
}

// Start marks the app as running and installs the first menu. Called from the
// ApplicationStarted hook — see the file comment for why not earlier.
func (m *trayMenu) Start() {
	m.mu.Lock()
	m.ready = true
	m.mu.Unlock()
	m.Refresh()
}

// Refresh rebuilds the menu if the profile list changed. No-op before Start.
// Every service call is guarded: a failure logs and still yields a usable menu
// (Settings… alone), never a panic and never an empty menu bar.
func (m *trayMenu) Refresh() {
	m.mu.Lock()
	ready := m.ready
	m.mu.Unlock()
	if !ready || m.tray == nil {
		return
	}

	profiles := m.listProfiles()
	userProfile := m.activeUserProfile()
	ordered := profileMenuEntries(profiles, userProfile)

	m.mu.Lock()
	if m.selected == "" || m.userProfile != userProfile.Slug {
		m.selected = userProfile.DefaultUseCase
		m.userProfile = userProfile.Slug
	}
	selected := m.selected
	sig := trayMenuSignature(ordered, selected != "")
	if sig == m.sig {
		m.mu.Unlock()
		return
	}
	menu := m.build(ordered, selected)
	m.sig = sig
	m.mu.Unlock()

	m.tray.SetMenu(menu)
}

// Selected returns the currently checked profile slug ("" before the first
// successful Refresh).
func (m *trayMenu) Selected() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.selected
}

// listProfiles reads the profile list, logging and degrading to none on error
// (S02 SPEC §3: host UI failures are log-only).
func (m *trayMenu) listProfiles() []service.ProfileSummary {
	if m.svc == nil {
		return nil
	}
	profiles, err := m.svc.Profiles().List(context.Background())
	if err != nil {
		log.Printf("tray menu: profile list failed: %v", err)
		return nil
	}
	return profiles
}

// complexityScale reads the ordered scale slugs, degrading to none on error.
func (m *trayMenu) complexityScale() []string {
	if m.svc == nil {
		return nil
	}
	return m.svc.Profiles().ComplexityScale()
}

// build assembles the native menu:
//
//	Quick select  ▸  <every profile, in complexity-scale order>
//	Settings…
//	────────────
//	Refresh data
//	Check for updates…
//	────────────
//	Quit which-model
//
// The profile items are plain Add() entries, not AddRadio(): the menu is a list
// of ACTIONS ("pick with this profile"), not a persisted mode selection, so a
// checkmark would imply state the menu does not own. `selected` is therefore
// unused for rendering and kept only so callers keep passing the popover's
// current profile, which selectProfile still needs to echo back.
func (m *trayMenu) build(profiles []service.ProfileSummary, selected string) *application.Menu {
	_ = selected
	menu := application.NewMenu()

	// Quick select ▸ — a submenu even when empty, so the menu's shape does not
	// change under the cursor between refreshes.
	quick := menu.AddSubmenu(trayQuickSelectLabel)
	for _, p := range profiles {
		slug := p.Slug
		label := p.Name
		if label == "" {
			label = slug
		}
		quick.Add(label).OnClick(func(*application.Context) { m.selectProfile(slug) })
	}
	if len(profiles) == 0 {
		quick.Add("No profiles").SetEnabled(false)
	}

	menu.Add(traySettingsLabel).OnClick(func(*application.Context) { showSettings(m.app) })

	menu.AddSeparator()
	menu.Add(trayRefreshLabel).OnClick(func(*application.Context) { m.refreshData() })
	menu.Add(trayUpdateLabel).OnClick(func(*application.Context) { m.checkForUpdates() })

	menu.AddSeparator()
	menu.Add(trayQuitLabel).OnClick(func(*application.Context) {
		if m.app != nil {
			m.app.Quit()
		}
	})

	return menu
}

// selectProfile is the profile item's click body: record the selection, tell
// the popover over the app event channel, and show the popover under the tray
// icon so the user sees the new ranking straight away.
func (m *trayMenu) selectProfile(slug string) {
	m.mu.Lock()
	m.selected = slug
	onSelection := m.onSelection
	m.mu.Unlock()

	// The menu-bar title names the selected profile, so it redraws here.
	if onSelection != nil {
		onSelection()
	}

	if m.app != nil {
		m.app.Event.Emit(trayProfileEvent, map[string]any{"slug": slug})
	}
	showPopoverAt(m.tray, m.pop)
}

// orderTrayProfiles sorts profiles by the complexity scale (SPEC §2.6: the
// scale is the user-facing ordering), with anything off the scale — custom
// profiles — following alphabetically by display name.
func orderTrayProfiles(profiles []service.ProfileSummary, scale []string) []service.ProfileSummary {
	rank := make(map[string]int, len(scale))
	for i, slug := range scale {
		rank[slug] = i
	}

	out := append([]service.ProfileSummary(nil), profiles...)
	sort.SliceStable(out, func(i, j int) bool {
		ri, oki := rank[out[i].Slug]
		rj, okj := rank[out[j].Slug]
		switch {
		case oki && okj:
			return ri < rj
		case oki != okj:
			return oki // on-scale profiles first
		}
		li, lj := out[i].Name, out[j].Name
		if li == lj {
			return out[i].Slug < out[j].Slug
		}
		return li < lj
	})
	return out
}

// defaultTraySelection picks the initially checked profile. It mirrors the
// popover's own seed (PopoverApp.tsx: `scale[1] ?? scale[0]`) so the native
// checkmark and the popover agree on launch, falling back to the first listed
// profile when the scale is unavailable.
func defaultTraySelection(profiles []service.ProfileSummary, scale []string) string {
	has := make(map[string]bool, len(profiles))
	for _, p := range profiles {
		has[p.Slug] = true
	}
	for _, i := range []int{1, 0} {
		if i < len(scale) && has[scale[i]] {
			return scale[i]
		}
	}
	if len(profiles) > 0 {
		return profiles[0].Slug
	}
	return ""
}

// trayMenuSignature is the identity of a built menu: rebuild only when a
// profile is added, removed or renamed. hasSelection is folded in so the very
// first build (before a selection exists) is never mistaken for a later one.
func trayMenuSignature(profiles []service.ProfileSummary, hasSelection bool) string {
	var b strings.Builder
	if hasSelection {
		b.WriteString("sel\x00")
	}
	for _, p := range profiles {
		b.WriteString(p.Slug)
		b.WriteByte(0)
		b.WriteString(p.Name)
		b.WriteByte(0)
	}
	return b.String()
}

// activeUserProfile uses persisted settings for both startup and refresh.
func (m *trayMenu) activeUserProfile() service.UserProfile {
	if m.svc == nil {
		return service.UserProfile{}
	}
	settings, err := m.svc.Settings().Get(context.Background())
	if err != nil {
		log.Printf("tray menu: settings failed: %v", err)
		return service.UserProfile{}
	}
	for _, p := range m.svc.Profiles().UserProfiles() {
		if p.Slug == settings.UserProfile {
			return p
		}
	}
	return service.UserProfile{}
}

func profileMenuEntries(all []service.ProfileSummary, profile service.UserProfile) []service.ProfileSummary {
	out := make([]service.ProfileSummary, 0, len(profile.UseCaseSlugs))
	bySlug := make(map[string]service.ProfileSummary, len(all))
	for _, p := range all {
		bySlug[p.Slug] = p
	}
	for _, slug := range profile.UseCaseSlugs {
		if p, ok := bySlug[slug]; ok {
			out = append(out, p)
		}
	}
	return out
}
