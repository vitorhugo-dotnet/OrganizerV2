package watcher

import (
	"context"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/vitorhugo-java/organizerv2/internal/config"
	"github.com/vitorhugo-java/organizerv2/internal/organizer"
)

// browserPartialExts are extensions browsers and download managers use for
// in-progress transfers. They are a floor: config.IgnoreExtensions is unioned on
// top, but trimming that list must never expose a .crdownload to the organizer.
var browserPartialExts = []string{
	".crdownload", // Chrome, Edge
	".part",       // Firefox
	".download",   // Safari and others
	".opdownload", // Opera
	".partial",    // Internet Explorer, some managers
	".tmp",
}

// Watcher watches configured directories for new files and triggers the
// organizer once each file is finished being written.
type Watcher struct {
	fsw *fsnotify.Watcher
	org *organizer.Organizer
	cfg *config.Config

	debounce    time.Duration
	window      time.Duration
	minAge      time.Duration
	maxChecks   int
	moveRetries int

	partialExts map[string]struct{}

	// mu guards the three maps below, which together track one path's progress
	// from first event to final move.
	mu sync.Mutex
	// timers holds the pending debounce or retry timer for a path.
	timers map[string]*time.Timer
	// inFlight marks paths whose readiness check is running right now. Without
	// it, an event arriving during the check's blocking window would start a
	// second check for the same path and both could move it.
	inFlight map[string]struct{}
	// attempts counts *consecutive fruitless* readiness checks and move retries
	// per path. It resets whenever the file grows, so a slow download is waited
	// on for as long as it keeps making progress and only a genuinely stalled
	// file runs out of budget.
	attempts map[string]int
	// lastSize is the size seen by the previous check, used to detect progress.
	lastSize map[string]int64
	// moveAttempts counts move retries per path. It is deliberately separate
	// from attempts: readiness checks and lock retries have different budgets,
	// and a file that needed many readiness checks still deserves a full set of
	// move retries.
	moveAttempts map[string]int

	// catMu guards categoryDirs. It is separate from mu because isCategoryPath
	// is called while mu is held.
	catMu sync.Mutex
	// categoryDirs is the set of category subdirectories we created. Events for
	// paths inside them are ignored to prevent reprocessing loops.
	categoryDirs map[string]struct{}
}

// New creates a Watcher for all paths in cfg.WatchPaths.
func New(cfg *config.Config, org *organizer.Organizer) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	for _, wp := range cfg.WatchPaths {
		if err := fsw.Add(wp.Path); err != nil {
			fsw.Close()
			return nil, err
		}
		log.Printf("[watcher] watching %s", wp.Path)
	}
	w := newWatcher(cfg, org)
	w.fsw = fsw
	return w, nil
}

// newWatcher builds the Watcher state without touching the filesystem, so the
// scheduling logic can be tested without a live fsnotify watcher.
func newWatcher(cfg *config.Config, org *organizer.Organizer) *Watcher {
	partial := make(map[string]struct{}, len(browserPartialExts)+len(cfg.IgnoreExtensions))
	for _, ext := range browserPartialExts {
		partial[ext] = struct{}{}
	}
	for _, ext := range cfg.IgnoreExtensions {
		partial[strings.ToLower(ext)] = struct{}{}
	}
	return &Watcher{
		org:          org,
		cfg:          cfg,
		debounce:     time.Duration(cfg.Stability.DebounceMs) * time.Millisecond,
		window:       time.Duration(cfg.Stability.WindowMs) * time.Millisecond,
		minAge:       time.Duration(cfg.Stability.MinAgeMs) * time.Millisecond,
		maxChecks:    cfg.Stability.MaxChecks,
		moveRetries:  cfg.Stability.MoveRetries,
		partialExts:  partial,
		timers:       make(map[string]*time.Timer),
		inFlight:     make(map[string]struct{}),
		attempts:     make(map[string]int),
		lastSize:     make(map[string]int64),
		moveAttempts: make(map[string]int),
		categoryDirs: make(map[string]struct{}),
	}
}

// Start begins watching and blocks until ctx is cancelled.
func (w *Watcher) Start(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-w.fsw.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Create) || event.Has(fsnotify.Write) {
				w.schedule(event.Name)
			}
		case err, ok := <-w.fsw.Errors:
			if !ok {
				return
			}
			log.Printf("[watcher] error: %v", err)
		}
	}
}

// Stop closes the underlying fsnotify watcher.
func (w *Watcher) Stop() {
	if w.fsw != nil {
		w.fsw.Close()
	}
}

// isPartial reports whether path carries an in-progress download extension.
func (w *Watcher) isPartial(path string) bool {
	_, ok := w.partialExts[strings.ToLower(filepath.Ext(path))]
	return ok
}

