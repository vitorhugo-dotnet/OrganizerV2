package notifier

import (
	"encoding/hex"
	"fmt"
	"strings"
)

type notificationAction string

const (
	actionOpenFile     notificationAction = "open_file"
	actionOpenLocation notificationAction = "open_location"
	actionCopyPath     notificationAction = "copy_path"
	actionMoveTo       notificationAction = "move_to"
	actionCopyTo       notificationAction = "copy_to"
	actionConfirm      notificationAction = "confirm"
	destinationInputID                    = "destination"
)

type inputValue struct {
	Key   string
	Value string
}

func encodeNotificationAction(action notificationAction, eventID string) string {
	return fmt.Sprintf("v1|%s|%s", action, eventID)
}

func parseNotificationAction(raw string) (notificationAction, string, error) {
	parts := strings.Split(raw, "|")
	if len(parts) != 3 {
		return "", "", fmt.Errorf("invalid notification action payload")
	}
	if parts[0] != "v1" {
		return "", "", fmt.Errorf("unsupported notification action version")
	}

	action := notificationAction(parts[1])
	switch action {
	case actionOpenFile, actionOpenLocation, actionCopyPath, actionMoveTo, actionCopyTo, actionConfirm:
	default:
		return "", "", fmt.Errorf("unknown notification action")
	}

	eventID := parts[2]
	if len(eventID) != 64 {
		return "", "", fmt.Errorf("invalid notification event ID length")
	}
	decoded, err := hex.DecodeString(eventID)
	if err != nil || len(decoded) != 32 {
		return "", "", fmt.Errorf("invalid notification event ID")
	}
	return action, eventID, nil
}

func selectedDestination(values []inputValue) (string, error) {
	var selected string
	matches := 0
	for _, value := range values {
		if value.Key != destinationInputID {
			continue
		}
		matches++
		selected = strings.TrimSpace(value.Value)
	}
	if matches != 1 || selected == "" {
		return "", fmt.Errorf("exactly one destination selection is required")
	}
	return selected, nil
}
