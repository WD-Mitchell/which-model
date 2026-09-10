//go:build linux

package company

import "os"

// Linux POSIX access ACL named-user/group write grants are limited by the group
// mask exposed by stat(2). trustOpened rejects that mask's write bit. Policy must
// reside on a local filesystem that enforces this POSIX permission contract.
func checkExtendedACL(*os.File) error { return nil }
