package notifier

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vitorhugo-java/organizerv2/internal/config"
)

type fakeFileActionService struct {
	mu               sync.Mutex
	calls            []string
	paths            []string
	destinations     []string
	err              error
	panicOnOperation bool
}

func (f *fakeFileActionService) record(call, path, destination string) error {
	if f.panicOnOperation {
		panic("fake action panic")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, call)
	f.paths = append(f.paths, path)
	f.destinations = append(f.destinations, destination)
	return f.err
}

func (f *fakeFileActionService) OpenFile(path string) error {
	return f.record("open_file", path, "")
}

func (f *fakeFileActionService) OpenLocation(path string) error {
	return f.record("open_location", path, "")
}

func (f *fakeFileActionService) CopyPath(path string) error {
	return f.record("copy_path", path, "")
}

func (f *fakeFileActionService) MoveTo(path, destinationDir string) (string, error) {
	err := f.record("move_to", path, destinationDir)
	return filepath.Join(destinationDir, filepath.Base(path)), err
}

func (f *fakeFileActionService) CopyTo(path, destinationDir string) (string, error) {
	err := f.record("copy_to", path, destinationDir)
	return filepath.Join(destinationDir, filepath.Base(path)), err
}

func (f *fakeFileActionService) snapshot() ([]string, []string, []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...), append([]string(nil), f.paths...), append([]string(nil), f.destinations...)
}

func newActionHandlerFixture(t *testing.T, files fileActionService) (*windowsNotificationActionHandler, *notificationEventRegistry, resolvedShortcut, *[]string) {
	t.Helper()
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	var sequence atomic.Uint64
	registry := newNotificationEventRegistry(eventRegistryOptions{
		now: func() time.Time { return now },
		newID: func() (string, error) {
			return fmt.Sprintf("%064x", sequence.Add(1)), nil
		},
		ttl:             7 * 24 * time.Hour,
		cleanupInterval: 0,
	})
	t.Cleanup(func() { _ = registry.Close() })

	destination := filepath.Join(t.TempDir(), "Destination")
	resolverValue := newNotificationShortcutResolver([]config.Shortcut{
		{ID: "shortcut-id", Name: "Destination", Path: destination},
	})
	shortcut, ok := resolverValue.Resolve("shortcut-id")
	if !ok {
		t.Fatal("fixture shortcut was not resolved")
	}
	logs := []string{}
	handler := &windowsNotificationActionHandler{
		registry:  registry,
		shortcuts: &resolverValue,
		files:     files,
		logf: func(format string, args ...any) {
			logs = append(logs, fmt.Sprintf(format, args...))
		},
	}
	return handler, registry, shortcut, &logs
}

func TestWindowsNotificationActionHandlerRoutesEveryAction(t *testing.T) {
	tests := []struct {
		name             string
		action           notificationAction
		data             []inputValue
		expectedCall     string
		expectedService  bool
		expectsDirectory bool
	}{
		{name: "open file", action: actionOpenFile, expectedCall: "open_file", expectedService: true},
		{name: "open location", action: actionOpenLocation, expectedCall: "open_location", expectedService: true},
		{name: "copy path", action: actionCopyPath, expectedCall: "copy_path", expectedService: true},
		{name: "move to", action: actionMoveTo, data: []inputValue{{Key: destinationInputID, Value: "shortcut-id"}}, expectedCall: "move_to", expectedService: true, expectsDirectory: true},
		{name: "copy to", action: actionCopyTo, data: []inputValue{{Key: destinationInputID, Value: "shortcut-id"}}, expectedCall: "copy_to", expectedService: true, expectsDirectory: true},
		{name: "confirm", action: actionConfirm},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fake := &fakeFileActionService{}
			handler, registry, shortcut, _ := newActionHandlerFixture(t, fake)
			path := filepath.Join(t.TempDir(), "file.txt")
			event, err := registry.Register(path, "Documents")
			if err != nil {
				t.Fatal(err)
			}

			handler.Handle(encodeNotificationAction(test.action, event.ID), test.data)
			calls, paths, destinations := fake.snapshot()
			if !test.expectedService {
				if len(calls) != 0 {
					t.Fatalf("expected no service call, got %#v", calls)
				}
				if _, ok := registry.Claim(event.ID); ok {
					t.Fatal("confirm did not consume the event")
				}
				return
			}
			if len(calls) != 1 || calls[0] != test.expectedCall {
				t.Fatalf("unexpected calls: %#v", calls)
			}
			if paths[0] != event.CurrentPath {
				t.Fatalf("unexpected path: %q", paths[0])
			}
			if test.expectsDirectory && destinations[0] != shortcut.Path {
				t.Fatalf("unexpected destination: %q", destinations[0])
			}
		})
	}
}

