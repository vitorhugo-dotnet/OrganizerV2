//go:build windows

package notifier

import (
	"path/filepath"
	"strings"
	"testing"

	"git.sr.ht/~jackmordaunt/go-toast/v2"
	"github.com/vitorhugo-java/organizerv2/internal/config"
)

func TestBuildWindowsToastMatchesLegacyPythonLayout(t *testing.T) {
	dir := t.TempDir()
	resolver := newNotificationShortcutResolver([]config.Shortcut{
		{ID: strings.Repeat("1", 64), Name: "Desktop", Path: filepath.Join(dir, "Desktop")},
		{ID: strings.Repeat("2", 64), Name: "Share", Path: filepath.Join(dir, "Share")},
	})
	event := notificationEvent{
		ID:          strings.Repeat("a", 64),
		CurrentPath: filepath.Join(dir, "Script", "Organizer.pyw"),
		Category:    "Script",
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

	notification := buildWindowsToast(event, cfg, resolver.All(), `C:\Apps\OrganizerV2\organizer.exe`)

	if notification.Title != "File Organizer.pyw moved to Script." {
		t.Fatalf("Title = %q", notification.Title)
	}
	if notification.Body != "Organizer" {
		t.Fatalf("Body = %q", notification.Body)
	}
	if notification.ActivationType != "" || notification.ActivationArguments != "" {
		t.Fatalf("notification body must not be actionable: %#v", notification)
	}

	if len(notification.Inputs) != 1 {
		t.Fatalf("expected one destination input, got %#v", notification.Inputs)
	}
	input := notification.Inputs[0]
	if input.ID != destinationInputID || input.Title != "Move file to" || input.Placeholder != "" {
		t.Fatalf("unexpected destination input: %#v", input)
	}
	if len(input.Selections) != 3 {
		t.Fatalf("expected current category plus two shortcuts, got %#v", input.Selections)
	}
	if input.Selections[0].ID != "current" || input.Selections[0].Content != "Script" {
		t.Fatalf("current destination must be the default selection: %#v", input.Selections[0])
	}
	for i, shortcut := range resolver.All() {
		selection := input.Selections[i+1]
		if selection.ID != shortcut.ID || selection.Content != shortcut.Name {
			t.Fatalf("unexpected shortcut selection: %#v", selection)
		}
	}

	wantActions := []string{"Open Location", "Open File", "Confirm"}
	if len(notification.Actions) != len(wantActions) {
		t.Fatalf("actions = %#v, want %v", notification.Actions, wantActions)
	}
	for i, want := range wantActions {
		action := notification.Actions[i]
		if action.Content != want || action.Type != toast.Foreground {
			t.Fatalf("action %d = %#v, want %q foreground", i, action, want)
		}
	}
	if notification.Actions[0].InputID != destinationInputID || notification.Actions[1].InputID != destinationInputID {
		t.Fatalf("open actions must consume the destination selection: %#v", notification.Actions)
	}
	if notification.Actions[2].InputID != "" {
		t.Fatalf("confirm must not depend on destination input: %#v", notification.Actions[2])
	}
}

func TestWindowsNotificationOpenLocationMovesBeforeOpeningSelectedShortcut(t *testing.T) {
	fake := &fakeFileActionService{}
	handler, registry, shortcut, _ := newActionHandlerFixture(t, fake)
	event, err := registry.Register(filepath.Join(t.TempDir(), "Script", "Organizer.pyw"), "Script")
	if err != nil {
		t.Fatal(err)
	}

	handler.Handle(
		encodeNotificationAction(actionOpenLocation, event.ID),
		[]inputValue{{Key: destinationInputID, Value: shortcut.ID}},
	)

	calls, paths, destinations := fake.snapshot()
	if len(calls) != 2 || calls[0] != "move_to" || calls[1] != "open_location" {
		t.Fatalf("unexpected calls: %#v", calls)
	}
	movedPath := filepath.Join(shortcut.Path, filepath.Base(event.CurrentPath))
	if paths[0] != event.CurrentPath || destinations[0] != shortcut.Path {
		t.Fatalf("unexpected move: paths=%#v destinations=%#v", paths, destinations)
	}
	if paths[1] != movedPath {
		t.Fatalf("opened path = %q, want %q", paths[1], movedPath)
	}
}

func TestWindowsNotificationOpenFileKeepsCurrentDestination(t *testing.T) {
	fake := &fakeFileActionService{}
	handler, registry, _, _ := newActionHandlerFixture(t, fake)
	event, err := registry.Register(filepath.Join(t.TempDir(), "Documents", "report.xlsx"), "Documents")
	if err != nil {
		t.Fatal(err)
	}

	handler.Handle(
		encodeNotificationAction(actionOpenFile, event.ID),
		[]inputValue{{Key: destinationInputID, Value: "current"}},
	)

	calls, paths, destinations := fake.snapshot()
	if len(calls) != 1 || calls[0] != "open_file" || paths[0] != event.CurrentPath || destinations[0] != "" {
		t.Fatalf("unexpected current-destination behavior: calls=%#v paths=%#v destinations=%#v", calls, paths, destinations)
	}
}
