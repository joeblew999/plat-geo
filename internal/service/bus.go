package service

import "sync"

// Event represents a resource mutation.
type Event struct {
	Resource string // e.g. "layers"
	Action   string // "created", "updated", "deleted"
	ID       string // resource ID
}

// EventBus is a simple fan-out pub/sub for resource change events.
type EventBus struct {
	mu   sync.RWMutex
	subs map[chan Event]struct{}
}

// NewEventBus creates a new event bus.
func NewEventBus() *EventBus {
	return &EventBus{subs: make(map[chan Event]struct{})}
}

// Publish sends an event to all subscribers (non-blocking).
func (b *EventBus) Publish(e Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.subs {
		select {
		case ch <- e:
		default:
			// subscriber too slow, skip
		}
	}
}

// Subscribe returns a buffered channel that receives events.
func (b *EventBus) Subscribe() chan Event {
	ch := make(chan Event, 16)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

// Unsubscribe removes a subscriber and closes its channel.
func (b *EventBus) Unsubscribe(ch chan Event) {
	b.mu.Lock()
	delete(b.subs, ch)
	b.mu.Unlock()
	close(ch)
}

// DefaultBus is the package-level event bus.
var DefaultBus = NewEventBus()
