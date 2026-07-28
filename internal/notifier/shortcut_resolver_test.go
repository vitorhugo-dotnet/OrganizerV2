package notifier

import (
	"path/filepath"
	"testing"

	"github.com/vitorhugo-java/organizerv2/internal/config"
)

func TestNotificationShortcutResolverPreservesOrderAndResolvesID(t *testing.T) {
	dir := t.TempDir()
	resolver := newNotificationShortcutResolver([]config.Shortcut{
		{ID: "id-desktop", Name: "Desktop", Path: filepath.Join(dir, "Desktop")},
		{ID: "id-documents", Name: "Documents", Path: filepath.Join(dir, "Documents")},
	})
	all := resolver.All()
	if len(all) != 2 || all[0].Name != "Desktop" || all[1].Name != "Documents" {
		t.Fatalf("unexpected order: %#v", all)
	}
	got, ok := resolver.Resolve("id-documents")
	if !ok || got.Name != "Documents" {
		t.Fatalf("unexpected resolve: %#v, %v", got, ok)
	}
}

func TestNotificationShortcutResolverRejectsPathsAndReturnsDefensiveCopies(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Desktop")
	resolver := newNotificationShortcutResolver([]config.Shortcut{
		{ID: "id-desktop", Name: "Desktop", Path: path},
		{ID: "", Name: "Missing ID", Path: filepath.Join(dir, "Missing")},
		{ID: "relative", Name: "Relative", Path: "relative/path"},
		{ID: "id-desktop", Name: "Duplicate", Path: filepath.Join(dir, "Duplicate")},
	})

	if _, ok := resolver.Resolve(path); ok {
		t.Fatal("resolver accepted a path as an ID")
	}
	all := resolver.All()
	if len(all) != 1 {
		t.Fatalf("expected one valid shortcut, got %#v", all)
	}
	all[0].Name = "mutated"
	if resolver.All()[0].Name != "Desktop" {
		t.Fatal("All returned mutable internal state")
	}
}
