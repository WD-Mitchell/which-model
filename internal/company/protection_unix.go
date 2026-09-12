//go:build darwin || linux

package company

import (
	"os"
	"runtime"
	"syscall"
)

func policyDirectory() (string, error) {
	if runtime.GOOS == "darwin" {
		return "/Library/Application Support/which-model/managed", nil
	}
	return "/etc/which-model/managed", nil
}

func isRedirect(info os.FileInfo) bool { return info.Mode()&os.ModeSymlink != 0 }

func trustOpened(file *os.File, directory bool) error {
	info, err := file.Stat()
	if err != nil {
		return err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != 0 || info.Mode().Perm()&0o022 != 0 || info.IsDir() != directory {
		return errProtection
	}
	return checkExtendedACL(file)
}
