//go:build windows

package config

import "strings"

func shortcutPathIdentity(path string) string {
	return strings.ToLower(path)
}
