package pathutil

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveDestination(t *testing.T) {
	result := ResolveDestination("/base", "Image", "photo.jpg")
	expected := filepath.Join("/base", "Image", "photo.jpg")
	if result != expected {
		t.Errorf("expected %s, got %s", expected, result)
	}
}

func TestResolveDuplicateNone(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	result, err := ResolveDuplicate(path)
	if err != nil {
		t.Fatal(err)
	}
	if result != path {
		t.Errorf("expected original path %s, got %s", path, result)
	}
}

func TestResolveDuplicateIncrement(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "file.txt")
	// Create the base file and a (2) duplicate.
	os.WriteFile(base, []byte("x"), 0o644)
	os.WriteFile(filepath.Join(dir, "file (2).txt"), []byte("x"), 0o644)

	result, err := ResolveDuplicate(base)
	if err != nil {
		t.Fatal(err)
	}
	expected := filepath.Join(dir, "file (3).txt")
	if result != expected {
		t.Errorf("expected %s, got %s", expected, result)
	}
}

func TestEnsureDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "a", "b", "c")
	if err := EnsureDir(dir); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		t.Errorf("expected directory to exist: %v", err)
	}
}

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir")
	}
	result, err := ExpandHome("~/foo")
	if err != nil {
		t.Fatal(err)
	}
	expected := filepath.Join(home, "foo")
	if result != expected {
		t.Errorf("expected %s, got %s", expected, result)
	}
}

func TestExpandHomeNoExpand(t *testing.T) {
	result, err := ExpandHome("/absolute/path")
	if err != nil {
		t.Fatal(err)
	}
	if result != "/absolute/path" {
		t.Errorf("unexpected expansion: %s", result)
	}
}

func TestCopyFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "dst.txt")
	content := []byte("hello pathutil")
	if err := os.WriteFile(src, content, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := CopyFile(src, dst); err != nil {
		t.Fatalf("CopyFile error: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("reading dst: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("content mismatch: got %q, want %q", got, content)
	}
	// Source must still exist.
	if _, err := os.Stat(src); err != nil {
		t.Errorf("source should still exist after CopyFile: %v", err)
	}
}

func TestCopyFileDoesNotOverwriteExistingDestination(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "dst.txt")
	if err := os.WriteFile(src, []byte("new"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := CopyFile(src, dst)
	if !errors.Is(err, os.ErrExist) {
		t.Fatalf("expected os.ErrExist, got %v", err)
	}
	got, readErr := os.ReadFile(dst)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != "existing" {
		t.Fatalf("destination was overwritten: %q", got)
	}
}

func TestMoveFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "dst.txt")
	content := []byte("move me")
	if err := os.WriteFile(src, content, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := MoveFile(src, dst); err != nil {
		t.Fatalf("MoveFile error: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("reading dst: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("content mismatch: got %q, want %q", got, content)
	}
	// Source must be gone.
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Error("source should be gone after MoveFile")
	}
}
