package notifier

import (
	"path/filepath"
	"strings"

	"github.com/vitorhugo-java/organizerv2/internal/config"
)

type resolvedShortcut struct {
	ID   string
	Name string
	Path string
}

type notificationShortcutResolver struct {
	ordered []resolvedShortcut
	byID    map[string]resolvedShortcut
}

func newNotificationShortcutResolver(shortcuts []config.Shortcut) notificationShortcutResolver {
	resolver := notificationShortcutResolver{
		ordered: make([]resolvedShortcut, 0, len(shortcuts)),
		byID:    make(map[string]resolvedShortcut, len(shortcuts)),
	}
	for _, shortcut := range shortcuts {
		if strings.TrimSpace(shortcut.ID) == "" || strings.TrimSpace(shortcut.Name) == "" || !filepath.IsAbs(shortcut.Path) {
			continue
		}
		if _, exists := resolver.byID[shortcut.ID]; exists {
			continue
		}
		resolved := resolvedShortcut{
			ID:   shortcut.ID,
			Name: shortcut.Name,
			Path: filepath.Clean(shortcut.Path),
		}
		resolver.byID[resolved.ID] = resolved
		resolver.ordered = append(resolver.ordered, resolved)
	}
	return resolver
}

func (r notificationShortcutResolver) All() []resolvedShortcut {
	return append([]resolvedShortcut(nil), r.ordered...)
}

func (r notificationShortcutResolver) Resolve(id string) (resolvedShortcut, bool) {
	shortcut, ok := r.byID[id]
	return shortcut, ok
}
