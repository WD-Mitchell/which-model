// S05 CONTRACTS §6 — parseShortcut table test. Modifier expectations are
// platform-dependent (the modifier slices come from the build-tagged
// modifiers_*.go files), so the test asserts the platform-agnostic parts: the
// key, the ok flag, and Shift presence for cmd+shift+m — plus that every
// mapping yields a non-empty modifier set.
package main

import (
	"testing"

	"golang.design/x/hotkey"
)

var (
	wantKeySpace = hotkey.KeySpace
	wantKeyM     = hotkey.KeyM
)

func TestParseShortcut(t *testing.T) {
	tests := []struct {
		in        string
		wantKey   hotkey.Key
		wantOK    bool
		wantShift bool // only asserted for cmd+shift+m
	}{
		{"alt+space", wantKeySpace, true, false},
		{"ctrl+space", wantKeySpace, true, false},
		{"cmd+shift+m", wantKeyM, true, true},
		{"unknown", wantKeySpace, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			mods, key, ok := parseShortcut(tt.in)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if key != tt.wantKey {
				t.Fatalf("key = %v, want %v", key, tt.wantKey)
			}
			if len(mods) == 0 {
				t.Fatalf("mods empty for %q", tt.in)
			}
			if tt.wantShift {
				hasShift := false
				for _, m := range mods {
					if m == hotkey.ModShift {
						hasShift = true
					}
				}
				if !hasShift {
					t.Fatalf("cmd+shift+m missing ModShift: %v", mods)
				}
			}
		})
	}
}