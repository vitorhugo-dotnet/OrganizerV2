package watcher

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/vitorhugo-java/organizerv2/internal/config"
	"github.com/vitorhugo-java/organizerv2/internal/notifier"
	"github.com/vitorhugo-java/organizerv2/internal/organizer"
	"github.com/vitorhugo-java/organizerv2/internal/rules"
)

// newTestWatcher builds a Watcher over dir with millisecond-scale timings and
// no live fsnotify watcher, so scheduling can be driven directly.
func newTestWatcher(t *testing.T, dir string) *Watcher {
	t.Helper()
	cfg := config.Default()
	cfg.WatchPaths = []config.WatchPath{{Path: dir, TargetBase: dir}}
	cfg.Stability = config.StabilityConfig{
		DebounceMs:  5,
		WindowMs:    20,
		MinAgeMs:    10,
		MaxChecks:   4,
		MoveRetries: 2,
	}
	clf := rules.NewClassifier(cfg.Rules, cfg.IgnoreExtensions, cfg.FallbackCategory)
	org := organizer.New(cfg, clf, notifier.NoopNotifier{})
	return newWatcher(cfg, org)
}

// entriesIn lists the file names directly inside dir.
func entriesIn(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("readdir %s: %v", dir, err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names
}

// waitIdle blocks until no timers or in-flight checks remain, or the deadline
// passes. It replaces guessing at a fixed sleep.
func waitIdle(t *testing.T, w *Watcher, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		w.mu.Lock()
		idle := len(w.timers) == 0 && len(w.inFlight) == 0
		w.mu.Unlock()
		if idle {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal("watcher did not settle before the timeout")
}

func TestPartialExtensionsIncludeConfigIgnoreList(t *testing.T) {
	w := newTestWatcher(t, t.TempDir())

	// These live in config.IgnoreExtensions but were missing from the old
	// hardcoded list in the watcher, so they reached the stability check.
	for _, name := range []string{"a.partial", "b.aria2", "c.filepart", "d.!qB", "e.downloading"} {
		if !w.isPartial(name) {
			t.Errorf("%s: expected to be treated as a partial download", name)
		}
	}
}

func TestPartialExtensionsKeepBrowserFloor(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Default()
	cfg.WatchPaths = []config.WatchPath{{Path: dir, TargetBase: dir}}
	cfg.IgnoreExtensions = nil // user trimmed the list entirely
	clf := rules.NewClassifier(cfg.Rules, cfg.IgnoreExtensions, cfg.FallbackCategory)
	w := newWatcher(cfg, organizer.New(cfg, clf, notifier.NoopNotifier{}))

	for _, name := range []string{"x.crdownload", "y.part", "z.opdownload"} {
		if !w.isPartial(name) {
			t.Errorf("%s: browser partial extension must be skipped even with an empty ignore list", name)
		}
	}
}

func TestPartialExtensionMatchIsCaseInsensitive(t *testing.T) {
	w := newTestWatcher(t, t.TempDir())
	if !w.isPartial("Installer.CRDOWNLOAD") {
		t.Error("expected .CRDOWNLOAD to match .crdownload")
	}
}

// TestZeroBytePlaceholderIsNotMoved reproduces the reported bug end to end: the
// browser reserves the final name with an empty file while writing the bytes to
// a .crdownload sibling. Neither may leave the watch directory.
func TestZeroBytePlaceholderIsNotMoved(t *testing.T) {
	dir := t.TempDir()
	w := newTestWatcher(t, dir)

	placeholder := filepath.Join(dir, "SonicDesktopRelay-win-x64-0.0.10.exe")
	partial := filepath.Join(dir, "SonicDesktopRelay-win-x64-0.0.10.exe.crdownload")
	writeFile(t, placeholder, 0)
	writeFile(t, partial, 4096)

	w.schedule(placeholder)
	w.schedule(partial)
	waitIdle(t, w, 5*time.Second)

	if _, err := os.Stat(placeholder); err != nil {
		t.Errorf("zero-byte placeholder should have stayed put: %v", err)
	}
	if names := entriesIn(t, filepath.Join(dir, "Executables")); len(names) != 0 {
		t.Errorf("nothing should have been moved, found %v", names)
	}
}

// TestCompletedDownloadIsMovedOnce is the other half of the reported bug: once
// the real file lands it must be moved exactly once, with no " (2)" duplicate.
func TestCompletedDownloadIsMovedOnce(t *testing.T) {
	dir := t.TempDir()
	w := newTestWatcher(t, dir)

	src := filepath.Join(dir, "SonicDesktopRelay-win-x64-0.0.10.exe")
	writeFile(t, src, 8192)
	time.Sleep(15 * time.Millisecond) // age it past minAge

	// Hammer schedule from several goroutines the way a burst of Write events
	// would. Only one move may result.
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w.schedule(src)
		}()
	}
	wg.Wait()
	waitIdle(t, w, 5*time.Second)

	names := entriesIn(t, filepath.Join(dir, "Executables"))
	if len(names) != 1 {
		t.Fatalf("expected exactly one moved file, got %v", names)
	}
	if names[0] != "SonicDesktopRelay-win-x64-0.0.10.exe" {
		t.Errorf("unexpected destination name %q", names[0])
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Errorf("source should be gone, stat returned %v", err)
	}
}

