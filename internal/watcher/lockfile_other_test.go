//go:build !linux

package watcher

import "testing"

// lockFile has no portable equivalent outside Linux, so the move-retry tests
// are skipped there. The retry logic itself is platform independent.
func lockFile(t *testing.T, path string) {
	t.Helper()
	t.Skip("no portable way to lock a file on this platform")
}

func unlockFile(string) {}
