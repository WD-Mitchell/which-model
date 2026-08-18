// S05 — darwin hotkey modifiers. hotkey.ModOption (alt), ModCmd (cmd/lower
// right), ModCtrl, ModShift. `cmd+shift+m` → Cmd+Shift+M.
//go:build darwin

package main

import "golang.design/x/hotkey"

func altSpaceModifiers() []hotkey.Modifier {
	return []hotkey.Modifier{hotkey.ModOption}
}

func cmdShiftMModifiers() []hotkey.Modifier {
	return []hotkey.Modifier{hotkey.ModCmd, hotkey.ModShift}
}