// schedule queues path for a readiness check after the debounce delay.
func (w *Watcher) schedule(path string) {
	// Skip files inside category subdirectories to prevent reprocessing loops.
	if w.isCategoryPath(path) {
		return
	}
	// Ignore partial-download extensions outright; the browser renames the file
	// once the transfer finishes, which raises a fresh event under the real name.
	if w.isPartial(path) {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	if _, running := w.inFlight[path]; running {
		// A check is already blocking on this path. It will decide the outcome.
		return
	}
	if t, ok := w.timers[path]; ok {
		t.Reset(w.debounce)
		return
	}
	w.timers[path] = time.AfterFunc(w.debounce, func() { w.run(path) })
}

// run performs one readiness check for path and either moves it, schedules
// another check, or drops it.
func (w *Watcher) run(path string) {
	w.mu.Lock()
	delete(w.timers, path)
	w.inFlight[path] = struct{}{}
	attempts := w.attempts[path]
	w.mu.Unlock()

	defer func() {
		w.mu.Lock()
		delete(w.inFlight, path)
		w.mu.Unlock()
	}()

	state, size := checkReady(path, w.minAge, w.window)
	switch state {
	case gone:
		w.forget(path)
	case notReady:
		// Growing means the transfer is alive: give the file a fresh budget so a
		// long download is never abandoned midway.
		w.mu.Lock()
		if size != w.lastSize[path] {
			attempts = 0
		}
		w.lastSize[path] = size
		w.mu.Unlock()
		w.retry(path, attempts, w.window, "still being written")
	case ready:
		// Clear only the readiness budget. process owns the move-retry budget;
		// clearing that here would reset it every cycle and retry forever.
		w.mu.Lock()
		delete(w.attempts, path)
		delete(w.lastSize, path)
		w.mu.Unlock()
		w.process(path)
	}
}

// retry re-arms a check for path unless it has already used up maxChecks.
func (w *Watcher) retry(path string, attempts int, delay time.Duration, reason string) {
	if attempts+1 >= w.maxChecks {
		log.Printf("[watcher] giving up on %s after %d checks with no progress: %s",
			filepath.Base(path), w.maxChecks, reason)
		w.forget(path)
		return
	}
	w.mu.Lock()
	w.attempts[path] = attempts + 1
	w.timers[path] = time.AfterFunc(delay, func() { w.run(path) })
	w.mu.Unlock()
}

// forget clears all tracking state for path.
func (w *Watcher) forget(path string) {
	w.mu.Lock()
	delete(w.attempts, path)
	delete(w.timers, path)
	delete(w.lastSize, path)
	delete(w.moveAttempts, path)
	w.mu.Unlock()
}

// process hands path to the organizer, retrying with backoff if the move failed
// only because another process still held the file open.
func (w *Watcher) process(path string) {
	result := w.org.ProcessFile(path)
	switch {
	case result.Err != nil:
		log.Printf("[watcher] error processing %s: %v", path, result.Err)
	case result.Retryable:
		w.mu.Lock()
		attempts := w.moveAttempts[path]
		if attempts >= w.moveRetries {
			w.mu.Unlock()
			log.Printf("[watcher] giving up moving %s after %d retries: %s",
				filepath.Base(path), w.moveRetries, result.SkipReason)
			w.forget(path)
			return
		}
		w.moveAttempts[path] = attempts + 1
		// Back off exponentially; the holder usually releases within seconds.
		// The shift is capped so an oversized move_retries cannot overflow.
		delay := w.window << min(attempts, 6)
		w.timers[path] = time.AfterFunc(delay, func() { w.run(path) })
		w.mu.Unlock()
		log.Printf("[watcher] %s is locked, retrying in %s", filepath.Base(path), delay)
	case result.Skipped:
		// Nothing to do: an ignored extension or a vanished file.
	default:
		w.rememberCategoryDir(filepath.Dir(result.Destination))
		log.Printf("[watcher] moved %s → %s", filepath.Base(path), result.Category)
	}
}

func (w *Watcher) rememberCategoryDir(dir string) {
	w.catMu.Lock()
	w.categoryDirs[dir] = struct{}{}
	w.catMu.Unlock()
}

// isCategoryPath returns true if path is inside a known category subdirectory.
func (w *Watcher) isCategoryPath(path string) bool {
	dir := filepath.Dir(path)

	w.catMu.Lock()
	_, known := w.categoryDirs[dir]
	w.catMu.Unlock()
	if known {
		return true
	}

	// Also check statically: if the parent dir name matches a known category.
	for _, wp := range w.cfg.WatchPaths {
		watchAbs, err := filepath.Abs(wp.Path)
		if err != nil {
			continue
		}
		dirAbs, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		// If the file's parent is a direct child of the watch path, it may be a
		// category folder. We check by verifying it is a subdirectory one level deep.
		parentOfParent := filepath.Dir(dirAbs)
		if strings.EqualFold(parentOfParent, watchAbs) {
			return true
		}
	}
	return false
}
