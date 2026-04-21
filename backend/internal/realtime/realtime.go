// Package realtime provides a lightweight in-process pub/sub hub for
// broadcasting calendar-change events to live subscribers (SSE clients).
//
// The hub is transport-agnostic: publishers call Publish, subscribers drain
// the returned channel. Wire format (SSE frames, WebSocket frames, etc.) is
// the transport layer's concern.
package realtime

import (
	"log/slog"
	"sync"
)

// Event types emitted by the service layer. Exposed as constants so callers
// do not have to repeat string literals.
const (
	EventAvailabilityChanged = "availability.changed"
	EventParticipantJoined   = "participant.joined"
	EventParticipantRemoved  = "participant.removed"
	EventParticipantClaimed  = "participant.claimed"
	EventParticipantRenamed  = "participant.renamed"
	EventCalendarUpdated     = "calendar.updated"
	EventCalendarFinalised   = "calendar.finalised"
	EventCalendarUnfinalised = "calendar.unfinalised"
)

// Event is the payload delivered to subscribers. Intentionally minimal: the
// client's reaction is always "refetch", so we only need a pointer.
type Event struct {
	Type       string
	CalendarID string
}

// subscriberChannelCapacity bounds per-subscriber buffering. A slow client
// is not allowed to back-pressure publishers; once the buffer is full the
// event is dropped (and logged at debug).
const subscriberChannelCapacity = 16

type subscriber struct {
	ch chan Event
}

// Hub is an in-memory pub/sub keyed by calendar ID. Safe for concurrent use.
// No goroutine is owned by the hub itself.
type Hub struct {
	mu     sync.RWMutex
	subs   map[string]map[*subscriber]struct{}
	logger *slog.Logger
}

// NewHub constructs a Hub. If logger is nil, slog.Default is used.
func NewHub(logger *slog.Logger) *Hub {
	if logger == nil {
		logger = slog.Default()
	}
	return &Hub{
		subs:   make(map[string]map[*subscriber]struct{}),
		logger: logger,
	}
}

// Publish fans an event out to all current subscribers of calendarID. Drops
// (with a debug log) for any subscriber whose buffer is full.
//
// Safe to call from any goroutine. Never blocks.
func (h *Hub) Publish(calendarID, eventType string) {
	if calendarID == "" || eventType == "" {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()

	set, ok := h.subs[calendarID]
	if !ok {
		return
	}
	ev := Event{Type: eventType, CalendarID: calendarID}
	for s := range set {
		select {
		case s.ch <- ev:
		default:
			h.logger.Debug("realtime: dropping event, subscriber buffer full",
				"calendar_id", calendarID, "event_type", eventType)
		}
	}
}

// Subscribe registers a new subscriber for calendarID and returns the
// receive-only channel plus an unsubscribe function. The unsubscribe function
// is idempotent; call it via defer to guarantee cleanup.
func (h *Hub) Subscribe(calendarID string) (<-chan Event, func()) {
	s := &subscriber{ch: make(chan Event, subscriberChannelCapacity)}

	h.mu.Lock()
	set, ok := h.subs[calendarID]
	if !ok {
		set = make(map[*subscriber]struct{})
		h.subs[calendarID] = set
	}
	set[s] = struct{}{}
	h.mu.Unlock()

	var once sync.Once
	unsubscribe := func() {
		once.Do(func() {
			h.mu.Lock()
			if set, ok := h.subs[calendarID]; ok {
				delete(set, s)
				if len(set) == 0 {
					delete(h.subs, calendarID)
				}
			}
			h.mu.Unlock()
			close(s.ch)
		})
	}
	return s.ch, unsubscribe
}

// ActiveSubscribers returns the total number of live subscribers across all
// calendars. Cheap: a read-locked sum.
func (h *Hub) ActiveSubscribers() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	n := 0
	for _, set := range h.subs {
		n += len(set)
	}
	return n
}

// ActiveCalendars returns the number of distinct calendars that currently
// have at least one subscriber.
func (h *Hub) ActiveCalendars() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.subs)
}
