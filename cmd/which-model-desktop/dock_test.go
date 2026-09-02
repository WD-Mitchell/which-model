package main

import (
	"testing"
)

func TestSetDockIconVisible(t *testing.T) {
	// Should not panic even if NSApp is nil (e.g. running in unit test runner).
	setDockIconVisible(true)
	setDockIconVisible(false)
	setDockIconVisible(false) // idempotent
	setDockIconVisible(true)
	setDockIconVisible(false)
}

func TestHideSettingsNilSafe(t *testing.T) {
	// hideSettings before ensureSettingsWindow must be a safe no-op.
	hideSettings()
}
