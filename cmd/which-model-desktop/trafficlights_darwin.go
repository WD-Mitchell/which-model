//go:build darwin && !ios

// Go half of the settings window's traffic-light positioning. See
// trafficlights_darwin.m for why AppKit's own placement is not usable here.
package main

/*
#cgo CFLAGS: -mmacosx-version-min=10.14 -x objective-c
#cgo LDFLAGS: -framework Cocoa

int wmPositionTrafficLights(void *nsWindow, double inset);
*/
import "C"

import (
	"log"
	"time"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// trafficLightInset is the gap left of, above and below the button cluster.
//
// It is half of (title row height - button height): the row is 40px
// (SettingsShell.module.css) and AppKit's buttons are 12pt, so 14 centres them
// vertically AND matches the left gap. Changing either value without the other
// reintroduces exactly the uneven padding this exists to fix.
const trafficLightInset = 14.0

// trafficLightSettleDelay covers the window not having its native buttons laid
// out yet on the first call — they exist only once the window has been shown.
const trafficLightSettleDelay = 60 * time.Millisecond

// positionTrafficLights moves the window's standard buttons to an even inset
// and keeps them there across the events that make AppKit re-lay them out.
//
// Failure is logged once and ignored: badly-placed buttons are cosmetic, and a
// settings window that will not open is not.
func positionTrafficLights(w *application.WebviewWindow) {
	if w == nil {
		return
	}

	apply := func() {
		native := w.NativeWindow()
		if native == nil {
			return
		}
		application.InvokeSync(func() {
			if C.wmPositionTrafficLights(unsafe.Pointer(native), C.double(trafficLightInset)) == 0 {
				log.Printf("settings: could not position the window buttons")
			}
		})
	}

	// AppKit restores its own layout on resize and on entering or leaving
	// fullscreen, so re-apply rather than positioning once at creation.
	for _, event := range []events.WindowEventType{
		events.Common.WindowDidResize,
		events.Common.WindowFullscreen,
		events.Common.WindowUnFullscreen,
	} {
		w.OnWindowEvent(event, func(*application.WindowEvent) { apply() })
	}

	time.AfterFunc(trafficLightSettleDelay, apply)
}