func TestWindowsNotificationActionHandlerRejectsInvalidInputsBeforeClaim(t *testing.T) {
	fake := &fakeFileActionService{}
	handler, registry, _, _ := newActionHandlerFixture(t, fake)
	path := filepath.Join(t.TempDir(), "file.txt")
	event, err := registry.Register(path, "Documents")
	if err != nil {
		t.Fatal(err)
	}

	handler.Handle("malformed", nil)
	handler.Handle(encodeNotificationAction(actionMoveTo, event.ID), nil)
	handler.Handle(encodeNotificationAction(actionMoveTo, event.ID), []inputValue{{Key: destinationInputID, Value: "unknown"}})
	handler.Handle(encodeNotificationAction(actionOpenFile, strings.Repeat("f", 64)), nil)

	handler.Handle(encodeNotificationAction(actionOpenFile, event.ID), nil)
	calls, _, _ := fake.snapshot()
	if len(calls) != 1 || calls[0] != "open_file" {
		t.Fatalf("invalid input consumed or executed the event: %#v", calls)
	}
}

func TestWindowsNotificationActionHandlerExecutesDuplicateCallbackOnce(t *testing.T) {
	fake := &fakeFileActionService{}
	handler, registry, _, _ := newActionHandlerFixture(t, fake)
	event, err := registry.Register(filepath.Join(t.TempDir(), "file.txt"), "Documents")
	if err != nil {
		t.Fatal(err)
	}
	args := encodeNotificationAction(actionCopyPath, event.ID)
	handler.Handle(args, nil)
	handler.Handle(args, nil)
	calls, _, _ := fake.snapshot()
	if len(calls) != 1 {
		t.Fatalf("expected one call, got %#v", calls)
	}
}

func TestWindowsNotificationActionHandlerLogsServiceErrorWithoutPanicking(t *testing.T) {
	fake := &fakeFileActionService{err: errors.New("service failed")}
	handler, registry, _, logs := newActionHandlerFixture(t, fake)
	event, err := registry.Register(filepath.Join(t.TempDir(), "file.txt"), "Documents")
	if err != nil {
		t.Fatal(err)
	}
	handler.Handle(encodeNotificationAction(actionOpenFile, event.ID), nil)
	if len(*logs) == 0 || !strings.Contains((*logs)[len(*logs)-1], "service failed") {
		t.Fatalf("expected service error log, got %#v", *logs)
	}
}

func TestWindowsNotificationActionHandlerKeepsConcurrentEventsAssociated(t *testing.T) {
	fake := &fakeFileActionService{}
	handler, registry, _, _ := newActionHandlerFixture(t, fake)
	first, err := registry.Register(filepath.Join(t.TempDir(), "first.txt"), "Documents")
	if err != nil {
		t.Fatal(err)
	}
	second, err := registry.Register(filepath.Join(t.TempDir(), "second.txt"), "Documents")
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for _, event := range []notificationEvent{first, second} {
		event := event
		wg.Add(1)
		go func() {
			defer wg.Done()
			handler.Handle(encodeNotificationAction(actionOpenFile, event.ID), nil)
		}()
	}
	wg.Wait()

	calls, paths, _ := fake.snapshot()
	if len(calls) != 2 || len(paths) != 2 {
		t.Fatalf("unexpected calls: %#v paths: %#v", calls, paths)
	}
	seen := map[string]bool{paths[0]: true, paths[1]: true}
	if !seen[first.CurrentPath] || !seen[second.CurrentPath] {
		t.Fatalf("events crossed paths: %#v", paths)
	}
}

func TestWindowsNotificationActionHandlerRecoversServicePanic(t *testing.T) {
	fake := &fakeFileActionService{panicOnOperation: true}
	handler, registry, _, logs := newActionHandlerFixture(t, fake)
	event, err := registry.Register(filepath.Join(t.TempDir(), "file.txt"), "Documents")
	if err != nil {
		t.Fatal(err)
	}
	handler.Handle(encodeNotificationAction(actionOpenFile, event.ID), nil)
	if len(*logs) == 0 || !strings.Contains((*logs)[len(*logs)-1], "recovered activation panic") {
		t.Fatalf("expected panic recovery log, got %#v", *logs)
	}
}
