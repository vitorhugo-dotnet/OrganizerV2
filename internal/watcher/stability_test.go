package watcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Small durations keep the suite fast; production defaults are 2s.
const (
	testWindow = 30 * time.Millisecond
	testMinAge = 20 * time.Millisecond
)

func writeFile(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

// TestZeroBytePlaceholderIsNotReady is the regression test for the reported bug:
// browsers create a zero-byte file to reserve the final download name while the
// real bytes go to a .crdownload sibling. Comparing only size saw 0 == 0 across
// the window and declared the placeholder complete.
func TestZeroBytePlaceholderIsNotReady(t *testing.T) {
	path := filepath.Join(t.TempDir(), "SonicDesktopRelay-win-x64-0.0.10.exe")
	writeFile(t, path, 0)

	// Age the placeholder well past minAge so only the size guard can reject it.
	time.Sleep(2 * testMinAge)

	if got, _ := checkReady(path, testMinAge, testWindow); got != notReady {
		t.Fatalf("zero-byte placeholder: got %v, want notReady", got)
	}
}

func TestGrowingFileIsNotReady(t *testing.T) {
	path := filepath.Join(t.TempDir(), "download.zip")
	writeFile(t, path, 1024)
	time.Sleep(2 * testMinAge)

	// Grow the file while checkReady is sleeping through its window.
	go func() {
		time.Sleep(testWindow / 2)
		f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return
		}
		defer f.Close()
		_, _ = f.Write(make([]byte, 2048))
	}()

	if got, _ := checkReady(path, testMinAge, testWindow); got != notReady {
		t.Fatalf("growing file: got %v, want notReady", got)
	}
}

func TestFreshlyWrittenFileIsNotReady(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fresh.pdf")
	writeFile(t, path, 512)

	// No sleep: the write just happened, so it is younger than minAge.
	if got, _ := checkReady(path, time.Hour, testWindow); got != notReady {
		t.Fatalf("freshly written file: got %v, want notReady", got)
	}
}

func TestStableFileIsReady(t *testing.T) {
	path := filepath.Join(t.TempDir(), "complete.exe")
	writeFile(t, path, 4096)
	time.Sleep(2 * testMinAge)

	if got, _ := checkReady(path, testMinAge, testWindow); got != ready {
		t.Fatalf("stable file: got %v, want ready", got)
	}
}

func TestMissingFileIsGone(t *testing.T) {
	path := filepath.Join(t.TempDir(), "never-existed.txt")

	if got, _ := checkReady(path, testMinAge, testWindow); got != gone {
		t.Fatalf("missing file: got %v, want gone", got)
	}
}

func TestDirectoryIsGone(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "subdir")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	time.Sleep(2 * testMinAge)

	if got, _ := checkReady(dir, testMinAge, testWindow); got != gone {
		t.Fatalf("directory: got %v, want gone", got)
	}
}

// TestFileRemovedDuringWindow covers the file vanishing between the two samples.
func TestFileRemovedDuringWindow(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vanishing.iso")
	writeFile(t, path, 1024)
	time.Sleep(2 * testMinAge)

	go func() {
		time.Sleep(testWindow / 2)
		_ = os.Remove(path)
	}()

	if got, _ := checkReady(path, testMinAge, testWindow); got != gone {
		t.Fatalf("removed during window: got %v, want gone", got)
	}
}
