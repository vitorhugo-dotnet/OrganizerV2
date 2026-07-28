package notifier

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"sync"
	"time"
)

type notificationEvent struct {
	ID          string
	CurrentPath string
	Category    string
	CreatedAt   time.Time
	ExpiresAt   time.Time
	Consumed    bool
}

type eventRegistryOptions struct {
	now             func() time.Time
	newID           func() (string, error)
	ttl             time.Duration
	cleanupInterval time.Duration
}

type notificationEventRegistry struct {
	mu      sync.Mutex
	events  map[string]notificationEvent
	options eventRegistryOptions
	stop    chan struct{}
	done    chan struct{}
	once    sync.Once
}

func defaultEventRegistryOptions() eventRegistryOptions {
	return eventRegistryOptions{
		now:             time.Now,
		newID:           randomEventID,
		ttl:             7 * 24 * time.Hour,
		cleanupInterval: time.Hour,
	}
}

func newNotificationEventRegistry(options eventRegistryOptions) *notificationEventRegistry {
	if options.now == nil {
		options.now = time.Now
	}
	if options.newID == nil {
		options.newID = randomEventID
	}
	if options.ttl <= 0 {
		options.ttl = 7 * 24 * time.Hour
	}

	registry := &notificationEventRegistry{
		events:  make(map[string]notificationEvent),
		options: options,
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
	}
	if options.cleanupInterval > 0 {
		go registry.cleanupLoop()
	} else {
		close(registry.done)
	}
	return registry
}

func randomEventID() (string, error) {
	data := make([]byte, 32)
	if _, err := rand.Read(data); err != nil {
		return "", fmt.Errorf("generate notification event ID: %w", err)
	}
	return hex.EncodeToString(data), nil
}

func (r *notificationEventRegistry) Register(path, category string) (notificationEvent, error) {
	if !filepath.IsAbs(path) {
		return notificationEvent{}, fmt.Errorf("notification event path must be absolute")
	}
	id, err := r.options.newID()
	if err != nil {
		return notificationEvent{}, err
	}
	if len(id) != 64 {
		return notificationEvent{}, fmt.Errorf("notification event ID must contain 64 hexadecimal characters")
	}
	if _, err := hex.DecodeString(id); err != nil {
		return notificationEvent{}, fmt.Errorf("notification event ID must be hexadecimal: %w", err)
	}

	now := r.options.now()
	event := notificationEvent{
		ID:          id,
		CurrentPath: filepath.Clean(path),
		Category:    category,
		CreatedAt:   now,
		ExpiresAt:   now.Add(r.options.ttl),
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.events[id]; exists {
		return notificationEvent{}, fmt.Errorf("duplicate notification event ID")
	}
	r.events[id] = event
	return event, nil
}

func (r *notificationEventRegistry) Claim(id string) (notificationEvent, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	event, exists := r.events[id]
	if !exists || event.Consumed {
		return notificationEvent{}, false
	}
	if !event.ExpiresAt.After(r.options.now()) {
		delete(r.events, id)
		return notificationEvent{}, false
	}
	event.Consumed = true
	r.events[id] = event
	return event, true
}

func (r *notificationEventRegistry) Remove(id string) {
	r.mu.Lock()
	delete(r.events, id)
	r.mu.Unlock()
}

func (r *notificationEventRegistry) Close() error {
	r.once.Do(func() {
		close(r.stop)
	})
	<-r.done
	return nil
}

func (r *notificationEventRegistry) cleanupLoop() {
	defer close(r.done)
	ticker := time.NewTicker(r.options.cleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			now := r.options.now()
			r.mu.Lock()
			for id, event := range r.events {
				if !event.ExpiresAt.After(now) {
					delete(r.events, id)
				}
			}
			r.mu.Unlock()
		case <-r.stop:
			return
		}
	}
}
