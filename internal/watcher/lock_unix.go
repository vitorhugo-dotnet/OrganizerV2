//go:build !windows

package watcher

import "os"

// canOpenExclusive reports whether path is readable.
//
// Unix has no mandatory locking equivalent to the Windows share mode, and
// renaming a file another process holds open is safe here, so a plain open is
// the strongest portable check available. It still catches a file whose
// permissions or parent directory changed under us.
func canOpenExclusive(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	f.Close()
	return true
}
