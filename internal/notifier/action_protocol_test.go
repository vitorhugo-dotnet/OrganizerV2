package notifier

import (
	"strings"
	"testing"
)

func TestNotificationActionRoundTrip(t *testing.T) {
	eventID := strings.Repeat("a", 64)
	actions := []notificationAction{
		actionOpenFile,
		actionOpenLocation,
		actionCopyPath,
		actionMoveTo,
		actionCopyTo,
		actionConfirm,
	}
	for _, action := range actions {
		raw := encodeNotificationAction(action, eventID)
		gotAction, gotEventID, err := parseNotificationAction(raw)
		if err != nil {
			t.Fatalf("parse %q: %v", raw, err)
		}
		if gotAction != action || gotEventID != eventID {
			t.Fatalf("got (%q, %q), want (%q, %q)", gotAction, gotEventID, action, eventID)
		}
	}
}

func TestNotificationActionRejectsInvalidPayloads(t *testing.T) {
	validID := strings.Repeat("a", 64)
	tests := map[string]string{
		"wrong version":  "v2|open_file|" + validID,
		"unknown action": "v1|execute|" + validID,
		"non hex id":     "v1|open_file|" + strings.Repeat("z", 64),
		"short id":       "v1|open_file|" + strings.Repeat("a", 62),
		"long id":        "v1|open_file|" + strings.Repeat("a", 66),
		"missing part":   "v1|open_file",
		"extra part":     "v1|open_file|" + validID + "|extra",
	}
	for name, raw := range tests {
		t.Run(name, func(t *testing.T) {
			if _, _, err := parseNotificationAction(raw); err == nil {
				t.Fatalf("expected %q to be rejected", raw)
			}
		})
	}
}

func TestSelectedDestination(t *testing.T) {
	got, err := selectedDestination([]inputValue{
		{Key: "other", Value: "ignored"},
		{Key: destinationInputID, Value: "shortcut-id"},
	})
	if err != nil || got != "shortcut-id" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestSelectedDestinationRejectsMissingEmptyOrDuplicateValues(t *testing.T) {
	tests := map[string][]inputValue{
		"missing":   {{Key: "other", Value: "ignored"}},
		"empty":     {{Key: destinationInputID, Value: ""}},
		"duplicate": {{Key: destinationInputID, Value: "one"}, {Key: destinationInputID, Value: "two"}},
	}
	for name, values := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := selectedDestination(values); err == nil {
				t.Fatal("expected selection to be rejected")
			}
		})
	}
}
