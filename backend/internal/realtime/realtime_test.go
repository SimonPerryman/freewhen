package realtime

import (
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"
)

func newTestHub() *Hub {
	return NewHub(slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestPublish_NoSubscribers_NoOp(t *testing.T) {
	h := newTestHub()
	// Should not panic or block.
	h.Publish("cal-1", EventAvailabilityChanged)
}

func TestPublish_FansOutToSubscribersForSameCalendar(t *testing.T) {
	h := newTestHub()
	ch1, unsub1 := h.Subscribe("cal-1")
	defer unsub1()
	ch2, unsub2 := h.Subscribe("cal-1")
	defer unsub2()

	h.Publish("cal-1", EventAvailabilityChanged)

	select {
	case ev := <-ch1:
		if ev.Type != EventAvailabilityChanged || ev.CalendarID != "cal-1" {
			t.Fatalf("ch1 got %+v", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("ch1 did not receive event")
	}

	select {
	case ev := <-ch2:
		if ev.Type != EventAvailabilityChanged || ev.CalendarID != "cal-1" {
			t.Fatalf("ch2 got %+v", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("ch2 did not receive event")
	}
}

func TestPublish_DoesNotCrossCalendars(t *testing.T) {
	h := newTestHub()
	chA, unsubA := h.Subscribe("cal-A")
	defer unsubA()
	chB, unsubB := h.Subscribe("cal-B")
	defer unsubB()

	h.Publish("cal-A", EventCalendarUpdated)

	select {
	case <-chA:
	case <-time.After(time.Second):
		t.Fatal("chA did not receive event")
	}

	select {
	case ev := <-chB:
		t.Fatalf("chB unexpectedly received event %+v", ev)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestUnsubscribe_RemovesChannelAndCleansUp(t *testing.T) {
	h := newTestHub()
	ch, unsub := h.Subscribe("cal-1")

	unsub()

	// Channel should be closed.
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("expected channel to be closed after unsubscribe")
		}
	case <-time.After(time.Second):
		t.Fatal("channel not closed after unsubscribe")
	}

	// Subsequent publish must not panic (sending on closed chan would panic
	// — confirms unsubscribe actually removed the channel).
	h.Publish("cal-1", EventAvailabilityChanged)

	if h.ActiveSubscribers() != 0 {
		t.Fatalf("ActiveSubscribers = %d, want 0", h.ActiveSubscribers())
	}
	if h.ActiveCalendars() != 0 {
		t.Fatalf("ActiveCalendars = %d, want 0", h.ActiveCalendars())
	}

	// Unsubscribe should be idempotent.
	unsub()
}

func TestPublish_FullBufferDrops_DoesNotBlock(t *testing.T) {
	h := newTestHub()
	_, unsub := h.Subscribe("cal-1")
	defer unsub()

	// Fill the buffer past capacity; publish must not block.
	done := make(chan struct{})
	go func() {
		for i := 0; i < subscriberChannelCapacity*4; i++ {
			h.Publish("cal-1", EventAvailabilityChanged)
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Publish blocked when subscriber buffer was full")
	}
}

func TestHub_ConcurrentSubscribePublish(t *testing.T) {
	h := newTestHub()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, unsub := h.Subscribe("cal-1")
			defer unsub()
			h.Publish("cal-1", EventParticipantJoined)
		}()
	}
	wg.Wait()
	if h.ActiveSubscribers() != 0 {
		t.Fatalf("ActiveSubscribers = %d, want 0", h.ActiveSubscribers())
	}
}

func TestActiveCounts(t *testing.T) {
	h := newTestHub()
	_, u1 := h.Subscribe("cal-1")
	_, u2 := h.Subscribe("cal-1")
	_, u3 := h.Subscribe("cal-2")

	if got := h.ActiveSubscribers(); got != 3 {
		t.Errorf("ActiveSubscribers = %d, want 3", got)
	}
	if got := h.ActiveCalendars(); got != 2 {
		t.Errorf("ActiveCalendars = %d, want 2", got)
	}

	u1()
	u2()
	if got := h.ActiveCalendars(); got != 1 {
		t.Errorf("after cal-1 drained, ActiveCalendars = %d, want 1", got)
	}
	u3()
	if got := h.ActiveSubscribers(); got != 0 {
		t.Errorf("ActiveSubscribers = %d, want 0", got)
	}
}

func TestPublish_EmptyArgsNoOp(t *testing.T) {
	h := newTestHub()
	_, unsub := h.Subscribe("cal-1")
	defer unsub()

	h.Publish("", EventAvailabilityChanged)
	h.Publish("cal-1", "")
	// Should not panic. No events should be delivered; give it a moment to
	// confirm nothing arrived.
}
