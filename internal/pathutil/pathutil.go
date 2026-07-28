package pathutil

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ResolveDestination returns the target path for a file inside a category
// subfolder of targetBase.
func ResolveDestination(targetBase, category, filename string) string {
	return filepath.Join(targetBase, category, filename)
}

// ResolveDuplicate returns a path that does not yet exist on disk. If destPath
// is free it is returned as-is. If it already exists the stem gets " (N)"
// appended until a free name is found (up to 1000 attempts).
func ResolveDuplicate(destPath string) (string, error) {
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		return destPath, nil
	}
	dir := filepath.Dir(destPath)
	base := filepath.Base(destPath)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)

	for n := 2; n <= 1000; n++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s (%d)%s", stem, n, ext))
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("could not find a free filename for %s after 1000 attempts", destPath)
}

// EnsureDir creates dir and all parent directories with mode 0755.
func EnsureDir(dir string) error {
	return os.MkdirAll(dir, 0o755)
}

// CopyFile copies the bytes of src to a newly created dst.
// The parent directory of dst must already exist. Existing destinations are
// never overwritten.
func CopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return fmt.Errorf("stat source: %w", err)
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
	if err != nil {
		return fmt.Errorf("create dest: %w", err)
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(dst)
		return fmt.Errorf("copy data: %w", err)
	}
	if err := out.Close(); err != nil {
		os.Remove(dst)
		return fmt.Errorf("close dest: %w", err)
	}
	return nil
}

// MoveFile moves src to dst. It tries os.Rename first; on any failure it
// falls back to CopyFile then Remove so cross-device moves also work.
func MoveFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	if err := CopyFile(src, dst); err != nil {
		return err
	}
	return os.Remove(src)
}

// ExpandHome replaces a leading "~/" with the current user's home directory.
func ExpandHome(path string) (string, error) {
	if !strings.HasPrefix(path, "~/") && path != "~" {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home dir: %w", err)
	}
	if path == "~" {
		return home, nil
	}
	return filepath.Join(home, path[2:]), nil
}
