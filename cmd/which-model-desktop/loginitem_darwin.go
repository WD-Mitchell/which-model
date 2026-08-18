// S05 — macOS launch-at-login via a LaunchAgent plist (S05 CONTRACTS §5.1).
// Reconciles the artifact to match `enabled`; removal when absent is success.
//go:build darwin

package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// loginItemPath is the LaunchAgent plist location.
func loginItemPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", "com.wdmitchell.which-model.plist"), nil
}

// setLoginItem writes (enabled) or removes (disabled) the LaunchAgent plist.
// The plist template is CONTRACTS §5.1 verbatim with %s = execPath.
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
	plist := fmt.Sprintf(loginItemPlistTemplate, execPath)
	return os.WriteFile(path, []byte(plist), 0o644)
}

// loginItemPlistTemplate is the CONTRACTS §5.1 plist (verbatim, %s = execPath).
const loginItemPlistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>com.wdmitchell.which-model</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
</dict>
</plist>
`