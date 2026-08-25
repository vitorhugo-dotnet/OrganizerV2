//go:build !windows

package organizer

import (
	"errors"
	"os"
	"syscall"
)

func isCrossDevice(err error) bool {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		return errno == syscall.EXDEV
	}
	return false
}

// isLocked reports whether err means the file is in use by another process.
// Unix has no mandatory locking, but a running binary (ETXTBSY) or a busy mount
// point (EBUSY) still refuses a rename, and both clear on their own.
func isLocked(err error) bool {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		switch errno {
		case syscall.EACCES, syscall.EBUSY, syscall.ETXTBSY, syscall.EPERM:
			return true
		}
	}
	return errors.Is(err, os.ErrPermission)
}
