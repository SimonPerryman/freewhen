import { renderHook, act } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useCalendarEvents } from "./useCalendarEvents";
import type {
  RealtimeEvent,
  RealtimeStatus,
  RealtimeStream,
  StreamFactory,
} from "./realtimeStream";

vi.mock("./analytics", () => ({
  default: { capture: vi.fn() },
}));

class FakeStream implements RealtimeStream {
  private eventFn: ((e: RealtimeEvent) => void) | null = null;
  private statusFn: ((s: RealtimeStatus) => void) | null = null;
  public closed = false;

  onEvent(fn: (e: RealtimeEvent) => void) {
    this.eventFn = fn;
  }

  onStatus(fn: (s: RealtimeStatus) => void) {
    this.statusFn = fn;
    fn("connecting");
  }

  close() {
    this.closed = true;
  }

  // Test helpers — called from tests to simulate backend activity.
  emitOpen() {
    this.statusFn?.("open");
  }

  emitError() {
    this.statusFn?.("error");
  }

  emitEvent(type: string, calendarId = "cal-1") {
    this.eventFn?.({ type, calendarId });
  }
}

function makeFactory(): { factory: StreamFactory; stream: FakeStream } {
  const stream = new FakeStream();
  const factory: StreamFactory = () => stream;
  return { factory, stream };
}

describe("useCalendarEvents", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.clearAllMocks();
  });

  it("calls onRefresh once for a burst of events (150ms debounce)", () => {
    const { factory, stream } = makeFactory();
    const onRefresh = vi.fn();

    renderHook(() => useCalendarEvents("cal-1", null, onRefresh, { streamFactory: factory }));

    act(() => {
      stream.emitOpen();
    });
    // Initial refresh on open is debounced; flush the first debounce window.
    act(() => {
      vi.advanceTimersByTime(200);
    });
    expect(onRefresh).toHaveBeenCalledTimes(1);
    onRefresh.mockClear();

    // Three rapid events → one debounced call.
    act(() => {
      stream.emitEvent("availability.changed");
      stream.emitEvent("availability.changed");
      stream.emitEvent("availability.changed");
    });
    act(() => {
      vi.advanceTimersByTime(200);
    });

    expect(onRefresh).toHaveBeenCalledTimes(1);
  });

  it("reports status transitions: connecting → open", () => {
    const { factory, stream } = makeFactory();
    const onRefresh = vi.fn();

    const { result } = renderHook(() =>
      useCalendarEvents("cal-1", null, onRefresh, { streamFactory: factory }),
    );

    expect(result.current.status).toBe("connecting");

    act(() => {
      stream.emitOpen();
    });
    expect(result.current.status).toBe("open");
  });

  it("falls back to polling after 3 consecutive errors", () => {
    const { factory, stream } = makeFactory();
    const onRefresh = vi.fn();

    const { result } = renderHook(() =>
      useCalendarEvents("cal-1", null, onRefresh, { streamFactory: factory }),
    );

    act(() => {
      stream.emitError();
      stream.emitError();
      stream.emitError();
    });

    expect(result.current.status).toBe("polling");
    expect(stream.closed).toBe(true);

    // Polling interval fires every 15s.
    act(() => {
      vi.advanceTimersByTime(15000);
    });
    expect(onRefresh).toHaveBeenCalledTimes(1);

    act(() => {
      vi.advanceTimersByTime(15000);
    });
    expect(onRefresh).toHaveBeenCalledTimes(2);
  });

  it("closes the stream and clears timers on unmount", () => {
    const { factory, stream } = makeFactory();
    const onRefresh = vi.fn();

    const { unmount } = renderHook(() =>
      useCalendarEvents("cal-1", null, onRefresh, { streamFactory: factory }),
    );

    unmount();

    expect(stream.closed).toBe(true);

    // No further refresh calls even if time advances.
    act(() => {
      vi.advanceTimersByTime(60_000);
    });
    expect(onRefresh).not.toHaveBeenCalled();
  });

  it("passes participantId through as ?as= to the stream URL", () => {
    const streams: string[] = [];
    const factory: StreamFactory = (url: string) => {
      streams.push(url);
      return new FakeStream();
    };
    renderHook(() => useCalendarEvents("cal-1", "p-abc", vi.fn(), { streamFactory: factory }));
    expect(streams[0]).toContain("/calendars/cal-1/events?as=p-abc");
  });

  it("does not include ?as= when participantId is null", () => {
    const streams: string[] = [];
    const factory: StreamFactory = (url: string) => {
      streams.push(url);
      return new FakeStream();
    };
    renderHook(() => useCalendarEvents("cal-1", null, vi.fn(), { streamFactory: factory }));
    expect(streams[0]).not.toContain("?as=");
    expect(streams[0]).toContain("/calendars/cal-1/events");
  });
});
