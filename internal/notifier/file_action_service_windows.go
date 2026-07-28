//go:build windows

package notifier

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"unsafe"

	"github.com/vitorhugo-java/organizerv2/internal/pathutil"
	"golang.design/x/clipboard"
	"golang.org/x/sys/windows"
)

type processStarter func(name string, args ...string) error
type clipboardWriter func(text string) error
type shellOpener func(path string) error

type windowsFileActionService struct {
	mu             sync.Mutex
	openFile       shellOpener
	startProcess   processStarter
	writeClipboard clipboardWriter
}

func newWindowsFileActionService(copyPathEnabled bool) *windowsFileActionService {
	service := &windowsFileActionService{
		openFile: shellExecuteOpen,
		startProcess: func(name string, args ...string) error {
			return exec.Command(name, args...).Start()
		},
		writeClipboard: func(string) error {
			return errors.New("clipboard action is disabled")
		},
	}
	if !copyPathEnabled {
		return service
	}
	if err := clipboard.Init(); err != nil {
		service.writeClipboard = func(string) error {
			return fmt.Errorf("initialize clipboard: %w", err)
		}
		return service
	}
	service.writeClipboard = func(text string) error {
		clipboard.Write(clipboard.FmtText, []byte(text))
		return nil
	}
	return service
}

func (s *windowsFileActionService) OpenFile(path string) error {
	if err := validateAbsolutePath(path); err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return errors.New("path is not a regular file")
	}
	if s.openFile == nil {
		return errors.New("file opener is unavailable")
	}
	return s.openFile(path)
}

func (s *windowsFileActionService) OpenLocation(path string) error {
	if err := validateAbsolutePath(path); err != nil {
		return err
	}
	if s.startProcess == nil {
		return errors.New("process starter is unavailable")
	}
	return s.startProcess("explorer.exe", `/select,"`+path+`"`)
}

func (s *windowsFileActionService) CopyPath(path string) error {
	if err := validateAbsolutePath(path); err != nil {
		return err
	}
	if s.writeClipboard == nil {
		return errors.New("clipboard writer is unavailable")
	}
	return s.writeClipboard(path)
}

func (s *windowsFileActionService) MoveTo(path, destinationDir string) (string, error) {
	return s.transfer(path, destinationDir, true)
}

func (s *windowsFileActionService) CopyTo(path, destinationDir string) (string, error) {
	return s.transfer(path, destinationDir, false)
}

func (s *windowsFileActionService) transfer(path, destinationDir string, move bool) (string, error) {
	if !filepath.IsAbs(path) || !filepath.IsAbs(destinationDir) {
		return "", errors.New("source and destination must be absolute")
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("stat source: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("source is not a regular file")
	}
	if err := pathutil.EnsureDir(destinationDir); err != nil {
		return "", fmt.Errorf("create destination: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	raw := filepath.Join(destinationDir, filepath.Base(path))
	for attempt := 0; attempt < 1000; attempt++ {
		destination, err := pathutil.ResolveDuplicate(raw)
		if err != nil {
			return "", err
		}
		if move {
			err = pathutil.MoveFile(path, destination)
		} else {
			err = pathutil.CopyFile(path, destination)
		}
		if err == nil {
			return destination, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return "", err
		}
	}
	return "", fmt.Errorf("could not reserve destination for %s", filepath.Base(path))
}

func validateAbsolutePath(path string) error {
	if !filepath.IsAbs(path) {
		return errors.New("path must be absolute")
	}
	return nil
}

var shellExecuteW = windows.NewLazySystemDLL("shell32.dll").NewProc("ShellExecuteW")

func shellExecuteOpen(path string) error {
	operation, err := windows.UTF16PtrFromString("open")
	if err != nil {
		return fmt.Errorf("encode shell operation: %w", err)
	}
	file, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return fmt.Errorf("encode file path: %w", err)
	}
	result, _, _ := shellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(operation)),
		uintptr(unsafe.Pointer(file)),
		0,
		0,
		1,
	)
	if result <= 32 {
		return fmt.Errorf("ShellExecuteW failed with code %d", result)
	}
	return nil
}
