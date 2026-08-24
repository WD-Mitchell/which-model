// S05 — windows hotkey modifiers. hotkey.ModAlt, ModCtrl, ModShift, ModWin.
// `cmd+shift+m` → Win+Shift+M.
//go:build windows

package main

import "golang.design/x/hotkey"

func altSpaceModifiers() []hotkey.Modifier {
	return []hotkey.Modifier{hotkey.ModAlt}
}

func cmdShiftMModifiers() []hotkey.Modifier {
	return []hotkey.Modifier{hotkey.ModWin, hotkey.ModShift}
}