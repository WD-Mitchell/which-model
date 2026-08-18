// S05 — linux (x11) hotkey modifiers. hotkey.Mod1 (=Alt), ModCtrl, ModShift;
// `cmd+shift+m` → Mod4 (Super)+Shift+M per S00 CONTRACTS §5.
//go:build linux

package main

import "golang.design/x/hotkey"

func altSpaceModifiers() []hotkey.Modifier {
	return []hotkey.Modifier{hotkey.Mod1}
}

func cmdShiftMModifiers() []hotkey.Modifier {
	return []hotkey.Modifier{hotkey.Mod4, hotkey.ModShift}
}