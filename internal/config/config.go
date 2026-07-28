package config

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// WatchPath pairs a directory to watch with a base directory where category
// subfolders will be created.
type WatchPath struct {
	Path       string `yaml:"path"        mapstructure:"path"`
	TargetBase string `yaml:"target_base" mapstructure:"target_base"`
}

// Rule maps a category name to a list of file extensions.
type Rule struct {
	Category   string   `yaml:"category"   mapstructure:"category"`
	Extensions []string `yaml:"extensions" mapstructure:"extensions"`
}

// Shortcut is a named, validated destination available to Windows toast actions.
type Shortcut struct {
	Name string `yaml:"name" mapstructure:"name"`
	Path string `yaml:"path" mapstructure:"path"`
	ID   string `yaml:"-" mapstructure:"-"`
}

// NotificationActions controls which actions appear on notifications.
type NotificationActions struct {
	OpenFile     bool `yaml:"open_file"     mapstructure:"open_file"`
	OpenLocation bool `yaml:"open_location" mapstructure:"open_location"`
	CopyPath     bool `yaml:"copy_path"     mapstructure:"copy_path"`
	MoveTo       bool `yaml:"move_to"       mapstructure:"move_to"`
	CopyTo       bool `yaml:"copy_to"       mapstructure:"copy_to"`
	Confirm      bool `yaml:"confirm"       mapstructure:"confirm"`
}

// NotificationConfig controls notification behaviour.
type NotificationConfig struct {
	Enabled   bool                `yaml:"enabled" mapstructure:"enabled"`
	Actions   NotificationActions `yaml:"actions" mapstructure:"actions"`
	Shortcuts []Shortcut          `yaml:"shortcuts" mapstructure:"shortcuts"`
}

// Config is the root configuration structure.
type Config struct {
	WatchPaths          []WatchPath        `yaml:"watch_paths"          mapstructure:"watch_paths"`
	Rules               []Rule             `yaml:"rules"                mapstructure:"rules"`
	IgnoreExtensions    []string           `yaml:"ignore_extensions"    mapstructure:"ignore_extensions"`
	FallbackCategory    string             `yaml:"fallback_category"    mapstructure:"fallback_category"`
	Notifications       NotificationConfig `yaml:"notifications"        mapstructure:"notifications"`
	PollIntervalSeconds int                `yaml:"poll_interval_seconds" mapstructure:"poll_interval_seconds"`
	LogLevel            string             `yaml:"log_level"            mapstructure:"log_level"`
	LogFile             string             `yaml:"log_file"             mapstructure:"log_file"`
}

// Default returns a sensible default configuration.
func Default() *Config {
	cfg := &Config{
		WatchPaths: []WatchPath{
			{Path: "~/Downloads", TargetBase: "~/Downloads"},
		},
		Rules: []Rule{
			{Category: "Image", Extensions: []string{".jpg", ".jpeg", ".png", ".gif", ".bmp", ".svg", ".webp", ".tiff", ".ico", ".heic", ".heif", ".avif", ".psd", ".tga", ".cr2", ".nef", ".arw"}},
			{Category: "Executables", Extensions: []string{".exe", ".msi", ".deb", ".appimage", ".bat", ".cmd", ".ps1", ".msix", ".msixbundle", ".appx", ".appxbundle"}},
			{Category: "Documents", Extensions: []string{".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx", ".txt", ".odt", ".ods", ".odp", ".csv", ".rtf", ".md"}},
			{Category: "Compacted", Extensions: []string{".zip", ".rar", ".7z", ".tar", ".gz", ".bz2", ".xz", ".zst", ".tgz", ".zipx"}},
			{Category: "ISO", Extensions: []string{".iso", ".img", ".vhd", ".vhdx"}},
			{Category: "Torrent", Extensions: []string{".torrent"}},
			{Category: "Video", Extensions: []string{".mkv", ".mp4", ".avi", ".mov", ".webm", ".flv", ".wmv", ".m4v", ".mpg", ".mpeg"}},
			{Category: "Audio", Extensions: []string{".wav", ".mp3", ".flac", ".aac", ".ogg", ".m4a", ".wma", ".opus"}},
			{Category: "Script", Extensions: []string{".py", ".pyw", ".js", ".ts", ".tsx", ".go", ".rs", ".rb", ".sh", ".java", ".c", ".cpp", ".json", ".yml", ".yaml", ".toml", ".ini", ".html", ".css", ".scss", ".vue", ".php"}},
		},
		IgnoreExtensions: []string{
			".tmp", ".!qB", ".!qb", ".!ut", ".fdmdownload", ".opdownload",
			".crdownload", ".aria2", ".part", ".download", ".partial",
			".downloading", ".filepart", ".tmpfile", ".!sync",
		},
		FallbackCategory:    "Others",
		PollIntervalSeconds: 2,
		LogLevel:            "info",
		Notifications: NotificationConfig{
			Enabled: true,
			Actions: NotificationActions{
				OpenFile:     true,
				OpenLocation: true,
				CopyPath:     true,
				MoveTo:       true,
				CopyTo:       true,
				Confirm:      true,
			},
			Shortcuts: []Shortcut{
				{Name: "Desktop", Path: "~/Desktop"},
				{Name: "Documents", Path: "~/Documents"},
			},
		},
	}

	// Defaults cannot return an error. If the home directory cannot be resolved,
	// omit shortcuts rather than exposing unvalidated destinations.
	shortcuts, err := normalizeShortcuts(cfg.Notifications.Shortcuts)
	if err != nil {
		cfg.Notifications.Shortcuts = nil
	} else {
		cfg.Notifications.Shortcuts = shortcuts
	}
	return cfg
}

