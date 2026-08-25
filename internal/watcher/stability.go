package watcher

import (
	"os"
	"time"
)

// readyState is the verdict of a readiness check on a single path.
type readyState int

const (
	// ready means the file stopped changing and nobody else holds it open.
	ready readyState = iota
	// notReady means the file is still being written, is too young, or is
	// locked by another process. The caller should try again later.
	notReady
	// gone means the path vanished or is not a regular file. Stop tracking it.
	gone
)

func (r readyState) String() string {
	switch r {
	case ready:
		return "ready"
	case notReady:
		return "notReady"
	case gone:
		return "gone"
	}
	return "unknown"
}

// checkReady reports whether path is safe to move.
//
// A file qualifies only when every one of these holds:
//
//   - it exists and is a regular file;
//   - it is not empty. Browsers create a zero-byte file to reserve the final
//     download name while the bytes go to a .crdownload sibling, and a size-only
//     comparison reads 0 == 0 across the window and calls that placeholder
//     complete. That is the bug this guard exists for;
//   - its last write is at least minAge old;
//   - its size and mtime are identical across two samples window apart. Size
//     alone misses a rewrite in place that keeps the length;
//   - it can be opened with no sharing allowed, proving no other process still
//     holds a handle (Windows; see canOpenExclusive).
//
// It returns the last observed size alongside the verdict so the caller can
// tell a growing file (making progress, keep waiting) from a stalled one.
//
// checkReady blocks for window. Callers run it off the event loop.
//
// Trade-off: a legitimately empty file is never ready, so the watcher retries
// until it gives up and logs. Use `organizer scan` to place such files, which
// goes straight to ProcessFile without the watcher.
func checkReady(path string, minAge, window time.Duration) (readyState, int64) {
	info1, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return gone, 0
		}
		return notReady, 0
	}
	if !info1.Mode().IsRegular() {
		return gone, 0
	}
	if info1.Size() == 0 {
		return notReady, 0
	}
	if time.Since(info1.ModTime()) < minAge {
		return notReady, info1.Size()
	}

	time.Sleep(window)

	info2, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return gone, 0
		}
		return notReady, info1.Size()
	}
	if info1.Size() != info2.Size() || !info1.ModTime().Equal(info2.ModTime()) {
		return notReady, info2.Size()
	}
	if !canOpenExclusive(path) {
		return notReady, info2.Size()
	}
	return ready, info2.Size()
}
