//go:build windows

package company

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

func policyDirectory() (string, error) {
	path, err := windows.KnownFolderPath(windows.FOLDERID_ProgramData, 0)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(path, `\\`) {
		return "", errProtection
	}
	return filepath.Join(path, "which-model", "managed"), nil
}

func isRedirect(info os.FileInfo) bool {
	if info.Mode()&os.ModeSymlink != 0 {
		return true
	}
	data, ok := info.Sys().(*syscall.Win32FileAttributeData)
	return !ok || data.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0
}

func trustedSID(sid *windows.SID) bool {
	if sid == nil {
		return false
	}
	if sid.IsWellKnown(windows.WinLocalSystemSid) || sid.IsWellKnown(windows.WinBuiltinAdministratorsSid) {
		return true
	}
	// Windows servicing owns some system ancestors through TrustedInstaller.
	return sid.String() == "S-1-5-80-956008885-3418522649-1831038044-1853292631-2271478464"
}

func trustDescriptor(sd *windows.SECURITY_DESCRIPTOR, directory bool) error {
	if sd == nil {
		return errProtection
	}
	owner, _, err := sd.Owner()
	if err != nil || !trustedSID(owner) {
		return errProtection
	}
	acl, _, err := sd.DACL()
	if err != nil || acl == nil {
		return errProtection
	}
	// Creating a sibling in a system ancestor cannot replace an existing protected
	// child. Delete-child, delete, ACL/owner changes and generic write are forbidden.
	unsafeMask := uint32(windows.DELETE | windows.WRITE_DAC | windows.WRITE_OWNER | windows.GENERIC_ALL | windows.GENERIC_WRITE)
	if directory {
		// ProgramData grants users directory attribute/EA updates as well as
		// sibling creation. These do not modify a protected child. Reparse points
		// cannot be added to nonempty directories; each existing next component
		// is independently protected against deletion/replacement below.
		unsafeMask |= 0x0040
	} else {
		unsafeMask |= windows.FILE_WRITE_DATA | windows.FILE_APPEND_DATA | windows.FILE_WRITE_ATTRIBUTES | windows.FILE_WRITE_EA
	}
	for i := uint32(0); i < uint32(acl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(acl, i, &ace); err != nil || ace == nil {
			return errProtection
		}
		header := &ace.Header
		if header.AceFlags&windows.INHERIT_ONLY_ACE != 0 {
			continue
		}
		if header.AceType == windows.ACCESS_DENIED_ACE_TYPE {
			continue
		}
		if header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE || header.AceSize < uint16(unsafe.Sizeof(windows.ACCESS_ALLOWED_ACE{})) {
			return errProtection
		}
		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		if uint32(ace.Mask)&unsafeMask != 0 && !trustedSID(sid) {
			return errProtection
		}
	}
	runtime.KeepAlive(sd)
	return nil
}

func trustOpened(file *os.File, directory bool) error {
	info, err := file.Stat()
	if err != nil || info.IsDir() != directory || isRedirect(info) {
		return errProtection
	}
	sd, err := windows.GetSecurityInfo(windows.Handle(file.Fd()), windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return errProtection
	}
	return trustDescriptor(sd, directory)
}
