//go:build windows

package notifier

import (
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vitorhugo-java/organizerv2/internal/config"
)

func newWindowsNotifierFixture(t *testing.T, push toastPusher) (*windowsNotifier, *fakeFileActionService) {
	t.Helper()
	var sequence atomic.Uint64
	registry := newNotificationEventRegistry(eventRegistryOptions{
		now: time.Now,
		newID: func() (string, error) {
			return fmt.Sprintf("%064x", sequence.Add(1)), nil
		},
		ttl:             7 * 24 * time.Hour,
		cleanupInterval: 0,
	})
	t.Cleanup(func() { _ = registry.Close() })
	resolverValue := newNotificationShortcutResolver(nil)
	files := &fakeFileActionService{}
	return &windowsNotifier{
		cfg: config.NotificationConfig{
			Enabled: true,
			Actions: config.NotificationActions{
				OpenFile: true,
				CopyPath: true,
				Confirm:  true,
			},
		},
		executable: `C:\Apps\OrganizerV2\organizer.exe`,
		registry:   registry,
		shortcuts:  &resolverValue,
		files:      files,
		push:       push,
	}, files
}

func eventIDFromNotification(t *testing.T, notification *windowsToast) string {
	t.Helper()
	for _, action := range notification.Actions {
		_, eventID, err := parseNotificationAction(action.Arguments)
		if err == nil {
			return eventID
		}
	}
	t.Fatal("notification has no parseable action payload")
	return ""
}

func TestWindowsNotifierNotifyReturnsBeforeBlockedPush(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	n, _ := newWindowsNotifierFixture(t, func(*windowsToast) error {
		close(started)
		<-release
		return nil
	})

	done := make(chan struct{})
	go func() {
		_ = n.Notify(FileEvent{Destination: filepath.Join(t.TempDir(), "file.txt"), Category: "Documents"})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Notify blocked on toast Push")
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("toast Push did not start")
	}
	close(release)
}

func TestWindowsNotifierRemovesEventWhenPushFails(t *testing.T) {
	var notification *windowsToast
	n, _ := newWindowsNotifierFixture(t, func(value *windowsToast) error {
		notification = value
		return errors.New("push failed")
	})

	n.deliver(FileEvent{Destination: filepath.Join(t.TempDir(), "file.txt"), Category: "Documents"})
	if notification == nil {
		t.Fatal("notification was not pushed")
	}
	eventID := eventIDFromNotification(t, notification)
	if _, ok := n.registry.Claim(eventID); ok {
		t.Fatal("failed Push left a claimable event")
	}
}

func TestWindowsNotifierLeavesSuccessfulEventClaimable(t *testing.T) {
	var notification *windowsToast
	n, _ := newWindowsNotifierFixture(t, func(value *windowsToast) error {
		notification = value
		return nil
	})

	path := filepath.Join(t.TempDir(), "file.txt")
	n.deliver(FileEvent{Destination: path, Category: "Documents"})
	eventID := eventIDFromNotification(t, notification)
	event, ok := n.registry.Claim(eventID)
	if !ok || event.CurrentPath != path {
		t.Fatalf("unexpected registered event: %#v, %v", event, ok)
	}
}

func TestWindowsNotifierDoesNotCopyPathDuringDelivery(t *testing.T) {
	n, files := newWindowsNotifierFixture(t, func(*windowsToast) error { return nil })
	n.deliver(FileEvent{Destination: filepath.Join(t.TempDir(), "file.txt"), Category: "Documents"})
	calls, _, _ := files.snapshot()
	if len(calls) != 0 {
		t.Fatalf("delivery executed file actions: %#v", calls)
	}
}

func TestWindowsNotifierConcurrentDeliveriesKeepDistinctEvents(t *testing.T) {
	var mu sync.Mutex
	var notifications []*windowsToast
	n, _ := newWindowsNotifierFixture(t, func(value *windowsToast) error {
		mu.Lock()
		notifications = append(notifications, value)
		mu.Unlock()
		return nil
	})

	paths := []string{
		filepath.Join(t.TempDir(), "first.txt"),
		filepath.Join(t.TempDir(), "second.txt"),
	}
	var wg sync.WaitGroup
	for _, path := range paths {
		path := path
		wg.Add(1)
		go func() {
			defer wg.Done()
			n.deliver(FileEvent{Destination: path, Category: "Documents"})
		}()
	}
	wg.Wait()

	if len(notifications) != 2 {
		t.Fatalf("expected two notifications, got %d", len(notifications))
	}
	seenIDs := map[string]bool{}
	seenPaths := map[string]bool{}
	for _, notification := range notifications {
		eventID := eventIDFromNotification(t, notification)
		if seenIDs[eventID] {
			t.Fatalf("duplicate event ID %q", eventID)
		}
		seenIDs[eventID] = true
		event, ok := n.registry.Claim(eventID)
		if !ok {
			t.Fatalf("event %q is unavailable", eventID)
		}
		seenPaths[event.CurrentPath] = true
	}
	for _, path := range paths {
		if !seenPaths[path] {
			t.Fatalf("missing event path %q in %#v", path, seenPaths)
		}
	}
}
