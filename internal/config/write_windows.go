//go:build windows

package config

import "golang.org/x/sys/windows"

func replaceFile(from, to string) error {
	source, err := windows.UTF16PtrFromString(from)
	if err != nil {
		return err
	}
	target, err := windows.UTF16PtrFromString(to)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(source, target, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
}

// Windows cannot FlushFileBuffers on the read-only directory handle used by
// os.File.Sync. The staged file is flushed and replacement requests write-through;
// do not report a false post-commit failure for unsupported directory fsync.
func platformSyncDirectory(string) error { return nil }
