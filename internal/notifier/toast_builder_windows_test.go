//go:build windows

package notifier

import (
	"path/filepath"
	"strings"
	"testing"

	"git.sr.ht/~jackmordaunt/go-toast/v2"
	"github.com/vitorhugo-java/organizerv2/internal/config"
)

func TestBuildWindowsToastUsesBodyForOpenFileAndFiveButtons(t *testing.T) {
	dir := t.TempDir()
	shortcuts := newNotificationShortcutResolver([]config.Shortcut{
		{ID: strings.Repeat("1", 64), Name: "Desktop", Path: filepath.Join(dir, "Desktop")},
		{ID: strings.Repeat("2", 64), Name: "Documents", Path: filepath.Join(dir, "Documents")},
	})
	event := notificationEvent{
		ID:          strings.Repeat("a", 64),
		CurrentPath: filepath.Join(dir, "arquivo final.txt"),
		Category:    "Documents",
	}
	cfg := config.NotificationConfig{
		Enabled: true,
		Actions: config.NotificationActions{
			OpenFile:     true,
			OpenLocation: true,
			CopyPath:     true,
			MoveTo:       true,
			CopyTo:       true,
			Confirm:      true,
		},
	}
	activationExe := `C:\Apps\OrganizerV2\organizer.exe`

	notification := buildWindowsToast(event, cfg, shortcuts.All(), activationExe)
	if notification.AppID != windowsToastAppID {
		t.Fatalf("AppID = %q", notification.AppID)
	}
	if notification.Title != "OrganizerV2" {
		t.Fatalf("Title = %q", notification.Title)
	}
	if notification.ActivationExe != activationExe {
		t.Fatalf("ActivationExe = %q", notification.ActivationExe)
	}
	if notification.ActivationType != toast.Foreground {
		t.Fatalf("body activation type = %q", notification.ActivationType)
	}
	if notification.ActivationArguments != encodeNotificationAction(actionOpenFile, event.ID) {
		t.Fatalf("unexpected body activation arguments: %q", notification.ActivationArguments)
	}
	if len(notification.Actions) != 5 {
		t.Fatalf("expected exactly five buttons, got %d", len(notification.Actions))
	}
	wantContents := []string{"Open Folder", "Copy Path", "Move To", "Copy To", "Confirm"}
	for i, action := range notification.Actions {
		if action.Content != wantContents[i] {
			t.Fatalf("action %d content = %q, want %q", i, action.Content, wantContents[i])
		}
		if action.Type != toast.Foreground {
			t.Fatalf("action %q type = %q", action.Content, action.Type)
		}
	}
	if notification.Actions[2].InputID != destinationInputID || notification.Actions[3].InputID != destinationInputID {
		t.Fatalf("move/copy actions do not reference destination input: %#v", notification.Actions)
	}
	if len(notification.Inputs) != 1 || notification.Inputs[0].ID != destinationInputID {
		t.Fatalf("destination select missing: %#v", notification.Inputs)
	}
	if len(notification.Inputs[0].Selections) != 2 {
		t.Fatalf("unexpected selections: %#v", notification.Inputs[0].Selections)
	}
	for i, selection := range notification.Inputs[0].Selections {
		if selection.ID != shortcuts.All()[i].ID || selection.Content != shortcuts.All()[i].Name {
			t.Fatalf("unexpected selection: %#v", selection)
		}
		if selection.ID == shortcuts.All()[i].Path || strings.Contains(notification.Actions[0].Arguments, shortcuts.All()[i].Path) {
			t.Fatal("toast exposed a shortcut path")
		}
	}
}

func TestBuildWindowsToastRespectsConfigurationAndShortcutAvailability(t *testing.T) {
	dir := t.TempDir()
	event := notificationEvent{
		ID:          strings.Repeat("b", 64),
		CurrentPath: filepath.Join(dir, "file.txt"),
		Category:    "Documents",
	}
	shortcut := resolvedShortcut{ID: strings.Repeat("3", 64), Name: "Desktop", Path: filepath.Join(dir, "Desktop")}

	tests := []struct {
		name             string
		actions          config.NotificationActions
		shortcuts        []resolvedShortcut
		wantButtons      []string
		wantInput        bool
		wantBodyOpenFile bool
	}{
		{
			name: "open file disabled",
			actions: config.NotificationActions{
				OpenLocation: true,
				Confirm:      true,
			},
			wantButtons: []string{"Open Folder", "Confirm"},
		},
		{
			name: "no shortcuts",
			actions: config.NotificationActions{
				OpenFile: true,
				MoveTo:   true,
				CopyTo:   true,
				Confirm:  true,
			},
			wantButtons:      []string{"Confirm"},
			wantBodyOpenFile: true,
		},
		{
			name: "destination actions disabled",
			actions: config.NotificationActions{
				OpenFile: true,
				CopyPath: true,
				Confirm:  true,
			},
			shortcuts:        []resolvedShortcut{shortcut},
			wantButtons:      []string{"Copy Path", "Confirm"},
			wantBodyOpenFile: true,
		},
		{
			name: "single destination action",
			actions: config.NotificationActions{
				OpenFile: true,
				MoveTo:   true,
			},
			shortcuts:        []resolvedShortcut{shortcut},
			wantButtons:      []string{"Move To"},
			wantInput:        true,
			wantBodyOpenFile: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			notification := buildWindowsToast(event, config.NotificationConfig{Enabled: true, Actions: test.actions}, test.shortcuts, `C:\organizer.exe`)
			if len(notification.Actions) > 5 {
				t.Fatalf("toast has %d buttons", len(notification.Actions))
			}
			if len(notification.Actions) != len(test.wantButtons) {
				t.Fatalf("buttons = %#v, want %#v", notification.Actions, test.wantButtons)
			}
			for i, content := range test.wantButtons {
				if notification.Actions[i].Content != content {
					t.Fatalf("button %d = %q, want %q", i, notification.Actions[i].Content, content)
				}
			}
			if (len(notification.Inputs) == 1) != test.wantInput {
				t.Fatalf("inputs = %#v, wantInput=%v", notification.Inputs, test.wantInput)
			}
			if test.wantBodyOpenFile {
				if notification.ActivationType != toast.Foreground || notification.ActivationArguments == "" {
					t.Fatalf("body open-file activation missing: %#v", notification)
				}
			} else if notification.ActivationType != "" || notification.ActivationArguments != "" {
				t.Fatalf("body activation should be empty: %#v", notification)
			}
		})
	}
}
