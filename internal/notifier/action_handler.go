package notifier

import (
	"path/filepath"
	"strings"
)

type fileActionService interface {
	OpenFile(path string) error
	OpenLocation(path string) error
	CopyPath(path string) error
	MoveTo(path, destinationDir string) (string, error)
	CopyTo(path, destinationDir string) (string, error)
}

type windowsNotificationActionHandler struct {
	registry  *notificationEventRegistry
	shortcuts *notificationShortcutResolver
	files     fileActionService
	logf      func(string, ...any)
}

func (h *windowsNotificationActionHandler) Handle(args string, data []inputValue) {
	defer func() {
		if recovered := recover(); recovered != nil {
			h.log("[notifier] recovered activation panic: %v", recovered)
		}
	}()

	action, eventID, err := parseNotificationAction(args)
	if err != nil {
		h.log("[notifier] ignored invalid activation: %v", err)
		return
	}

	destination, ok := h.resolveDestination(action, data)
	if !ok {
		return
	}

	if h.registry == nil {
		h.log("[notifier] ignored activation without event registry")
		return
	}
	event, ok := h.registry.Claim(eventID)
	if !ok {
		h.log("[notifier] ignored unavailable event")
		return
	}

	currentPath := event.CurrentPath
	var actionErr error

	switch action {
	case actionOpenFile, actionOpenLocation:
		if destination != "" && !sameDestinationDirectory(currentPath, destination) {
			currentPath, actionErr = h.files.MoveTo(currentPath, destination)
			if actionErr != nil {
				h.log("[notifier] activation %s failed while moving: %v", action, actionErr)
				return
			}
		}
		if action == actionOpenFile {
			actionErr = h.files.OpenFile(currentPath)
		} else {
			actionErr = h.files.OpenLocation(currentPath)
		}
	case actionCopyPath:
		actionErr = h.files.CopyPath(currentPath)
	case actionMoveTo:
		if destination == "" {
			return
		}
		_, actionErr = h.files.MoveTo(currentPath, destination)
	case actionCopyTo:
		if destination == "" {
			return
		}
		_, actionErr = h.files.CopyTo(currentPath, destination)
	case actionConfirm:
		return
	}
	if actionErr != nil {
		h.log("[notifier] activation %s failed: %v", action, actionErr)
	}
}

func (h *windowsNotificationActionHandler) resolveDestination(action notificationAction, data []inputValue) (string, bool) {
	requiresDestination := action == actionMoveTo || action == actionCopyTo
	usesDestination := requiresDestination || action == actionOpenFile || action == actionOpenLocation
	if !usesDestination {
		return "", true
	}

	selected, err := selectedDestination(data)
	if err != nil {
		if !requiresDestination && !hasDestinationInput(data) {
			return "", true
		}
		h.log("[notifier] ignored activation without destination: %v", err)
		return "", false
	}
	if selected == currentDestinationSelectionID {
		return "", true
	}
	if h.shortcuts == nil {
		h.log("[notifier] ignored activation without shortcut resolver")
		return "", false
	}
	shortcut, ok := h.shortcuts.Resolve(selected)
	if !ok {
		h.log("[notifier] ignored unknown shortcut ID")
		return "", false
	}
	return shortcut.Path, true
}

func hasDestinationInput(values []inputValue) bool {
	for _, value := range values {
		if value.Key == destinationInputID {
			return true
		}
	}
	return false
}

func sameDestinationDirectory(path, destination string) bool {
	currentDirectory := filepath.Clean(filepath.Dir(path))
	destination = filepath.Clean(destination)
	return strings.EqualFold(currentDirectory, destination)
}

func (h *windowsNotificationActionHandler) log(format string, args ...any) {
	if h.logf != nil {
		h.logf(format, args...)
	}
}
