package security

import (
	"errors"
	"io/fs"
	"os"
)

// ReadBoundedFile ports readBoundedFile (core.mjs:37-60). The file must be a
// regular file whose size is >= 1 and <= maxBytes (checked via stat, then
// re-checked on the actually-read bytes); the bytes and the file's mode are
// returned so the caller can warn on broad permissions. Missing/non-regular
// -> Error{Code:"credential_file", Message:"The credential file was not found."}
// Size violations -> Error{Code:"credential_file",
// Message:"The credential file has an invalid size."}  Any other I/O failure
// -> Error{Code:"credential_file",
// Message:"The credential file could not be read safely."}  Underlying error
// details are never surfaced. Never modifies the file.
func ReadBoundedFile(path string, maxBytes int64) ([]byte, fs.FileMode, error) {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, 0, &Error{Code: "credential_file", Message: "The credential file was not found."}
		}
		return nil, 0, &Error{Code: "credential_file", Message: "The credential file could not be read safely."}
	}
	if !info.Mode().IsRegular() {
		return nil, 0, &Error{Code: "credential_file", Message: "The credential file was not found."}
	}
	if info.Size() < 1 || info.Size() > maxBytes {
		return nil, 0, &Error{Code: "credential_file", Message: "The credential file has an invalid size."}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, &Error{Code: "credential_file", Message: "The credential file could not be read safely."}
	}
	if int64(len(data)) > maxBytes {
		return nil, 0, &Error{Code: "credential_file", Message: "The credential file has an invalid size."}
	}
	return data, info.Mode(), nil
}

// HasBroadPermissions ports hasBroadPermissions (core.mjs:72-74): true when
// any group or other rwx bit is set, i.e. mode.Perm() & 0o077 != 0.
// The caller warns; nothing here ever chmod's the file.
func HasBroadPermissions(mode fs.FileMode) bool { return mode.Perm()&0o077 != 0 }
