package handler

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/freewhen/backend/internal/realtime"
	"github.com/freewhen/backend/internal/store"
)

// heartbeatInterval is how often the transport emits a keep-alive frame.
// 25s stays comfortably under the 30–60s idle-timeout of most proxies.
const heartbeatInterval = 25 * time.Second

// Subscriber decouples the events handler from a concrete hub implementation.
// realtime.Hub satisfies it.
type Subscriber interface {
	Subscribe(calendarID string) (<-chan realtime.Event, func())
}

// EventTransport abstracts the wire format (SSE today, potentially WebSocket
// later). The handler loop is byte-identical between transports.
type EventTransport interface {
	WriteEvent(e realtime.Event) error
	WriteHeartbeat() error
}

// EventsHandler streams realtime.Event values to connected clients over the
// configured EventTransport. Construction keeps the transport factory
// injectable so tests can swap in a recorder.
type EventsHandler struct {
	hub    Subscriber
	svc    CalendarService
	logger *slog.Logger
	newTxp func(http.ResponseWriter) (EventTransport, error)
	clock  func() time.Time
}

// NewEvents wires an EventsHandler over the default SSE transport.
func NewEvents(hub Subscriber, svc CalendarService, logger *slog.Logger) *EventsHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &EventsHandler{
		hub:    hub,
		svc:    svc,
		logger: logger,
		newTxp: newSSETransport,
		clock:  time.Now,
	}
}

// Stream handles GET /calendars/{id}/events.
func (h *EventsHandler) Stream(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// Cheap 404 check — the stream should not open if the calendar does not
	// exist. We use the non-viewed fetch so visibility rules don't interfere.
	if _, err := h.svc.GetCalendarOnly(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "calendar not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to load calendar")
		return
	}

	txp, err := h.newTxp(w)
	if err != nil {
		// Streaming isn't supported by this ResponseWriter (e.g. a buffered
		// test recorder). Fall through as a plain 500 — the client falls
		// back to polling.
		respondError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	ch, unsubscribe := h.hub.Subscribe(id)
	defer unsubscribe()

	openedAt := h.clock()
	h.logger.Info("sse_open", "calendar_id", id)
	defer func() {
		h.logger.Info("sse_close",
			"calendar_id", id,
			"duration_ms", h.clock().Sub(openedAt).Milliseconds(),
		)
	}()

	// Initial heartbeat confirms the stream is live so clients can flip
	// status to "open" promptly.
	if err := txp.WriteHeartbeat(); err != nil {
		return
	}

	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			if err := txp.WriteEvent(ev); err != nil {
				return
			}
		case <-ticker.C:
			if err := txp.WriteHeartbeat(); err != nil {
				return
			}
		}
	}
}

// --- SSE transport ---

type sseTransport struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

func newSSETransport(w http.ResponseWriter) (EventTransport, error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, errors.New("response writer does not support flushing")
	}
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache, no-transform")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()
	return &sseTransport{w: w, flusher: flusher}, nil
}

func (s *sseTransport) WriteEvent(e realtime.Event) error {
	if _, err := fmt.Fprintf(s.w, "event: %s\ndata: {\"calendar_id\":%q}\n\n", e.Type, e.CalendarID); err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}

func (s *sseTransport) WriteHeartbeat() error {
	if _, err := io.WriteString(s.w, ": ping\n\n"); err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}
