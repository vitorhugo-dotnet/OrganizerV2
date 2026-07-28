package notifier

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

	var destination string
	if action == actionMoveTo || action == actionCopyTo {
		shortcutID, selectErr := selectedDestination(data)
		if selectErr != nil {
			h.log("[notifier] ignored activation without destination: %v", selectErr)
			return
		}
		if h.shortcuts == nil {
			h.log("[notifier] ignored activation without shortcut resolver")
			return
		}
		shortcut, ok := h.shortcuts.Resolve(shortcutID)
		if !ok {
			h.log("[notifier] ignored unknown shortcut ID")
			return
		}
		destination = shortcut.Path
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

	var actionErr error
	switch action {
	case actionOpenFile:
		actionErr = h.files.OpenFile(event.CurrentPath)
	case actionOpenLocation:
		actionErr = h.files.OpenLocation(event.CurrentPath)
	case actionCopyPath:
		actionErr = h.files.CopyPath(event.CurrentPath)
	case actionMoveTo:
		_, actionErr = h.files.MoveTo(event.CurrentPath, destination)
	case actionCopyTo:
		_, actionErr = h.files.CopyTo(event.CurrentPath, destination)
	case actionConfirm:
		return
	}
	if actionErr != nil {
		h.log("[notifier] activation %s failed: %v", action, actionErr)
	}
}

func (h *windowsNotificationActionHandler) log(format string, args ...any) {
	if h.logf != nil {
		h.logf(format, args...)
	}
}
