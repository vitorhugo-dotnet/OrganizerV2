//go:build windows

package main

import (
	"os"
	"time"

	"github.com/vitorhugo-java/organizerv2/internal/notifier"
)

func init() {
	if !isToastEmbeddingInvocation(os.Args) {
		return
	}
	notifier.RunWindowsEmbeddingHost(10 * time.Second)
	os.Exit(0)
}

func isToastEmbeddingInvocation(args []string) bool {
	for _, arg := range args[1:] {
		if arg == "-Embedding" {
			return true
		}
	}
	return false
}
