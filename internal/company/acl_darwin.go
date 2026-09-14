//go:build darwin

package company

import (
	"encoding/binary"
	"os"
	"runtime"
	"syscall"
	"unsafe"
)

// macOS ACL grants are independent of POSIX mode bits. Require ACL-free policy
// files/parents rather than infer effective rights from an incomplete ACL model.
// ATTR_CMN_EXTENDED_SECURITY returns an attrreference to kauth_filesec; a zero
// reference length means no ACL. The syscall avoids launching a helper or CGO.
// ABI: Apple's xnu bsd/man/man2/getattrlist.2 and bsd/sys/attr.h.
func checkExtendedACL(file *os.File) error {
	attributes := struct {
		Count, Reserved                       uint16
		Common, Volume, Directory, File, Fork uint32
	}{Count: 5, Common: 0x00400000}
	buffer := make([]byte, 65536)
	_, _, errno := syscall.Syscall6(syscall.SYS_FGETATTRLIST, file.Fd(), uintptr(unsafe.Pointer(&attributes)), uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)), 0, 0)
	runtime.KeepAlive(file)
	if errno != 0 {
		return errProtection
	}
	length := binary.LittleEndian.Uint32(buffer[:4])
	if length < 12 || length > uint32(len(buffer)) {
		return errProtection
	}
	if binary.LittleEndian.Uint32(buffer[8:12]) != 0 {
		return errProtection
	}
	return nil
}
