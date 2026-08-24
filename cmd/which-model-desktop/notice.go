// S05 — host:notice. A shell-local event (Deviation from D00 §3; see S05
// CONTRACTS §2) carrying transient toast messages for non-fatal host
// failures. Emitted only by the shell process, never by internal/service.
package main

import "github.com/wailsapp/wails/v3/pkg/application"

// hostNoticeEvent is the shell-local transient-toast event name (S05
// CONTRACTS §2). It is intentionally NOT in internal/service/events.go or the
// D00 §3 enum.
const hostNoticeEvent = "host:notice"

// notice emits a host:notice toast payload for the frontend. A nil app
// (before app creation) or nil Event manager is a silent no-op — notice
// failures are non-fatal by design (S05 SPEC §3).
func notice(app *application.App, message string) {
	if app == nil || app.Event == nil {
		return
	}
	app.Event.Emit(hostNoticeEvent, map[string]string{"message": message})
}