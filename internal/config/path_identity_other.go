//go:build !windows

package config

func shortcutPathIdentity(path string) string {
	return path
}
