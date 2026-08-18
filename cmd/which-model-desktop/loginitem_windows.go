// S05 — Windows launch-at-login via the HKCU Run registry value (S05
// CONTRACTS §5.2). Enabled = SetStringValue, disabled = DeleteValue.
//go:build windows

package main

import (
	"fmt"

	"golang.org/x/sys/windows/registry"
)

// runRegKey is the HKCU\...\Run key (CONTRACTS §5.2).
const runRegKey = `Software\Microsoft\Windows\CurrentVersion\Run`

// runRegValueName is the value name under runRegKey (CONTRACTS §5.2).
const runRegValueName = "which-model"

// loginItemPath returns the registry value name for reconcile reporting.
func loginItemPath() (string, error) {
	return runRegValueName, nil
}

// setLoginItem writes (enabled) or removes (disabled) the Run value. The
// data is `"%s"` (quoted exec path) per CONTRACTS §5.2.
func setLoginItem(enabled bool, execPath string) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runRegKey, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if !enabled {
		// DeleteValue on an absent value returns ErrNotExist; treat as success
		// (removal when absent is not an error, CONTRACTS §4).
		if err := k.DeleteValue(runRegValueName); err != nil && err != registry.ErrNotExist {
			return err
		}
		return nil
	}
	return k.SetStringValue(runRegValueName, fmt.Sprintf("%q", execPath))
}