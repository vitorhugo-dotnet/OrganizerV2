package organizer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/vitorhugo-java/organizerv2/internal/config"
	"github.com/vitorhugo-java/organizerv2/internal/notifier"
	"github.com/vitorhugo-java/organizerv2/internal/rules"
)

func newTestOrganizer(t *testing.T, watchDir string) *Organizer {
	t.Helper()
	cfg := config.Default()
	cfg.WatchPaths = []config.WatchPath{{Path: watchDir, TargetBase: watchDir}}
	clf := rules.NewClassifier(cfg.Rules, cfg.IgnoreExtensions, cfg.FallbackCategory)
	return New(cfg, clf, notifier.NoopNotifier{})
}

func writeFile(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestProcessFileMovesToCategory(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "photo.jpg")
	writeFile(t, src)

	o := newTestOrganizer(t, dir)
	result := o.ProcessFile(src)

	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	if result.Skipped {
		t.Fatal("expected file to be moved, not skipped")
	}
	if result.Category != "Image" {
		t.Errorf("expected Image, got %s", result.Category)
	}
	expected := filepath.Join(dir, "Image", "photo.jpg")
	if result.Destination != expected {
		t.Errorf("expected %s, got %s", expected, result.Destination)
	}
	if _, err := os.Stat(expected); err != nil {
		t.Errorf("destination file not found: %v", err)
	}
}

func TestProcessFileIgnored(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "partial.tmp")
	writeFile(t, src)

	o := newTestOrganizer(t, dir)
	result := o.ProcessFile(src)

	if !result.Skipped {
		t.Error("expected .tmp file to be skipped")
	}
	// Source must not be moved.
	if _, err := os.Stat(src); err != nil {
		t.Error("source should still exist")
	}
}

func TestProcessFileFallback(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "mystery.xyz")
	writeFile(t, src)

	o := newTestOrganizer(t, dir)
	result := o.ProcessFile(src)

	if result.Err != nil {
		t.Fatal(result.Err)
	}
	if result.Category != "Others" {
		t.Errorf("expected Others, got %s", result.Category)
	}
}

func TestProcessFileDuplicate(t *testing.T) {
	dir := t.TempDir()
	// Pre-create the destination.
	if err := os.MkdirAll(filepath.Join(dir, "Image"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "Image", "photo.jpg"))

	src := filepath.Join(dir, "photo.jpg")
	writeFile(t, src)

	o := newTestOrganizer(t, dir)
	result := o.ProcessFile(src)

	if result.Err != nil {
		t.Fatal(result.Err)
	}
	expected := filepath.Join(dir, "Image", "photo (2).jpg")
	if result.Destination != expected {
		t.Errorf("expected %s, got %s", expected, result.Destination)
	}
}

func TestProcessFileNotExist(t *testing.T) {
	dir := t.TempDir()
	o := newTestOrganizer(t, dir)
	result := o.ProcessFile(filepath.Join(dir, "ghost.jpg"))
	if !result.Skipped {
		t.Error("missing file should be skipped")
	}
}

func TestScanDir(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.jpg"))
	writeFile(t, filepath.Join(dir, "b.mp3"))
	writeFile(t, filepath.Join(dir, "c.tmp"))

	o := newTestOrganizer(t, dir)
	results := o.ScanDir(dir)

	var moved, skipped int
	for _, r := range results {
		if r.Skipped {
			skipped++
		} else if r.Err == nil {
			moved++
		}
	}
	if moved != 2 {
		t.Errorf("expected 2 moved, got %d", moved)
	}
	if skipped != 1 {
		t.Errorf("expected 1 skipped, got %d", skipped)
	}
}

func TestIsLockedRecognisesPermissionErrors(t *testing.T) {
	if !isLocked(os.ErrPermission) {
		t.Error("os.ErrPermission should count as locked")
	}
	if !isLocked(fmt.Errorf("wrapped: %w", os.ErrPermission)) {
		t.Error("a wrapped permission error should count as locked")
	}
	if isLocked(os.ErrNotExist) {
		t.Error("a missing file is not a lock")
	}
	if isLocked(errors.New("some other failure")) {
		t.Error("an unrelated error is not a lock")
	}
}

// TestCopyAndDeleteDoesNotTruncateDestination covers the cross-device fallback.
// It used to call os.Create, which truncates, so a file that appeared between
// ResolveDuplicate picking the name and the copy running would be destroyed.
func TestCopyAndDeleteDoesNotTruncateDestination(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.bin")
	dst := filepath.Join(dir, "dst.bin")

	if err := os.WriteFile(src, []byte("new data"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("precious existing data"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := copyAndDelete(src, dst); err == nil {
		t.Fatal("expected copyAndDelete to refuse an existing destination")
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "precious existing data" {
		t.Errorf("destination was clobbered, contains %q", got)
	}
	if _, err := os.Stat(src); err != nil {
		t.Errorf("source must survive a failed move: %v", err)
	}
}

func TestCopyAndDeleteMovesToFreeDestination(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.bin")
	dst := filepath.Join(dir, "sub", "dst.bin")

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := copyAndDelete(src, dst); err != nil {
		t.Fatalf("copyAndDelete: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "payload" {
		t.Errorf("destination contains %q, want %q", got, "payload")
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Errorf("source should be removed after a successful move, stat gave %v", err)
	}
}
