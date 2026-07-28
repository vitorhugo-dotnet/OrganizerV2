//go:build windows

package config

import "testing"

func TestNormalizeShortcutsDeduplicatesWindowsPathCase(t *testing.T) {
	shortcuts, err := normalizeShortcuts([]Shortcut{
		{Name: "First", Path: `C:\Users\Hugo\Desktop`},
		{Name: "Second", Path: `c:\users\hugo\desktop`},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(shortcuts) != 1 {
		t.Fatalf("expected one shortcut, got %d", len(shortcuts))
	}
}
