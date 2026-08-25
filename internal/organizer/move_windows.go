//go:build windows

package organizer

import (
	"errors"
	"os"
	"syscall"
)

func isCrossDevice(err error) bool {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		// ERROR_NOT_SAME_DEVICE = 17
		return errno == 17
	}
	return false
}

// Windows error codes for a file another process still holds open.
const (
	errorAccessDenied     = 5
	errorSharingViolation = 32
	errorLockViolation    = 33
)

// isLocked reports whether err means the file is in use by another process.
//
// ERROR_SHARING_VIOLATION does not map to os.ErrPermission in Go, so an
// errors.Is check against os.ErrPermission misses the common case of a browser
// still holding a finished download open.
func isLocked(err error) bool {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		switch errno {
		case errorAccessDenied, errorSharingViolation, errorLockViolation:
			return true
		}
	}
	return errors.Is(err, os.ErrPermission)
}
