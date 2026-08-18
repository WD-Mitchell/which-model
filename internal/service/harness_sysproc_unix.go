//go:build !windows

package service

import "syscall"

// launchSysProcAttr starts the harness as a new session leader (setsid) so
// the detached process is not killed with the app and ignores terminal
// signals (B07 SPEC §2.9.4).
func launchSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}
