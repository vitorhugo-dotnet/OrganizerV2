//go:build windows

package notifier

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/vitorhugo-java/organizerv2/internal/config"
)

func TestBuildWindowsToastRespectsVisibleActionConfiguration(t *testing.T) {
	dir := t.TempDir()
	event := notificationEvent{
		ID:          strings.Repeat("a", 64),
		CurrentPath: filepath.Join(dir, "Documents", "report.xlsx"),
		Category:    "Documents",
	}
	shortcut := resolvedShortcut{
		ID:   strings.Repeat("1", 64),
		Name: "Desktop",
		Path: filepath.Join(dir, "Desktop"),
	}

	tests := []struct {
		name           string
		actions        config.NotificationActions
		wantActions    []string
		wantSelections int
	}{
		{
			name: "all legacy actions",
			actions: config.NotificationActions{
				OpenFile:     true,
				OpenLocation: true,
				MoveTo:       true,
				Confirm:      true,
			},
			wantActions:    []string{"Open Location", "Open File", "Confirm"},
			wantSelections: 2,
		},
		{
			name: "open file only",
			actions: config.NotificationActions{
				OpenFile: true,
			},
			wantActions:    []string{"Open File"},
			wantSelections: 1,
		},
		{
			name: "confirm only",
			actions: config.NotificationActions{
				Confirm: true,
			},
			wantActions:    []string{"Confirm"},
			wantSelections: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			notification := buildWindowsToast(
				event,
				config.NotificationConfig{Enabled: true, Actions: test.actions},
				[]resolvedShortcut{shortcut},
				`C:\organizer.exe`,
			)
			if len(notification.Actions) != len(test.wantActions) {
				t.Fatalf("actions = %#v, want %v", notification.Actions, test.wantActions)
			}
			for i, want := range test.wantActions {
				if notification.Actions[i].Content != want {
					t.Fatalf("action %d = %q, want %q", i, notification.Actions[i].Content, want)
				}
			}
			if len(notification.Inputs) != 1 || len(notification.Inputs[0].Selections) != test.wantSelections {
				t.Fatalf("unexpected inputs: %#v", notification.Inputs)
			}
			if notification.Inputs[0].Selections[0].ID != currentDestinationSelectionID {
				t.Fatalf("current destination is not first: %#v", notification.Inputs[0].Selections)
			}
		})
	}
}

func TestBuildWindowsToastKeepsPathsOutOfActivationPayloads(t *testing.T) {
	dir := t.TempDir()
	shortcutPath := filepath.Join(dir, "Private Destination")
	shortcut := resolvedShortcut{
		ID:   strings.Repeat("2", 64),
		Name: "Private",
		Path: shortcutPath,
	}
	event := notificationEvent{
		ID:          strings.Repeat("b", 64),
		CurrentPath: filepath.Join(dir, "Script", "tool.pyw"),
		Category:    "Script",
	}
	cfg := config.NotificationConfig{
		Enabled: true,
		Actions: config.NotificationActions{
			OpenFile:     true,
			OpenLocation: true,
			MoveTo:       true,
			Confirm:      true,
		},
	}

	notification := buildWindowsToast(event, cfg, []resolvedShortcut{shortcut}, `C:\organizer.exe`)
	for _, action := range notification.Actions {
		if strings.Contains(action.Arguments, event.CurrentPath) || strings.Contains(action.Arguments, shortcutPath) {
			t.Fatalf("action exposed a path: %#v", action)
		}
	}
	for _, selection := range notification.Inputs[0].Selections {
		if selection.ID == event.CurrentPath || selection.ID == shortcutPath {
			t.Fatalf("selection exposed a path: %#v", selection)
		}
	}
}
