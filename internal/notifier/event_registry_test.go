package notifier

import (
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNotificationEventRegistryLifecycle(t *testing.T) {
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	registry := newNotificationEventRegistry(eventRegistryOptions{
		now:             func() time.Time { return now },
		newID:           func() (string, error) { return strings.Repeat("b", 64), nil },
		ttl:             7 * 24 * time.Hour,
		cleanupInterval: 0,
	})
	t.Cleanup(func() { _ = registry.Close() })

	if _, err := registry.Register("relative/file.txt", "Documents"); err == nil {
		t.Fatal("expected relative path registration to fail")
	}

	path := filepath.Join(t.TempDir(), "file.txt")
	event, err := registry.Register(path, "Documents")
	if err != nil {
		t.Fatal(err)
	}
	if event.ID != strings.Repeat("b", 64) || event.CurrentPath != path || event.Category != "Documents" {
		t.Fatalf("unexpected event: %#v", event)
	}
	if !event.CreatedAt.Equal(now) || !event.ExpiresAt.Equal(now.Add(7*24*time.Hour)) || event.Consumed {
		t.Fatalf("unexpected event timestamps/state: %#v", event)
	}

	claimed, ok := registry.Claim(event.ID)
	if !ok || !claimed.Consumed {
		t.Fatalf("expected first claim to succeed: %#v, %v", claimed, ok)
	}
	if _, ok := registry.Claim(event.ID); ok {
		t.Fatal("expected second claim to fail")
	}

	if err := registry.Close(); err != nil {
		t.Fatal(err)
	}
	if err := registry.Close(); err != nil {
		t.Fatalf("second close failed: %v", err)
	}
}

func TestNotificationEventRegistryRejectsExpiredAndRemovedEvents(t *testing.T) {
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	ids := []string{strings.Repeat("c", 64), strings.Repeat("d", 64)}
	var next atomic.Int32
	registry := newNotificationEventRegistry(eventRegistryOptions{
		now: func() time.Time { return now },
		newID: func() (string, error) {
			return ids[int(next.Add(1))-1], nil
		},
		ttl:             time.Hour,
		cleanupInterval: 0,
	})
	t.Cleanup(func() { _ = registry.Close() })

	expired, err := registry.Register(filepath.Join(t.TempDir(), "expired.txt"), "Documents")
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Hour)
	if _, ok := registry.Claim(expired.ID); ok {
		t.Fatal("expected expired event claim to fail")
	}

	pending, err := registry.Register(filepath.Join(t.TempDir(), "removed.txt"), "Documents")
	if err != nil {
		t.Fatal(err)
	}
	registry.Remove(pending.ID)
	if _, ok := registry.Claim(pending.ID); ok {
		t.Fatal("expected removed event claim to fail")
	}
}

func TestNotificationEventRegistryAllowsOneConcurrentClaim(t *testing.T) {
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	registry := newNotificationEventRegistry(eventRegistryOptions{
		now:             func() time.Time { return now },
		newID:           func() (string, error) { return strings.Repeat("e", 64), nil },
		ttl:             7 * 24 * time.Hour,
		cleanupInterval: 0,
	})
	t.Cleanup(func() { _ = registry.Close() })

	event, err := registry.Register(filepath.Join(t.TempDir(), "file.txt"), "Documents")
	if err != nil {
		t.Fatal(err)
	}

	var successes atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, ok := registry.Claim(event.ID); ok {
				successes.Add(1)
			}
		}()
	}
	wg.Wait()
	if successes.Load() != 1 {
		t.Fatalf("expected one claim, got %d", successes.Load())
	}
}
