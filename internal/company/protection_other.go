//go:build !darwin && !linux && !windows

package company

import "os"

// Managed deployments support the three documented OSes. Personal behavior on
// other targets remains available because no machine policy origin is defined.
func policyDirectory() (string, error) { return "", nil }
func isRedirect(info os.FileInfo) bool { return info.Mode()&os.ModeSymlink != 0 }
func trustOpened(*os.File, bool) error { return errProtection }
