//go:build windows

package notifier

import (
	"fmt"
	"path/filepath"

	"git.sr.ht/~jackmordaunt/go-toast/v2"
	"github.com/vitorhugo-java/organizerv2/internal/config"
)

func buildWindowsToast(event notificationEvent, cfg config.NotificationConfig, shortcuts []resolvedShortcut, activationExe string) toast.Notification {
	notification := toast.Notification{
		AppID:         windowsToastAppID,
		Title:         "OrganizerV2",
		Body:          fmt.Sprintf("%s → %s/", filepath.Base(event.CurrentPath), event.Category),
		ActivationExe: activationExe,
	}

	if cfg.Actions.OpenFile {
		notification.ActivationType = toast.Foreground
		notification.ActivationArguments = encodeNotificationAction(actionOpenFile, event.ID)
	}

	hasDestinationActions := len(shortcuts) > 0 && (cfg.Actions.MoveTo || cfg.Actions.CopyTo)
	if hasDestinationActions {
		selections := make([]toast.InputSelection, 0, len(shortcuts))
		for _, shortcut := range shortcuts {
			selections = append(selections, toast.InputSelection{
				ID:      shortcut.ID,
				Content: shortcut.Name,
			})
		}
		notification.Inputs = append(notification.Inputs, toast.Input{
			ID:          destinationInputID,
			Title:       "Redirect to",
			Placeholder: "Choose destination",
			Selections:  selections,
		})
	}

	appendAction := func(enabled bool, content string, action notificationAction, inputID string) {
		if !enabled {
			return
		}
		notification.Actions = append(notification.Actions, toast.Action{
			Type:      toast.Foreground,
			Content:   content,
			Arguments: encodeNotificationAction(action, event.ID),
			InputID:   inputID,
		})
	}

	appendAction(cfg.Actions.OpenLocation, "Open Folder", actionOpenLocation, "")
	appendAction(cfg.Actions.CopyPath, "Copy Path", actionCopyPath, "")
	appendAction(hasDestinationActions && cfg.Actions.MoveTo, "Move To", actionMoveTo, destinationInputID)
	appendAction(hasDestinationActions && cfg.Actions.CopyTo, "Copy To", actionCopyTo, destinationInputID)
	appendAction(cfg.Actions.Confirm, "Confirm", actionConfirm, "")

	return notification
}