func TestGivesUpAfterMaxChecks(t *testing.T) {
	dir := t.TempDir()
	w := newTestWatcher(t, dir)

	// A file that stays empty forever is never ready, so the watcher must stop
	// rescheduling it instead of retrying without bound.
	stuck := filepath.Join(dir, "stuck.exe")
	writeFile(t, stuck, 0)

	w.schedule(stuck)
	waitIdle(t, w, 5*time.Second)

	w.mu.Lock()
	attempts, hasAttempts := w.attempts[stuck]
	timers := len(w.timers)
	w.mu.Unlock()

	if hasAttempts {
		t.Errorf("attempt state should be cleared after giving up, got %d", attempts)
	}
	if timers != 0 {
		t.Errorf("no timer should remain after giving up, got %d", timers)
	}
	if _, err := os.Stat(stuck); err != nil {
		t.Errorf("the file itself must be left alone: %v", err)
	}
}

func TestPartialFileIsNeverScheduled(t *testing.T) {
	dir := t.TempDir()
	w := newTestWatcher(t, dir)

	partial := filepath.Join(dir, "movie.mkv.part")
	writeFile(t, partial, 4096)

	w.schedule(partial)

	w.mu.Lock()
	timers := len(w.timers)
	w.mu.Unlock()
	if timers != 0 {
		t.Errorf("a partial download must not be queued at all, got %d timer(s)", timers)
	}
}

func TestCategoryDirIsRememberedAfterMove(t *testing.T) {
	dir := t.TempDir()
	w := newTestWatcher(t, dir)

	src := filepath.Join(dir, "report.pdf")
	writeFile(t, src, 1024)
	time.Sleep(15 * time.Millisecond)

	w.schedule(src)
	waitIdle(t, w, 5*time.Second)

	dest := filepath.Join(dir, "Documents", "report.pdf")
	if _, err := os.Stat(dest); err != nil {
		t.Fatalf("expected %s to exist: %v", dest, err)
	}
	// A later event for the moved file must be ignored, or it would loop.
	if !w.isCategoryPath(dest) {
		t.Error("the destination directory should be recognised as a category path")
	}
}

