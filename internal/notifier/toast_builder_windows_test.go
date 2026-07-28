//go:build windows

package notifier

import (
	"encoding/xml"
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

func TestBuildWindowsToastPreselectsRealCurrentDirectoryAndKeepsConfiguredDestinations(t *testing.T) {
	dir := t.TempDir()
	currentDir := filepath.Join(dir, "Moved & Sorted")
	event := notificationEvent{
		ID:          strings.Repeat("c", 64),
		CurrentPath: filepath.Join(currentDir, "report.xlsx"),
		Category:    "Spreadsheets",
	}
	shortcuts := []resolvedShortcut{
		{
			ID:   strings.Repeat("3", 64),
			Name: "Duplicate current directory",
			Path: currentDir,
		},
		{
			ID:   strings.Repeat("4", 64),
			Name: "Desktop & Work",
			Path: filepath.Join(dir, "Desktop"),
		},
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

	notification := buildWindowsToast(event, cfg, shortcuts, `C:\organizer.exe`)
	if notification.DefaultInput != currentDestinationSelectionID {
		t.Fatalf("default input = %q, want %q", notification.DefaultInput, currentDestinationSelectionID)
	}
	if len(notification.Inputs) != 1 {
		t.Fatalf("inputs = %#v", notification.Inputs)
	}
	selections := notification.Inputs[0].Selections
	if len(selections) != 2 {
		t.Fatalf("selections = %#v, want current directory plus one configured destination", selections)
	}
	if selections[0].ID != currentDestinationSelectionID || selections[0].Content != "Moved & Sorted (current)" {
		t.Fatalf("unexpected current directory selection: %#v", selections[0])
	}
	if selections[1].ID != shortcuts[1].ID || selections[1].Content != shortcuts[1].Name {
		t.Fatalf("unexpected configured destination: %#v", selections[1])
	}

	payload, err := notification.XML()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(payload, `defaultInput="current"`) {
		t.Fatalf("toast XML does not preselect the current directory: %s", payload)
	}
	var document struct {
		XMLName xml.Name `xml:"toast"`
	}
	if err := xml.Unmarshal([]byte(payload), &document); err != nil {
		t.Fatalf("toast XML is invalid: %v\n%s", err, payload)
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
