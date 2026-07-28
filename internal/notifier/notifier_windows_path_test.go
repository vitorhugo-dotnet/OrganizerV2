//go:build windows

package notifier

import (
	"path/filepath"
	"testing"

	"git.sr.ht/~jackmordaunt/go-toast/v2"
)

func TestWindowsNotifierNormalizesRelativeDestinationBeforeRegister(t *testing.T) {
	var notification *toast.Notification
	n, _ := newWindowsNotifierFixture(t, func(value *toast.Notification) error {
		notification = value
		return nil
	})

	relative := filepath.Join("relative", "file.txt")
	n.deliver(FileEvent{Destination: relative, Category: "Documents"})
	if notification == nil {
		t.Fatal("relative destination was not delivered")
	}
	eventID := eventIDFromNotification(t, notification)
	event, ok := n.registry.Claim(eventID)
	if !ok {
		t.Fatal("relative destination event was not registered")
	}
	want, err := filepath.Abs(relative)
	if err != nil {
		t.Fatal(err)
	}
	if event.CurrentPath != want {
		t.Fatalf("registered path = %q, want %q", event.CurrentPath, want)
	}
}
