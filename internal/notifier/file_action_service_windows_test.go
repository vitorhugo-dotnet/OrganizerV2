//go:build windows

package notifier

import (
	"os"
	"path/filepath"
	"testing"
)

func newTestWindowsFileActionService() *windowsFileActionService {
	return &windowsFileActionService{
		openFile:       func(string) error { return nil },
		startProcess:   func(string, ...string) error { return nil },
		writeClipboard: func(string) error { return nil },
	}
}

func TestWindowsFileActionServiceMoveToResolvesDuplicate(t *testing.T) {
	sourceDir := t.TempDir()
	destinationDir := t.TempDir()
	source := filepath.Join(sourceDir, "arquivo.txt")
	if err := os.WriteFile(source, []byte("new"), 0o640); err != nil {
		t.Fatal(err)
	}
	existing := filepath.Join(destinationDir, "arquivo.txt")
	if err := os.WriteFile(existing, []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}

	service := newTestWindowsFileActionService()
	got, err := service.MoveTo(source, destinationDir)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(destinationDir, "arquivo (2).txt")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatal("source still exists")
	}
	content, err := os.ReadFile(existing)
	if err != nil || string(content) != "existing" {
		t.Fatalf("existing destination changed: %q, %v", content, err)
	}
}

func TestWindowsFileActionServiceCopyToResolvesDuplicateAndKeepsSource(t *testing.T) {
	sourceDir := t.TempDir()
	destinationDir := t.TempDir()
	source := filepath.Join(sourceDir, "arquivo.txt")
	if err := os.WriteFile(source, []byte("new"), 0o640); err != nil {
		t.Fatal(err)
	}
	existing := filepath.Join(destinationDir, "arquivo.txt")
	if err := os.WriteFile(existing, []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}

	service := newTestWindowsFileActionService()
	got, err := service.CopyTo(source, destinationDir)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(destinationDir, "arquivo (2).txt")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatalf("source was removed: %v", err)
	}
	content, err := os.ReadFile(existing)
	if err != nil || string(content) != "existing" {
		t.Fatalf("existing destination changed: %q, %v", content, err)
	}
	copied, err := os.ReadFile(want)
	if err != nil || string(copied) != "new" {
		t.Fatalf("unexpected copy: %q, %v", copied, err)
	}
}

func TestWindowsFileActionServiceOpenLocationPassesExplorerSelectionAsSeparateArguments(t *testing.T) {
	path := `C:\Users\Hugo\A & B\arquivo (1) # final.txt`
	var gotName string
	var gotArgs []string
	service := newTestWindowsFileActionService()
	service.startProcess = func(name string, args ...string) error {
		gotName = name
		gotArgs = append([]string(nil), args...)
		return nil
	}

	if err := service.OpenLocation(path); err != nil {
		t.Fatal(err)
	}
	if gotName != "explorer.exe" {
		t.Fatalf("process = %q", gotName)
	}
	wantArgs := []string{"/select,", path}
	if len(gotArgs) != len(wantArgs) {
		t.Fatalf("unexpected arguments: %#v, want %#v", gotArgs, wantArgs)
	}
	for i := range wantArgs {
		if gotArgs[i] != wantArgs[i] {
			t.Fatalf("argument %d = %q, want %q", i, gotArgs[i], wantArgs[i])
		}
	}
}

func TestWindowsFileActionServiceCopyPathUsesExactValue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "arquivo com espaço.txt")
	var values []string
	service := newTestWindowsFileActionService()
	service.writeClipboard = func(text string) error {
		values = append(values, text)
		return nil
	}

	if err := service.CopyPath(path); err != nil {
		t.Fatal(err)
	}
	if len(values) != 1 || values[0] != path {
		t.Fatalf("unexpected clipboard values: %#v", values)
	}
}

func TestWindowsFileActionServiceOpenFileValidatesPathAndExistence(t *testing.T) {
	service := newTestWindowsFileActionService()
	var opened []string
	service.openFile = func(path string) error {
		opened = append(opened, path)
		return nil
	}

	if err := service.OpenFile("relative.txt"); err == nil {
		t.Fatal("expected relative path rejection")
	}
	missing := filepath.Join(t.TempDir(), "missing.txt")
	if err := service.OpenFile(missing); err == nil {
		t.Fatal("expected missing file rejection")
	}
	file := filepath.Join(t.TempDir(), "existing.txt")
	if err := os.WriteFile(file, []byte("ok"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := service.OpenFile(file); err != nil {
		t.Fatal(err)
	}
	if len(opened) != 1 || opened[0] != file {
		t.Fatalf("unexpected open calls: %#v", opened)
	}
}

func TestWindowsFileActionServiceRejectsRelativeTransferPaths(t *testing.T) {
	service := newTestWindowsFileActionService()
	if _, err := service.MoveTo("relative.txt", t.TempDir()); err == nil {
		t.Fatal("expected relative source rejection")
	}
	if _, err := service.CopyTo(filepath.Join(t.TempDir(), "file.txt"), "relative"); err == nil {
		t.Fatal("expected relative destination rejection")
	}
}
