//go:build darwin && !ios

// Go half of the menu-bar item: it hands the two title lines and the provider
// mark to the native compositor. See traytitle_darwin.m for why the item is
// drawn by hand rather than set as a title and an icon.
package main

/*
#cgo CFLAGS: -mmacosx-version-min=10.14 -x objective-c
#cgo LDFLAGS: -framework Cocoa

#include <stdlib.h>

int wmSetStatusTitleTwoLine(const char *top, const char *bottom, const void *icon, int iconLen);
*/
import "C"

import (
	"log"
	"sync"
	"time"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	// trayTitleRetry / trayTitleAttempts mirror the click repair's retry loop
	// (traybutton_darwin.go): Wails creates the status item asynchronously, so
	// the first title set after setupTray usually finds no button yet. 20 ×
	// 250ms covers a cold launch without spinning forever when the icon is
	// disabled (show_menu_bar_icon = false).
	trayTitleRetry    = 250 * time.Millisecond
	trayTitleAttempts = 20
)

var (
	// trayTitleMu guards the pending item and the retry flag.
	trayTitleMu sync.Mutex
	// trayTitleTop / trayTitleBottom / trayTitleIcon are the item as of the
	// LAST call. The retry loop re-reads them on every attempt, so a refresh
	// that lands mid-retry wins rather than being overwritten by a stale one.
	trayTitleTop, trayTitleBottom string
	trayTitleIcon                 []byte
	// trayTitleRetrying is true while a retry chain is in flight, so repeated
	// refreshes before the status item exists cannot stack up timers.
	trayTitleRetrying bool
	// trayTitleDrawn is the last composition actually installed, so an
	// unchanged refresh — the common case, since the bridge taps every pick,
	// config and catalog event — costs nothing.
	trayTitleDrawn string
)

// setTrayTitleLines paints the menu-bar item: `provider`'s mark, then top over
// bottom. An unknown provider (or none) leaves the mark off and shows the app
// glyph in its place.
//
// The tray argument is unused on macOS — neither SetLabel nor SetTemplateIcon
// is called. Both assign the status button from dispatch_async blocks that
// would land on top of the composed image (see the .m file).
//
// Never blocks: the work is scheduled onto the main thread from a timer,
// because application.InvokeSync deadlocks if the app is not running yet and
// this is called during setup.
func setTrayTitleLines(_ *application.SystemTray, top, bottom, provider string) {
	icon := providerIcon(provider)
	if icon == nil {
		// The app's own glyph, so the item never appears without a mark.
		icon = trayIconTemplate2x
	}

	trayTitleMu.Lock()
	trayTitleTop, trayTitleBottom, trayTitleIcon = top, bottom, icon
	if trayTitleRetrying {
		// A chain is already running and will pick these values up.
		trayTitleMu.Unlock()
		return
	}
	trayTitleRetrying = true
	trayTitleMu.Unlock()

	time.AfterFunc(trayTitleRetry, func() { applyTrayTitle(1) })
}

// applyTrayTitle pushes the current item onto the status button, retrying while
// no status item exists yet. Failure is non-fatal and logged once: the menu bar
// keeps whatever it is already showing (S05 SPEC §3).
func applyTrayTitle(attempt int) {
	trayTitleMu.Lock()
	top, bottom, icon := trayTitleTop, trayTitleBottom, trayTitleIcon
	key := top + "\x00" + bottom + "\x00" + string(icon)
	unchanged := key == trayTitleDrawn
	trayTitleMu.Unlock()

	if unchanged {
		trayTitleMu.Lock()
		trayTitleRetrying = false
		trayTitleMu.Unlock()
		return
	}

	// No app, no main thread to dispatch onto: application.InvokeSync would
	// dereference a nil global. Happens in tests, and in any run where the
	// status item never comes up.
	if application.Get() == nil {
		if attempt >= trayTitleAttempts {
			trayTitleMu.Lock()
			trayTitleRetrying = false
			trayTitleMu.Unlock()
			return
		}
		time.AfterFunc(trayTitleRetry, func() { applyTrayTitle(attempt + 1) })
		return
	}

	cTop := C.CString(top)
	cBottom := C.CString(bottom)
	var iconPtr unsafe.Pointer
	if len(icon) > 0 {
		// Legal cgo: the bytes hold no Go pointers and the callee does not
		// retain them past the call (it copies into an NSData).
		iconPtr = unsafe.Pointer(&icon[0])
	}
	var set int
	// InvokeSync returns only once the block has run, so the C strings and the
	// icon slice stay alive for the whole native call.
	application.InvokeSync(func() {
		set = int(C.wmSetStatusTitleTwoLine(cTop, cBottom, iconPtr, C.int(len(icon))))
	})
	C.free(unsafe.Pointer(cTop))
	C.free(unsafe.Pointer(cBottom))

	if set > 0 {
		trayTitleMu.Lock()
		trayTitleDrawn = key
		trayTitleRetrying = false
		trayTitleMu.Unlock()
		return
	}
	if attempt >= trayTitleAttempts {
		trayTitleMu.Lock()
		trayTitleRetrying = false
		trayTitleMu.Unlock()
		log.Printf("tray: no status button to draw on after %d attempts", attempt)
		return
	}
	time.AfterFunc(trayTitleRetry, func() { applyTrayTitle(attempt + 1) })
}
