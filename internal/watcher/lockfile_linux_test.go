//go:build linux

package watcher

import (
	"os/exec"
	"testing"
)

// lockFile makes path immutable so os.Rename fails with EPERM, which isLocked
// treats as "held by another process". This stands in for the Windows sharing
// violation a browser causes while it still holds a finished download open,
// and unlike a read-only parent directory it also works when running as root.
func lockFile(t *testing.T, path string) {
	t.Helper()
	if _, err := exec.LookPath("chattr"); err != nil {
		t.Skip("chattr not available; cannot simulate a locked file")
	}
	if out, err := exec.Command("chattr", "+i", path).CombinedOutput(); err != nil {
		t.Skipf("chattr +i failed (needs CAP_LINUX_IMMUTABLE): %v: %s", err, out)
	}
	t.Cleanup(func() { unlockFile(path) })
}

func unlockFile(path string) {
	_ = exec.Command("chattr", "-i", path).Run()
}
