package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefault(t *testing.T) {
	cfg := Default()
	if cfg == nil {
		t.Fatal("Default() returned nil")
	}
	if len(cfg.Rules) == 0 {
		t.Error("expected default rules")
	}
	if cfg.FallbackCategory == "" {
		t.Error("expected non-empty fallback category")
	}
	if !cfg.Notifications.Enabled {
		t.Error("notifications should be enabled by default")
	}
	if !cfg.Notifications.Actions.CopyPath {
		t.Error("copy_path action should be enabled by default")
	}
	if !cfg.Notifications.Actions.MoveTo || !cfg.Notifications.Actions.CopyTo {
		t.Error("move_to and copy_to should be enabled by default")
	}
	if len(cfg.Notifications.Shortcuts) != 2 {
		t.Fatalf("expected 2 default shortcuts, got %d", len(cfg.Notifications.Shortcuts))
	}
	for _, shortcut := range cfg.Notifications.Shortcuts {
		if shortcut.ID == "" {
			t.Errorf("shortcut %q has empty ID", shortcut.Name)
		}
		if !filepath.IsAbs(shortcut.Path) {
			t.Errorf("shortcut %q path is not absolute: %s", shortcut.Name, shortcut.Path)
		}
	}
}

func TestLoadMissingFile(t *testing.T) {
	cfg, err := Load("/tmp/does-not-exist-organizerv2.yaml")
	if err != nil {
		t.Fatalf("unexpected error for missing file: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected default config when file missing")
	}
}

func TestLoadYAML(t *testing.T) {
	content := `
watch_paths:
  - path: /tmp/watch
    target_base: /tmp/target
rules:
  - category: Image
    extensions: [.JPG, .PNG]
ignore_extensions: [.TMP, .part]
fallback_category: Misc
notifications:
  enabled: false
  actions:
    open_file: true
    open_location: false
    copy_path: true
    move_to: true
    copy_to: false
    confirm: false
  shortcuts:
    - name: " Desktop "
      path: "~/Desktop"
    - name: Desktop
      path: "~/OtherDesktop"
    - name: EmptyPath
      path: ""
    - name: Documents
      path: "./Documents"
`
	f, err := os.CreateTemp("", "organizer-config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(f.Name())
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if cfg.FallbackCategory != "Misc" {
		t.Errorf("expected Misc, got %s", cfg.FallbackCategory)
	}
	if cfg.Notifications.Enabled {
		t.Error("notifications should be disabled")
	}
	if cfg.Notifications.Actions.OpenLocation {
		t.Error("open_location should be disabled")
	}
	if !cfg.Notifications.Actions.MoveTo {
		t.Error("move_to should be enabled")
	}
	if cfg.Notifications.Actions.CopyTo {
		t.Error("copy_to should remain disabled")
	}
	if len(cfg.Notifications.Shortcuts) != 2 {
		t.Fatalf("expected 2 valid shortcuts, got %d", len(cfg.Notifications.Shortcuts))
	}
	if cfg.Notifications.Shortcuts[0].Name != "Desktop" {
		t.Fatalf("expected trimmed Desktop shortcut, got %q", cfg.Notifications.Shortcuts[0].Name)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	expectedDesktop := filepath.Join(home, "Desktop")
	if cfg.Notifications.Shortcuts[0].Path != expectedDesktop {
		t.Fatalf("expected first Desktop path %q, got %q", expectedDesktop, cfg.Notifications.Shortcuts[0].Path)
	}
	if cfg.Notifications.Shortcuts[1].Name != "Documents" {
		t.Fatalf("expected Documents shortcut, got %q", cfg.Notifications.Shortcuts[1].Name)
	}
	if !filepath.IsAbs(cfg.Notifications.Shortcuts[1].Path) {
		t.Fatalf("expected absolute Documents path, got %q", cfg.Notifications.Shortcuts[1].Path)
	}
	for _, shortcut := range cfg.Notifications.Shortcuts {
		if shortcut.ID == "" {
			t.Errorf("shortcut %q has empty ID", shortcut.Name)
		}
	}

	// Extensions must be normalized to lowercase.
	for _, r := range cfg.Rules {
		for _, ext := range r.Extensions {
			if ext != strings.ToLower(ext) {
				t.Errorf("extension not lowercased: %s", ext)
			}
		}
	}
	for _, ext := range cfg.IgnoreExtensions {
		if ext != strings.ToLower(ext) {
			t.Errorf("ignore extension not lowercased: %s", ext)
		}
	}
}

func TestSaveAndReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	cfg := Default()
	cfg.FallbackCategory = "SavedOthers"

	if err := Save(cfg, path); err != nil {
		t.Fatalf("Save error: %v", err)
	}
	reloaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if reloaded.FallbackCategory != "SavedOthers" {
		t.Errorf("expected SavedOthers, got %s", reloaded.FallbackCategory)
	}
}

func TestExpandHomePath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir")
	}
	result, err := expandHome("~/foo/bar")
	if err != nil {
		t.Fatal(err)
	}
	expected := filepath.Join(home, "foo/bar")
	if result != expected {
		t.Errorf("expected %s, got %s", expected, result)
	}
}