// TestGrowingFileKeepsItsBudget guards against abandoning a long download.
// maxChecks counts consecutive checks with no progress, so a file that keeps
// growing must never exhaust it, however long the transfer runs.
func TestGrowingFileKeepsItsBudget(t *testing.T) {
	dir := t.TempDir()
	w := newTestWatcher(t, dir) // maxChecks is 4, minAge 10ms, window 20ms

	src := filepath.Join(dir, "big.iso")
	writeFile(t, src, 1024)

	// Write in bursts far shorter than minAge, so the file never looks quiet,
	// for long enough to outlast maxChecks several times over.
	const chunks = 100
	var written int
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < chunks; i++ {
			f, err := os.OpenFile(src, os.O_APPEND|os.O_WRONLY, 0o644)
			if err != nil {
				return // the file was moved out from under us; asserted below
			}
			if _, err := f.Write(make([]byte, 1024)); err != nil {
				f.Close()
				return
			}
			f.Close()
			written++
			time.Sleep(2 * time.Millisecond)
		}
	}()

	w.schedule(src)
	<-done
	waitIdle(t, w, 10*time.Second)

	if written != chunks {
		t.Fatalf("the file was moved mid-download after %d of %d chunks", written, chunks)
	}
	names := entriesIn(t, filepath.Join(dir, "ISO"))
	if len(names) != 1 || names[0] != "big.iso" {
		t.Fatalf("the finished download should have been moved once, found %v", names)
	}
	info, err := os.Stat(filepath.Join(dir, "ISO", "big.iso"))
	if err != nil {
		t.Fatalf("stat destination: %v", err)
	}
	if want := int64(1024 * (chunks + 1)); info.Size() != want {
		t.Errorf("expected the complete file (%d bytes), got %d", want, info.Size())
	}
}

// TestStalledFileStillGivesUp confirms the budget still bounds a file that is
// never going to finish.
func TestStalledFileStillGivesUp(t *testing.T) {
	dir := t.TempDir()
	w := newTestWatcher(t, dir)

	// Non-empty but permanently locked-looking: mtime keeps moving, size never
	// does, so it is never ready and never makes progress.
	src := filepath.Join(dir, "stalled.zip")
	writeFile(t, src, 2048)

	stop := make(chan struct{})
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
				now := time.Now()
				_ = os.Chtimes(src, now, now)
				time.Sleep(5 * time.Millisecond)
			}
		}
	}()
	defer close(stop)

	w.schedule(src)
	waitIdle(t, w, 5*time.Second)

	if _, err := os.Stat(src); err != nil {
		t.Errorf("a stalled file must be left in place: %v", err)
	}
	if names := entriesIn(t, filepath.Join(dir, "Compacted")); len(names) != 0 {
		t.Errorf("nothing should have been moved, found %v", names)
	}
}

// TestLockedMoveGivesUpAfterMoveRetries covers the move-retry budget. The
// readiness budget and the move budget are separate counters: an earlier
// version cleared the shared counter before every move attempt, so a file that
// could never be moved was retried forever.
func TestLockedMoveGivesUpAfterMoveRetries(t *testing.T) {
	dir := t.TempDir()
	w := newTestWatcher(t, dir) // moveRetries is 2

	src := filepath.Join(dir, "locked.jpg")
	writeFile(t, src, 1024)
	lockFile(t, src) // rename now fails with a "file in use" error
	time.Sleep(15 * time.Millisecond)

	w.schedule(src)
	waitIdle(t, w, 10*time.Second)

	w.mu.Lock()
	moveAttempts, tracked := w.moveAttempts[src]
	timers := len(w.timers)
	w.mu.Unlock()

	if tracked {
		t.Errorf("move state should be cleared after giving up, got %d attempts", moveAttempts)
	}
	if timers != 0 {
		t.Errorf("no timer should remain after giving up, got %d", timers)
	}
	if _, err := os.Stat(src); err != nil {
		t.Errorf("the source must be left in place after a failed move: %v", err)
	}
}

// TestLockedMoveSucceedsWhenReleased proves the retry actually retries: the
// file is unlocked while the watcher is backing off, and must then be moved.
func TestLockedMoveSucceedsWhenReleased(t *testing.T) {
	dir := t.TempDir()
	w := newTestWatcher(t, dir)

	src := filepath.Join(dir, "released.jpg")
	writeFile(t, src, 1024)
	lockFile(t, src)
	time.Sleep(15 * time.Millisecond)

	// Release the lock after the first move attempt has already failed.
	go func() {
		time.Sleep(60 * time.Millisecond)
		unlockFile(src)
	}()

	w.schedule(src)
	waitIdle(t, w, 10*time.Second)

	names := entriesIn(t, filepath.Join(dir, "Image"))
	if len(names) != 1 || names[0] != "released.jpg" {
		t.Fatalf("the file should have been moved once the lock cleared, found %v", names)
	}
}
