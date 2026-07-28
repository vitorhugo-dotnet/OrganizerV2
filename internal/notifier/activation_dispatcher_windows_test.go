//go:build windows

package notifier

import (
	"strings"
	"testing"

	"git.sr.ht/~jackmordaunt/go-toast/v2"
)

type recordingActivationHandler struct {
	args  string
	data  []inputValue
	calls int
	panic bool
}

func (h *recordingActivationHandler) Handle(args string, data []inputValue) {
	h.calls++
	if h.panic {
		panic("activation handler panic")
	}
	h.args = args
	h.data = append([]inputValue(nil), data...)
}

func TestDispatchWindowsActivationConvertsUserData(t *testing.T) {
	handler := &recordingActivationHandler{}
	installWindowsActivationHandler(handler)
	t.Cleanup(func() { clearWindowsActivationHandler(handler) })

	args := "v1|confirm|" + strings.Repeat("c", 64)
	dispatchWindowsActivation(args, []toast.UserData{
		{Key: destinationInputID, Value: "shortcut-id"},
	})

	if handler.calls != 1 || handler.args != args {
		t.Fatalf("unexpected dispatch: calls=%d args=%q", handler.calls, handler.args)
	}
	if len(handler.data) != 1 || handler.data[0].Key != destinationInputID || handler.data[0].Value != "shortcut-id" {
		t.Fatalf("unexpected data: %#v", handler.data)
	}
}

func TestDispatchWindowsActivationWithoutHandlerIsSafe(t *testing.T) {
	windowsActivationState.mu.Lock()
	previous := windowsActivationState.handler
	windowsActivationState.handler = nil
	windowsActivationState.mu.Unlock()
	t.Cleanup(func() {
		windowsActivationState.mu.Lock()
		windowsActivationState.handler = previous
		windowsActivationState.mu.Unlock()
	})

	dispatchWindowsActivation("ignored", nil)
}

func TestInstallWindowsActivationHandlerReplacesHandler(t *testing.T) {
	first := &recordingActivationHandler{}
	second := &recordingActivationHandler{}
	installWindowsActivationHandler(first)
	installWindowsActivationHandler(second)
	t.Cleanup(func() { clearWindowsActivationHandler(second) })

	dispatchWindowsActivation("args", nil)
	if first.calls != 0 || second.calls != 1 {
		t.Fatalf("unexpected calls: first=%d second=%d", first.calls, second.calls)
	}
}

func TestClearWindowsActivationHandlerOnlyClearsSameInstance(t *testing.T) {
	first := &recordingActivationHandler{}
	second := &recordingActivationHandler{}
	installWindowsActivationHandler(first)
	clearWindowsActivationHandler(second)
	t.Cleanup(func() { clearWindowsActivationHandler(first) })

	dispatchWindowsActivation("args", nil)
	if first.calls != 1 {
		t.Fatalf("different handler cleared active handler: calls=%d", first.calls)
	}
}

func TestDispatchWindowsActivationRecoversHandlerPanic(t *testing.T) {
	handler := &recordingActivationHandler{panic: true}
	installWindowsActivationHandler(handler)
	t.Cleanup(func() { clearWindowsActivationHandler(handler) })

	dispatchWindowsActivation("args", nil)
	if handler.calls != 1 {
		t.Fatalf("expected handler invocation, got %d", handler.calls)
	}
}
