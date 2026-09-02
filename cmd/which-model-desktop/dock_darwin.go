//go:build darwin && !ios

// Go half of dynamic Dock icon management. On macOS, the app runs with
// ActivationPolicyAccessory by default (no Dock icon, menu-bar app).
// When the Settings window is shown, it transitions to
// NSApplicationActivationPolicyRegular so that a Dock icon appears, Cmd-Tab
// window cycling includes it, and clicking the Dock icon brings Settings to
// the front. When Settings is hidden, it transitions back to Accessory.
package main

/*
#cgo CFLAGS: -mmacosx-version-min=10.14 -x objective-c
#cgo LDFLAGS: -framework Cocoa

void wmSetDockIconVisible(int visible);
int  wmGetDockIconVisible(void);
*/
import "C"

// setDockIconVisible toggles between Regular (visible=true, with Dock icon)
// and Accessory (visible=false, menu-bar only) activation policy.
func setDockIconVisible(visible bool) {
	val := 0
	if visible {
		val = 1
	}
	C.wmSetDockIconVisible(C.int(val))
}

// dockIconVisible reports whether the activation policy is currently Regular.
func dockIconVisible() bool {
	return C.wmGetDockIconVisible() != 0
}
