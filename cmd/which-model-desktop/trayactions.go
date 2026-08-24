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
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/WD-Mitchell/which-model/pkg/whichmodel"
)

// latestReleaseURL is GitHub's API for the newest published release of this
// repository (the module path's own origin).
const latestReleaseURL = "https://api.github.com/repos/WD-Mitchell/which-model/releases/latest"

// releasesPageURL is opened when an update is available; the app has no
// self-updater, so the download is the user's own deliberate step.
const releasesPageURL = "https://github.com/WD-Mitchell/which-model/releases/latest"

// updateCheckTimeout bounds the release lookup. A menu action that hangs on a
// dead network is worse than one that reports "could not check".
const updateCheckTimeout = 10 * time.Second

var (
	// refreshRunning guards the catalogue rebuild; catalogMu additionally
	// serialises the CLI entry point, which mutates whichmodel.Global.
	refreshRunning sync.Mutex
	refreshBusy    bool
	catalogMu      sync.Mutex

	updateRunning sync.Mutex
	updateBusy    bool
)

// refreshBenchmarks rebuilds the scores CSV from models.dev + Artificial
// Analysis, then reloads the in-process catalogue so the popover reflects it
// without a restart.
//
// It runs the real CLI pipeline via whichmodel.ExecuteArgs rather than
// duplicating the collect/derive stages: `catalog refresh`'s implementation is
// unexported and cobra-bound, and a second copy here would drift. ExecuteArgs
// mutates package-level flag state (whichmodel.Global), hence catalogMu.
func (m *trayMenu) refreshBenchmarks() {
	refreshRunning.Lock()
	if refreshBusy {
		refreshRunning.Unlock()
		notice(m.app, "benchmark refresh already running")
		return
	}
	refreshBusy = true
	refreshRunning.Unlock()

	notice(m.app, "refreshing benchmarks…")
	go func() {
		defer func() {
			refreshRunning.Lock()
			refreshBusy = false
			refreshRunning.Unlock()
		}()

		catalogMu.Lock()
		code := whichmodel.ExecuteArgs([]string{"catalog", "refresh"})
		catalogMu.Unlock()

		if code != 0 {
			log.Printf("tray: catalog refresh exited %d", code)
			notice(m.app, "benchmark refresh failed — see the log")
			return
		}
		// Routes are derived from the same catalogue; refreshing scores without
		// them would leave new models unroutable.
		catalogMu.Lock()
		routesCode := whichmodel.ExecuteArgs([]string{"routes", "refresh"})
		catalogMu.Unlock()
		if routesCode != 0 {
			log.Printf("tray: routes refresh exited %d", routesCode)
		}

		if m.svc != nil {
			if err := m.svc.ReloadCatalog(); err != nil {
				log.Printf("tray: catalog reload failed: %v", err)
				notice(m.app, "benchmarks refreshed — restart to pick up the new catalogue")
				return
			}
		}
		notice(m.app, "benchmarks refreshed")
	}()
}

// checkForUpdates compares the running build against the newest GitHub
// release. There is no self-updater: a newer version opens the releases page,
// which is the honest outcome for an unsigned, hand-packaged .app.
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

		latest, err := latestReleaseTag()
		if err != nil {
			log.Printf("tray: update check failed: %v", err)
			notice(m.app, "could not check for updates")
			return
		}
		current := whichmodel.Version
		switch {
		case current == "" || current == "dev":
			// A local build carries no release identity, so "up to date" would
			// be a guess. Report what is out there and let the user judge.
			notice(m.app, fmt.Sprintf("development build; latest release is %s", latest))
			openURL(releasesPageURL)
		case sameVersion(current, latest):
			notice(m.app, fmt.Sprintf("up to date (%s)", current))
		default:
			notice(m.app, fmt.Sprintf("update available: %s (you have %s)", latest, current))
			openURL(releasesPageURL)
		}
	}()
}

// latestReleaseTag returns the newest release's tag name.
func latestReleaseTag() (string, error) {
	client := &http.Client{Timeout: updateCheckTimeout}
	req, err := http.NewRequest(http.MethodGet, latestReleaseURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github returned %s", resp.Status)
	}
	var payload struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	if payload.TagName == "" {
		return "", fmt.Errorf("release has no tag_name")
	}
	return payload.TagName, nil
}

// sameVersion compares a build version against a release tag, tolerating the
// "v" prefix tags carry and builds do not.
func sameVersion(current, tag string) bool {
	return strings.TrimPrefix(current, "v") == strings.TrimPrefix(tag, "v")
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
