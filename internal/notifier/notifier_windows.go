//go:build windows

package notifier

import (
	"log"
	"os"
	"path/filepath"
	"sync"

	"git.sr.ht/~jackmordaunt/go-toast/v2"
	"github.com/vitorhugo-java/organizerv2/internal/config"
)

type toastPusher func(*toast.Notification) error

type windowsNotifier struct {
	cfg        config.NotificationConfig
	executable string
	registry   *notificationEventRegistry
	shortcuts  *notificationShortcutResolver
	handler    *windowsNotificationActionHandler
	files      fileActionService
	push       toastPusher
	closeOnce  sync.Once
	closeErr   error
}

func newPlatform(cfg config.NotificationConfig) Notifier {
	executable, err := os.Executable()
	if err != nil {
		log.Printf("[notifier] executable lookup failed: %v", err)
	}
	if executable != "" {
		absolute, absErr := filepath.Abs(executable)
		if absErr != nil {
			log.Printf("[notifier] executable normalization failed: %v", absErr)
		} else {
			executable = absolute
		}
	}

	registry := newNotificationEventRegistry(defaultEventRegistryOptions())
	resolver := newNotificationShortcutResolver(cfg.Shortcuts)
	files := newWindowsFileActionService(cfg.Actions.CopyPath)
	handler := &windowsNotificationActionHandler{
		registry:  registry,
		shortcuts: &resolver,
		files:     files,
		logf:      log.Printf,
	}
	if err := initializeWindowsActivation(executable); err != nil {
		log.Printf("[notifier] activation initialization failed: %v", err)
	}
	installWindowsActivationHandler(handler)

	return &windowsNotifier{
		cfg:        cfg,
		executable: executable,
		registry:   registry,
		shortcuts:  &resolver,
		handler:    handler,
		files:      files,
		push: func(notification *toast.Notification) error {
			return notification.Push()
		},
	}
}

func (n *windowsNotifier) Notify(event FileEvent) error {
	go n.deliver(event)
	return nil
}

func (n *windowsNotifier) deliver(event FileEvent) {
	destination, err := filepath.Abs(event.Destination)
	if err != nil {
		log.Printf("[notifier] destination normalization failed: %v", err)
		return
	}
	registered, err := n.registry.Register(destination, event.Category)
	if err != nil {
		log.Printf("[notifier] event registration failed: %v", err)
		return
	}

	var shortcuts []resolvedShortcut
	if n.shortcuts != nil {
		shortcuts = n.shortcuts.All()
	}
	notification := buildWindowsToast(registered, n.cfg, shortcuts, n.executable)
	if n.push == nil {
		n.registry.Remove(registered.ID)
		log.Printf("[notifier] toast pusher is unavailable")
		return
	}
	if err := n.push(&notification); err != nil {
		n.registry.Remove(registered.ID)
		log.Printf("[notifier] toast push error: %v", err)
	}
}

func (n *windowsNotifier) Close() error {
	n.closeOnce.Do(func() {
		clearWindowsActivationHandler(n.handler)
		if n.registry != nil {
			n.closeErr = n.registry.Close()
		}
	})
	return n.closeErr
}
