//go:build windows

package watcher

import "syscall"

// canOpenExclusive reports whether path can be opened with sharing disabled.
//
// A browser writing a download holds the file open, so CreateFile with
// dwShareMode 0 fails with ERROR_SHARING_VIOLATION and the file is not yet safe
// to move. This is the check that catches a download whose size happens to sit
// still across the stability window — a paused transfer, or a slow link.
func canOpenExclusive(path string) bool {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return false
	}
	h, err := syscall.CreateFile(
		p,
		syscall.GENERIC_READ,
		0, // dwShareMode 0: deny read, write and delete to everyone else
		nil,
		syscall.OPEN_EXISTING,
		syscall.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		return false
	}
	syscall.CloseHandle(h)
	return true
}
