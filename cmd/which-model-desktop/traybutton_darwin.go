//go:build darwin && !ios

// Go half of the tray left-click repair. See traybutton_darwin.m for the full
// explanation of the upstream Wails bug this works around.
package main

/*
#cgo CFLAGS: -mmacosx-version-min=10.14 -x objective-c
#cgo LDFLAGS: -framework Cocoa

int  wmRewireStatusButtons(void);
int  wmInstallTrayMonitor(void);
void wmSetTrayDebug(int on);
int  wmTestClickStatusButton(void);
int  wmStatusButtonFrameCG(double *x, double *y, double *w, double *h);
*/
import "C"

import (
	"log"
	"os"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	// trayRewireRetry is the gap between attempts to find the status item.
	// Wails creates it asynchronously from the app-start goroutine
	// (application.go:745 runs each pending runnable in its own goroutine), so
	// it is usually absent on the first attempt.
	trayRewireRetry = 250 * time.Millisecond

	// trayRewireAttempts bounds the retry loop: 20 * 250ms = 5s, comfortably
	// past a cold launch without spinning forever when no status item was ever
	// created (show_menu_bar_icon = false).
	trayRewireAttempts = 20

	// trayClickDebounce collapses the two delivery paths. The button action and
	// the event monitor can both fire for one physical click; without this the
	// popover would toggle twice and appear not to open at all — the very
	// symptom being fixed.
	trayClickDebounce = 150 * time.Millisecond
)

var (
	// trayButtonMu guards trayButtonClick and trayLastClick.
	trayButtonMu sync.Mutex
	// trayButtonClick is invoked on every left-click of the menu-bar icon. It is
	// set by installTrayButtonHandler before either delivery path is armed, so a
	// click can never arrive with it nil.
	trayButtonClick func()
	// trayLastClick is when the last click was accepted (debounce, above).
	trayLastClick time.Time
)

//export wmTrayButtonClicked
func wmTrayButtonClicked() {
	trayButtonMu.Lock()
	fn := trayButtonClick
	now := time.Now()
	if fn == nil || now.Sub(trayLastClick) < trayClickDebounce {
		trayButtonMu.Unlock()
		return
	}
	trayLastClick = now
	trayButtonMu.Unlock()

	log.Printf("tray: left click")
	fn()
}

// installTrayButtonHandler registers onClick as the menu-bar icon's left-click
// action and arms both native delivery paths: the button target/action rewire
// (primary) and a local event monitor (fallback). Either firing is enough; both
// firing is collapsed by trayClickDebounce.
//
// It returns immediately. Failure is non-fatal and logged — the popover stays
// reachable via the global hotkey and by relaunching the app (S05 SPEC §3:
// integrations never crash the app).
func installTrayButtonHandler(onClick func()) {
	trayButtonMu.Lock()
	trayButtonClick = onClick
	trayButtonMu.Unlock()

	debug := 0
	if os.Getenv("WM_TRAYDEBUG") == "1" {
		debug = 1
	}

	// Fallback path first: it needs no status item, only a live NSApp, so it is
	// armed before the retry loop starts hunting for the button.
	time.AfterFunc(trayRewireRetry, func() {
		var ok int
		application.InvokeSync(func() {
			C.wmSetTrayDebug(C.int(debug))
			ok = int(C.wmInstallTrayMonitor())
		})
		log.Printf("tray: left-click monitor installed=%v", ok == 1)
	})

	// Primary path: retry until the status item exists.
	var attempt func(int)
	attempt = func(n int) {
		var rewired int
		// NSStatusBar and setTarget:/setAction: are main-thread-only.
		application.InvokeSync(func() {
			rewired = int(C.wmRewireStatusButtons())
		})
		if rewired > 0 {
			log.Printf("tray: click handler bound to %d status button(s)", rewired)
			return
		}
		if n >= trayRewireAttempts {
			log.Printf("tray: could not bind the status button after %d attempts; "+
				"relying on the event monitor", n)
			return
		}
		time.AfterFunc(trayRewireRetry, func() { attempt(n + 1) })
	}
	time.AfterFunc(trayRewireRetry, func() { attempt(1) })
}

// testClickStatusButton drives the rewired status button from inside the app.
// Diagnostic only (WM_SELFTEST=1); see wmTestClickStatusButton in the .m file.
func testClickStatusButton() bool {
	var ok int
	application.InvokeSync(func() {
		ok = int(C.wmTestClickStatusButton())
	})
	return ok == 1
}

// statusButtonFrameCG reports the status button's frame in CG screen
// coordinates. Diagnostic only (WM_SELFTEST=1).
func statusButtonFrameCG() (x, y, w, h float64, ok bool) {
	var cx, cy, cw, ch C.double
	var got C.int
	application.InvokeSync(func() {
		got = C.wmStatusButtonFrameCG(&cx, &cy, &cw, &ch)
	})
	return float64(cx), float64(cy), float64(cw), float64(ch), got == 1
}
