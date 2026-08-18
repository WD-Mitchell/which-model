//go:build windows

package service

import "syscall"

// launchSysProcAttr starts the harness in a new process group on Windows so
// it survives the app and ignores console control signals (B07 SPEC §2.9.4).
func launchSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}
