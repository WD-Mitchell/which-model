package company

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const MaxInstallationBytes int64 = 1 << 30

// VerifyInstallation applies the protected-policy ownership/ACL boundary to an
// approved image or explicit input. No provider credentials or process are used.
// Ordinary users cannot replace the file or its ancestors between proof and use;
// administrators/software deployment remain part of the trusted endpoint boundary.
func VerifyInstallation(in Installation, executable bool) error {
	if err := verifyInstallation(in, executable, trustOpened); err != nil {
		return &Error{Reason: "approved installation is absent, changed, redirected or insufficiently protected"}
	}
	return nil
}
func verifyInstallation(in Installation, executable bool, trust func(*os.File, bool) error) error {
	if !filepath.IsAbs(in.Path) || filepath.Clean(in.Path) != in.Path || !digestPattern.MatchString(in.SHA256) {
		return errProtection
	}
	for _, parent := range parentPaths(in.Path) {
		info, err := os.Lstat(parent)
		if err != nil || !info.IsDir() || isRedirect(info) {
			return errProtection
		}
		f, err := os.Open(parent)
		if err != nil {
			return errProtection
		}
		opened, e := f.Stat()
		if e == nil && !os.SameFile(info, opened) {
			e = errProtection
		}
		if e == nil {
			e = trust(f, true)
		}
		f.Close()
		if e != nil {
			return errProtection
		}
	}
	info, err := os.Lstat(in.Path)
	if err != nil || !info.Mode().IsRegular() || isRedirect(info) || (executable && info.Size() <= 0) || info.Size() > MaxInstallationBytes {
		return errProtection
	}
	if executable && ((runtime.GOOS != "windows" && info.Mode().Perm()&0111 == 0) || (runtime.GOOS == "windows" && !strings.EqualFold(filepath.Ext(in.Path), ".exe"))) {
		return errProtection
	}
	f, err := os.Open(in.Path)
	if err != nil {
		return errProtection
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) || trust(f, false) != nil {
		return errProtection
	}
	if executable {
		header := make([]byte, 4)
		if _, err := io.ReadFull(f, header); err != nil || !nativeImage(header) {
			return errProtection
		}
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return errProtection
	}
	hash := sha256.New()
	n, err := io.Copy(hash, io.LimitReader(f, MaxInstallationBytes+1))
	if err != nil || n != opened.Size() || n > MaxInstallationBytes || hex.EncodeToString(hash.Sum(nil)) != in.SHA256 {
		return errProtection
	}
	after, err := f.Stat()
	if err != nil {
		return errProtection
	}
	current, err := os.Lstat(in.Path)
	if err != nil || isRedirect(current) || !os.SameFile(opened, current) || after.Size() != opened.Size() || !after.ModTime().Equal(opened.ModTime()) || trust(f, false) != nil {
		return errProtection
	}
	return nil
}
func nativeImage(header []byte) bool {
	switch runtime.GOOS {
	case "windows":
		return bytes.HasPrefix(header, []byte("MZ"))
	case "linux":
		return bytes.Equal(header, []byte{0x7f, 'E', 'L', 'F'})
	case "darwin":
		for _, magic := range [][]byte{{0xfe, 0xed, 0xfa, 0xce}, {0xce, 0xfa, 0xed, 0xfe}, {0xfe, 0xed, 0xfa, 0xcf}, {0xcf, 0xfa, 0xed, 0xfe}, {0xca, 0xfe, 0xba, 0xbe}, {0xbe, 0xba, 0xfe, 0xca}, {0xca, 0xfe, 0xba, 0xbf}, {0xbf, 0xba, 0xfe, 0xca}} {
			if bytes.Equal(header, magic) {
				return true
			}
		}
	}
	return false
}
