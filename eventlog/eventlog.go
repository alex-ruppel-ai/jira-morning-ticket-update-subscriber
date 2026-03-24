package eventlog

import (
	"fmt"
	"sync"
	"time"
)

// Event stores a single event entry
type Event struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	User      string    `json:"user"`
	Channel   string    `json:"channel"`
	Text      string    `json:"text"`
	Timestamp time.Time `json:"timestamp"`
}

var (
	events   []Event
	mu       sync.RWMutex
	maxEvents = 50
)

// Add logs a new event
func Add(eventType, user, channel, text string) {
	mu.Lock()
	defer mu.Unlock()

	event := Event{
		ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
		Type:      eventType,
		User:      user,
		Channel:   channel,
		Text:      text,
		Timestamp: time.Now(),
	}

	events = append([]Event{event}, events...)

	if len(events) > maxEvents {
		events = events[:maxEvents]
	}
}

// GetRecent returns recent events
func GetRecent() []Event {
	mu.RLock()
	defer mu.RUnlock()

	result := make([]Event, len(events))
	copy(result, events)
	return result
}
