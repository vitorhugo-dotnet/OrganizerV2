//go:build windows

package notifier

import (
	"log"
	"sync"

	"git.sr.ht/~jackmordaunt/go-toast/v2"
)

type activationHandler interface {
	Handle(args string, data []inputValue)
}

var windowsActivationState struct {
	once       sync.Once
	mu         sync.RWMutex
	handler    activationHandler
	activated  chan struct{}
	initialize error
}

func initializeWindowsActivation(executable string) error {
	windowsActivationState.once.Do(func() {
		windowsActivationState.activated = make(chan struct{}, 1)
		windowsActivationState.initialize = toast.SetAppData(toast.AppData{
			AppID:         windowsToastAppID,
			GUID:          windowsToastGUID,
			ActivationExe: executable,
		})
		toast.SetActivationCallback(dispatchWindowsActivation)
	})
	return windowsActivationState.initialize
}

func installWindowsActivationHandler(handler activationHandler) {
	windowsActivationState.mu.Lock()
	windowsActivationState.handler = handler
	windowsActivationState.mu.Unlock()
}

func clearWindowsActivationHandler(handler activationHandler) {
	windowsActivationState.mu.Lock()
	if windowsActivationState.handler == handler {
		windowsActivationState.handler = nil
	}
	windowsActivationState.mu.Unlock()
}

func dispatchWindowsActivation(args string, data []toast.UserData) {
	values := make([]inputValue, len(data))
	for i, value := range data {
		values[i] = inputValue{Key: value.Key, Value: value.Value}
	}

	select {
	case windowsActivationState.activated <- struct{}{}:
	default:
	}

	windowsActivationState.mu.RLock()
	handler := windowsActivationState.handler
	windowsActivationState.mu.RUnlock()
	if handler == nil {
		return
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			log.Printf("[notifier] recovered dispatcher panic: %v", recovered)
		}
	}()
	handler.Handle(args, values)
}
