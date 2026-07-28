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
		Title:         fmt.Sprintf("File %s moved to %s.", filepath.Base(event.CurrentPath), event.Category),
		Body:          "Organizer",
		ActivationExe: activationExe,
	}

	selections := []toast.InputSelection{
		{ID: currentDestinationSelectionID, Content: event.Category},
	}
	if cfg.Actions.MoveTo {
		for _, shortcut := range shortcuts {
			selections = append(selections, toast.InputSelection{
				ID:      shortcut.ID,
				Content: shortcut.Name,
			})
		}
	}
	notification.Inputs = []toast.Input{
		{
			ID:         destinationInputID,
			Title:      "Move file to",
			Selections: selections,
		},
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

	appendAction(cfg.Actions.OpenLocation, "Open Location", actionOpenLocation, destinationInputID)
	appendAction(cfg.Actions.OpenFile, "Open File", actionOpenFile, destinationInputID)
	appendAction(cfg.Actions.Confirm, "Confirm", actionConfirm, "")

	return notification
}
