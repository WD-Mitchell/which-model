// S05 — Linux launch-at-login via an autostart .desktop file (S05 CONTRACTS
// §5.3). Reconciles; removal when absent is success.
//go:build linux

package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// loginItemPath is the autostart desktop-file location.
func loginItemPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "autostart", "which-model.desktop"), nil
}

// setLoginItem writes (enabled) or removes (disabled) the autostart desktop
// file. Template is CONTRACTS §5.3 verbatim with %s = execPath.
func setLoginItem(enabled bool, execPath string) error {
	path, err := loginItemPath()
	if err != nil {
		return err
	}
	return writeLoginItem(enabled, execPath, path)
}

// writeLoginItem implements setLoginItem against an explicit artifact path
// (tests pass a temp dir standing in for the real location, CONTRACTS §6).
func writeLoginItem(enabled bool, execPath, path string) error {
	if !enabled {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	content := fmt.Sprintf(loginItemDesktopTemplate, execPath)
	return os.WriteFile(path, []byte(content), 0o644)
}

// loginItemDesktopTemplate is the CONTRACTS §5.3 .desktop template.
const loginItemDesktopTemplate = `[Desktop Entry]
Type=Application
Name=which-model
Exec=%s
X-GNOME-Autostart-enabled=true
`