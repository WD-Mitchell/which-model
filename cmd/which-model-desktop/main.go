// which-model desktop host. Full bootstrap per S00 SPEC §2.1 and S02 SPEC
// §2.1: resolve config paths, load config, build the events bridge and the
// service, create the app (single-instance), the tray + popover, wire the
// bridge to the frontend, start the catalog refresher, then run. Fatal init
// errors show a native modal dialog (osascript on darwin, stderr elsewhere)
// and exit 1 — never a blank window.
//
// The three usage provider packages are blank-imported so their init
// registration runs and the binary carries all providers (D00 §2.10); the
// desktop binary never builds with -tags nousage (S02 SPEC §2.1).
package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/WD-Mitchell/which-model/internal/config"
	"github.com/WD-Mitchell/which-model/internal/service"
	_ "github.com/WD-Mitchell/which-model/internal/usage/provider/claude"
	_ "github.com/WD-Mitchell/which-model/internal/usage/provider/codex"
	_ "github.com/WD-Mitchell/which-model/internal/usage/provider/copilot"
	"github.com/wailsapp/wails/v3/pkg/application"
)

const fatalStartupTitle = "which-model can't start"

func main() {
	// 1. Resolve config paths (S02 SPEC §2.1.1).
	home, _ := os.UserHomeDir()
	paths := config.ResolvePaths(runtime.GOOS, home, os.Getenv)

	// 2. Load config (S02 SPEC §2.1.2).
	cfg, err := config.Load(config.LoadOptions{Path: paths.UserConfigFile})
	if err != nil {
		fatalStartup(nil, fatalStartupTitle, err.Error())
	}

	// 3. Events bridge + service (S02 SPEC §2.1.3). The bridge queues events
	// until the app exists; refresh wires to the tray label and is set after
	// setupTray. nil-safe guard: refresh is populated before any event drains.
	var refresh func()
	bridge := newEmitBridge(func(name string) {
		switch name {
		case service.EventPickRecorded, service.EventConfigChanged, service.EventCatalogChanged:
			if refresh != nil {
				refresh()
			}
		}
	})

	svc, err := service.New(paths, cfg, bridge.Emit)
	if err != nil {
		title, msg := initErrorMessage(err)
		fatalStartup(nil, title, msg)
	}

	// 4. application.New with single-instance (S02 SPEC §2.1.4). The second
	// launch callback shows the popover (pop is assigned immediately after).
	var pop *application.WebviewWindow
	app := application.New(application.Options{
		Name: "which-model",
		Mac: application.MacOptions{
			// Menu-bar app: no Dock icon; only the tray + on-demand windows.
			ActivationPolicy: application.ActivationPolicyAccessory,
		},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "com.wdmitchell.which-model",
			OnSecondInstanceLaunch: func(application.SecondInstanceData) {
				if pop != nil {
					showPopover(pop)
				}
			},
		},
		// S03 SPEC §2.7: mark the app quitting so the settings close hook
		// stops intercepting and allows clean teardown.
		OnShutdown: func() {
			setQuitting(true)
		},
	})

	// 5. Tray + popover, then hand the app to the bridge (S02 SPEC §2.1.5).
	pop = newPopoverWindow(app)
	_, refresh = setupTray(app, svc, pop)
	bridge.SetApp(app)

	// 6. Catalog refresher: immediate reload then every 5m until shutdown. The
	// app context is cancelled during cleanup, stopping the loop (S02 SPEC
	// §2.1.6, B08 SPEC §2.10).
	go svc.StartRefresher(app.Context(), 5*time.Minute)

	// 7. Run (S02 SPEC §2.1.7).
	if err := app.Run(); err != nil {
		fatalStartup(app, fatalStartupTitle, err.Error())
	}
}

// initErrorMessage maps a service.New failure to the fatal dialog (title,
// message). When the error is the missing-scores-CSV io_error the message is
// the remedial form with the carried path (S02 SPEC §3); all other errors are
// shown verbatim.
func initErrorMessage(err error) (title, message string) {
	if s := scoresCSVPath(err); s != "" {
		return fatalStartupTitle, fmt.Sprintf(
			"The model catalog is missing.\n\nExpected file:\n%s\n\nRun \"which-model catalog refresh\" in a terminal, then reopen which-model.", s)
	}
	return fatalStartupTitle, err.Error()
}

// scoresCSVPath extracts the CSV path from a missing-scores error. The
// service's scoresMissingError renders "scores CSV not found at <path>; run:
// which-model catalog refresh"; we detect that form by its fixed prefix (the
// type itself is unexported, and the SPEC keys detection on the message).
func scoresCSVPath(err error) string {
	const prefix = "scores CSV not found at "
	msg := err.Error()
	if !strings.HasPrefix(msg, prefix) {
		return ""
	}
	rest := msg[len(prefix):]
	if i := strings.Index(rest, ";"); i >= 0 {
		rest = rest[:i]
	}
	return strings.TrimSpace(rest)
}

// fatalStartup shows a native modal dialog then exits 1 (S02 SPEC §3). The
// Wails message-dialog machinery requires a running platform app, but every
// fatal init error happens before app.Run, so we use the documented platform
// fallback: osascript on darwin, stderr elsewhere. app is accepted for
// signature parity with the SPEC; it is not consulted.
func fatalStartup(_ *application.App, title, message string) {
	if runtime.GOOS == "darwin" {
		showNativeAlert(title, message)
	} else {
		log.Printf("fatal: %s: %s", title, message)
	}
	os.Exit(1)
}

// showNativeAlert raises a synchronous native modal alert via osascript on
// macOS. Best-effort; a failure to spawn osascript is logged and we still
// exit 1.
func showNativeAlert(title, message string) {
	// osascript escapes: pass the strings via argv; tabs/newlines in the
	// message are preserved when quoted with qQ.
	script := fmt.Sprintf(`display alert %q message %q as critical`, title, message)
	cmd := exec.Command("osascript", "-e", script)
	cmd.Stdout = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Printf("fatal: failed to show dialog: %v", err)
	}
}