// Load reads configuration from the given YAML file path. If the file does not
// exist the default config is returned so first-run works without a config file.
func Load(path string) (*Config, error) {
	expanded, err := expandHome(path)
	if err != nil {
		return nil, fmt.Errorf("expanding config path: %w", err)
	}

	v := viper.New()
	v.SetConfigFile(expanded)
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		if os.IsNotExist(err) {
			return Default(), nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	cfg := Default()
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	if err := normalize(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Save writes cfg as YAML to path, creating parent directories as needed.
func Save(cfg *Config, path string) error {
	expanded, err := expandHome(path)
	if err != nil {
		return fmt.Errorf("expanding config path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(expanded), 0o755); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	f, err := os.Create(expanded)
	if err != nil {
		return fmt.Errorf("creating config file: %w", err)
	}
	defer f.Close()
	enc := yaml.NewEncoder(f)
	enc.SetIndent(2)
	return enc.Encode(cfg)
}

// normalize lowercases extensions and expands configured paths.
func normalize(cfg *Config) error {
	for i, r := range cfg.Rules {
		for j, ext := range r.Extensions {
			cfg.Rules[i].Extensions[j] = strings.ToLower(ext)
		}
	}
	for i, ext := range cfg.IgnoreExtensions {
		cfg.IgnoreExtensions[i] = strings.ToLower(ext)
	}
	for i, wp := range cfg.WatchPaths {
		p, err := expandHome(wp.Path)
		if err != nil {
			return err
		}
		cfg.WatchPaths[i].Path = p
		tb, err := expandHome(wp.TargetBase)
		if err != nil {
			return err
		}
		cfg.WatchPaths[i].TargetBase = tb
	}
	shortcuts, err := normalizeShortcuts(cfg.Notifications.Shortcuts)
	if err != nil {
		return err
	}
	cfg.Notifications.Shortcuts = shortcuts
	return nil
}

func normalizeShortcuts(shortcuts []Shortcut) ([]Shortcut, error) {
	seenNames := make(map[string]struct{}, len(shortcuts))
	seenPaths := make(map[string]struct{}, len(shortcuts))
	result := make([]Shortcut, 0, len(shortcuts))

	for _, shortcut := range shortcuts {
		name := strings.TrimSpace(shortcut.Name)
		rawPath := strings.TrimSpace(shortcut.Path)
		if name == "" || rawPath == "" {
			continue
		}

		expanded, err := expandHome(rawPath)
		if err != nil {
			return nil, fmt.Errorf("expanding notification shortcut %q: %w", name, err)
		}
		absolute, err := filepath.Abs(expanded)
		if err != nil {
			return nil, fmt.Errorf("resolving notification shortcut %q: %w", name, err)
		}
		absolute = filepath.Clean(absolute)
		nameKey := strings.ToLower(name)
		pathKey := shortcutPathIdentity(absolute)

		if _, exists := seenNames[nameKey]; exists {
			continue
		}
		if _, exists := seenPaths[pathKey]; exists {
			continue
		}

		sum := sha256.Sum256([]byte("organizerv2-shortcut-v1\x00" + pathKey))
		shortcut = Shortcut{
			Name: name,
			Path: absolute,
			ID:   hex.EncodeToString(sum[:]),
		}
		seenNames[nameKey] = struct{}{}
		seenPaths[pathKey] = struct{}{}
		result = append(result, shortcut)
	}
	return result, nil
}

func expandHome(path string) (string, error) {
	if !strings.HasPrefix(path, "~/") && path != "~" {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home dir: %w", err)
	}
	if path == "~" {
		return home, nil
	}
	return filepath.Join(home, path[2:]), nil
}
