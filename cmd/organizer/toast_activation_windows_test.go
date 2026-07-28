//go:build windows

package main

import "testing"

func TestIsToastEmbeddingInvocation(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{name: "embedding", args: []string{"organizer.exe", "-Embedding"}, want: true},
		{name: "normal start", args: []string{"organizer.exe", "start"}, want: false},
		{name: "wrong case", args: []string{"organizer.exe", "--embedding"}, want: false},
		{name: "substring", args: []string{"organizer.exe", "x-Embedding"}, want: false},
		{name: "later argument", args: []string{"organizer.exe", "--show-terminal", "-Embedding"}, want: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := isToastEmbeddingInvocation(test.args); got != test.want {
				t.Fatalf("args=%v got=%v want=%v", test.args, got, test.want)
			}
		})
	}
}
