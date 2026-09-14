// Tray menu actions that reach outside the service layer: rebuilding the
// benchmark catalogue, and checking GitHub for a newer release.
//
// Both run OFF the AppKit main thread and report through notice() — they touch
// the network and can take seconds, and a menu click must never block the menu
// bar. Both are single-flight: a second click while one is running is ignored
// rather than queued, because the work is idempotent and duplicate concurrent
// runs would race on the same cache files.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"runtime"
	"sync"

	"github.com/WD-Mitchell/which-model/pkg/whichmodel"
)

var (
	// refreshRunning guards the catalogue rebuild; catalogMu additionally
	// serialises the CLI entry point, which mutates whichmodel.Global.
	refreshRunning sync.Mutex
	refreshBusy    bool
	catalogMu      sync.Mutex

	updateRunning sync.Mutex
	updateBusy    bool
)

// refreshData rebuilds the scores CSV (GitHub repo by default, or a
// local Artificial Analysis collect when that is enabled in Settings), then
// rebuilds routes so the popover reflects the new catalogue without a restart.
func (m *trayMenu) refreshData() {
	refreshRunning.Lock()
	if refreshBusy {
		refreshRunning.Unlock()
		notice(m.app, "data refresh already running")
		return
	}
	refreshBusy = true
	refreshRunning.Unlock()

	notice(m.app, "refreshing data…")
	go func() {
		defer func() {
			refreshRunning.Lock()
			refreshBusy = false
			refreshRunning.Unlock()
		}()

		if m.svc == nil {
			notice(m.app, "data refresh failed — engine not ready")
			return
		}
		if err := m.svc.Providers().RefreshRoutes(context.Background()); err != nil {
			log.Printf("tray: %v", err)
			notice(m.app, "data refresh failed — see the log")
			return
		}
		notice(m.app, "data refreshed")
	}()
}

// refreshCatalogCLI runs `which-model catalog refresh` under catalogMu so
// the tray item and Settings Refresh models cannot race on whichmodel.Global.
func refreshCatalogCLI() error {
	catalogMu.Lock()
	defer catalogMu.Unlock()
	if code := whichmodel.ExecuteArgs([]string{"catalog", "refresh", "--rebuild"}); code != 0 {
		return fmt.Errorf("catalog refresh exited %d", code)
	}
	return nil
}

// checkForUpdates compares the running build against published GitHub versions,
// including prereleases. Only a newer version opens its specific release page.
func (m *trayMenu) checkForUpdates() {
	updateRunning.Lock()
	if updateBusy {
		updateRunning.Unlock()
		return
	}
	updateBusy = true
	updateRunning.Unlock()

	go func() {
		defer func() {
			updateRunning.Lock()
			updateBusy = false
			updateRunning.Unlock()
		}()

		ctx, cancel := context.WithTimeout(context.Background(), updateCheckTimeout)
		defer cancel()
		message, link, err := checkReleaseUpdate(ctx, &http.Client{}, whichmodel.Version)
		if err != nil {
			log.Printf("tray: update check failed: %v", err)
			notice(m.app, "could not check for updates")
			return
		}
		notice(m.app, message)
		if link != "" {
			openURL(link)
		}
	}()
}

// openURL opens a link in the user's browser. Failure is logged, never fatal.
func openURL(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		log.Printf("tray: could not open %s: %v", url, err)
	}
}
