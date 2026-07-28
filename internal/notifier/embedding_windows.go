//go:build windows

package notifier

import (
	"log"
	"os"
	"path/filepath"
	"time"
)

// RunWindowsEmbeddingHost keeps a COM-activated process alive long enough for
// Windows to deliver a toast activation callback.
func RunWindowsEmbeddingHost(timeout time.Duration) {
	executable, err := os.Executable()
	if err != nil {
		log.Printf("[notifier] embedding executable lookup failed: %v", err)
		return
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		log.Printf("[notifier] embedding executable normalization failed: %v", err)
		return
	}
	if err := initializeWindowsActivation(executable); err != nil {
		log.Printf("[notifier] embedding activation initialization failed: %v", err)
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-windowsActivationState.activated:
	case <-timer.C:
	}
}
