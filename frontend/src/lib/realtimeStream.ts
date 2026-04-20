// Transport-agnostic realtime stream abstraction. The hook (useCalendarEvents)
// depends only on this interface, not EventSource directly, so swapping to
// WebSocket later is a one-file change (see plans/live-updates.md).

export type RealtimeEvent = {
  type: string;
  calendarId: string;
};

export type RealtimeStatus = "connecting" | "open" | "error" | "closed";

export interface RealtimeStream {
  onEvent(fn: (e: RealtimeEvent) => void): void;
  onStatus(fn: (s: RealtimeStatus) => void): void;
  close(): void;
}

export interface StreamFactory {
  (url: string): RealtimeStream;
}

// Default factory — uses the browser's native EventSource. Production code
// should import `createSseStream` from this module; tests can pass a mock
// factory to `useCalendarEvents`.
export function createSseStream(url: string): RealtimeStream {
  let eventListener: ((e: RealtimeEvent) => void) | null = null;
  let statusListener: ((s: RealtimeStatus) => void) | null = null;

  const es = new EventSource(url, { withCredentials: true });

  const emitStatus = (s: RealtimeStatus) => statusListener?.(s);
  emitStatus("connecting");

  es.addEventListener("open", () => emitStatus("open"));

  es.addEventListener("error", () => {
    // EventSource reconnects on its own. Flip status so the hook can count
    // errors and decide when to fall back to polling.
    emitStatus("error");
  });

  // The backend emits one named event per event type (e.g. "availability.changed").
  // Attach to the handful of known types plus "message" so unknown event
  // names still propagate.
  const eventTypes = [
    "message",
    "availability.changed",
    "participant.joined",
    "participant.removed",
    "participant.claimed",
    "calendar.updated",
    "calendar.finalised",
    "calendar.unfinalised",
  ];
  for (const type of eventTypes) {
    es.addEventListener(type, (e: MessageEvent) => {
      if (!eventListener) return;
      let calendarId = "";
      try {
        const data = JSON.parse(e.data) as { calendar_id?: string };
        calendarId = data.calendar_id ?? "";
      } catch {
        // Malformed payload — still surface an event with empty id.
      }
      eventListener({ type: e.type, calendarId });
    });
  }

  return {
    onEvent(fn) {
      eventListener = fn;
    },
    onStatus(fn) {
      statusListener = fn;
    },
    close() {
      es.close();
      emitStatus("closed");
    },
  };
}
