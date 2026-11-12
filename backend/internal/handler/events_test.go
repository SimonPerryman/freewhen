package handler

import (
	"bufio"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/freewhen/backend/internal/model"
	"github.com/freewhen/backend/internal/realtime"
)

// mockHub implements Subscriber for the events handler tests.
type mockHub struct {
	mu   sync.Mutex
	subs map[string][]chan realtime.Event
}

func newMockHub() *mockHub {
	return &mockHub{subs: make(map[string][]chan realtime.Event)}
}

func (m *mockHub) Subscribe(calendarID string) (<-chan realtime.Event, func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ch := make(chan realtime.Event, 16)
	m.subs[calendarID] = append(m.subs[calendarID], ch)
	return ch, func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		existing := m.subs[calendarID]
		for i, c := range existing {
			if c == ch {
				m.subs[calendarID] = append(existing[:i], existing[i+1:]...)
				close(ch)
				return
			}
		}
	}
}

func (m *mockHub) Publish(calendarID, eventType string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, ch := range m.subs[calendarID] {
		ch <- realtime.Event{Type: eventType, CalendarID: calendarID}
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func calendarFoundService() *mockService {
	return &mockService{
		getCalendarOnly: func(_ context.Context, id string) (model.Calendar, error) {
			return model.Calendar{ID: id}, nil
		},
	}
}

func TestEvents_Stream_404WhenCalendarMissing(t *testing.T) {
	svc := &mockService{
		getCalendarOnly: func(_ context.Context, _ string) (model.Calendar, error) {
			return model.Calendar{}, model.ErrNotFound
		},
	}
	h := NewEvents(newMockHub(), svc, discardLogger())

	req := httptest.NewRequest(http.MethodGet, "/calendars/missing/events", nil)
	req.SetPathValue("id", "missing")
	rec := httptest.NewRecorder()

	h.Stream(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestEvents_Stream_DeliversEventFrame(t *testing.T) {
	hub := newMockHub()
	svc := calendarFoundService()
	h := NewEvents(hub, svc, discardLogger())

	// Shorter heartbeat not needed — we'll emit a real event and close.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/calendars/cal-1/events", nil).WithContext(ctx)
	req.SetPathValue("id", "cal-1")

	// Use an httptest.Server so Flusher works properly.
	srv := httptest.NewServer(http.HandlerFunc(h.Stream))
	defer srv.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	// httptest.NewServer doesn't run mux path parameters; wrap the handler
	// in a mux so `GET /calendars/{id}/events` resolves correctly.
	mux := http.NewServeMux()
	mux.HandleFunc("GET /calendars/{id}/events", h.Stream)
	srv.Config.Handler = mux

	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/calendars/cal-1/events", nil)

	resp, err := client.Do(httpReq)
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("Content-Type = %q, want text/event-stream", ct)
	}

	// Read initial heartbeat so we know Subscribe has happened before we
	// publish. Otherwise we'd race against the handler's goroutine.
	reader := bufio.NewReader(resp.Body)
	if err := waitForPing(reader); err != nil {
		t.Fatalf("waiting for heartbeat: %v", err)
	}

	hub.Publish("cal-1", realtime.EventAvailabilityChanged)

	got, err := readEventFrame(reader)
	if err != nil {
		t.Fatalf("reading event frame: %v", err)
	}
	if got.event != realtime.EventAvailabilityChanged {
		t.Errorf("event = %q, want %q", got.event, realtime.EventAvailabilityChanged)
	}
	if !strings.Contains(got.data, `"calendar_id":"cal-1"`) {
		t.Errorf("data = %q, missing calendar_id", got.data)
	}
}

func TestEvents_Stream_MultipleSubscribersReceiveSameEvent(t *testing.T) {
	hub := newMockHub()
	svc := calendarFoundService()
	h := NewEvents(hub, svc, discardLogger())

	mux := http.NewServeMux()
	mux.HandleFunc("GET /calendars/{id}/events", h.Stream)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	makeClient := func() *bufio.Reader {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/calendars/cal-1/events", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET failed: %v", err)
		}
		t.Cleanup(func() { _ = resp.Body.Close() })
		r := bufio.NewReader(resp.Body)
		if err := waitForPing(r); err != nil {
			t.Fatalf("waiting for heartbeat: %v", err)
		}
		return r
	}

	r1 := makeClient()
	r2 := makeClient()

	hub.Publish("cal-1", realtime.EventParticipantJoined)

	var got atomic.Int32
	var wg sync.WaitGroup
	for _, r := range []*bufio.Reader{r1, r2} {
		wg.Add(1)
		go func(reader *bufio.Reader) {
			defer wg.Done()
			frame, err := readEventFrame(reader)
			if err == nil && frame.event == realtime.EventParticipantJoined {
				got.Add(1)
			}
		}(r)
	}
	wg.Wait()
	if got.Load() != 2 {
		t.Fatalf("received by %d subscribers, want 2", got.Load())
	}
}

// --- helpers for parsing SSE frames ---

type frame struct {
	event string
	data  string
}

// waitForPing consumes lines until it sees ": ping".
func waitForPing(r *bufio.Reader) error {
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		line, err := r.ReadString('\n')
		if err != nil {
			return err
		}
		if strings.HasPrefix(line, ": ping") {
			return nil
		}
	}
	return context.DeadlineExceeded
}

// readEventFrame reads up to an empty-line terminator and extracts the
// event + data fields.
func readEventFrame(r *bufio.Reader) (frame, error) {
	var f frame
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return f, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			if f.event != "" || f.data != "" {
				return f, nil
			}
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		if strings.HasPrefix(line, "event: ") {
			f.event = strings.TrimPrefix(line, "event: ")
		} else if strings.HasPrefix(line, "data: ") {
			if f.data != "" {
				f.data += "\n"
			}
			f.data += strings.TrimPrefix(line, "data: ")
		}
	}
}
