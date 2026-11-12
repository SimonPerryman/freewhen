import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

// React's cache() only memoises inside a Server Components request; in a
// plain test runtime it is a pass-through. Stand in a per-"request" memo so
// we can assert generateMetadata and the page share one fetch.
const requestCache = new Map<string, unknown>();
vi.mock("react", async () => {
  const actual = await vi.importActual<typeof import("react")>("react");
  return {
    ...actual,
    cache:
      <A extends unknown[], R>(fn: (...args: A) => R) =>
      (...args: A): R => {
        const key = JSON.stringify(args);
        if (!requestCache.has(key)) requestCache.set(key, fn(...args));
        return requestCache.get(key) as R;
      },
  };
});

import { API_BASE_URL } from "./config";
import { getCalendarForServer, initialCalendarData } from "./serverCalendar";
import CalendarPage, { generateMetadata } from "@/app/calendars/[id]/page";

const apiCalendar = {
  calendar: {
    id: "cal-1",
    name: "Trip",
    owner_id: "o1",
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-02T00:00:00Z",
    has_passphrase: false,
  },
  participants: [{ id: "p1", calendar_id: "cal-1", name: "Alice", colour: "#f00" }],
  availability: [],
};

const gatedStub = { calendar: { id: "cal-1", has_passphrase: true }, requires_passphrase: true };

function respond(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

let fetchMock: ReturnType<typeof vi.fn>;

beforeEach(() => {
  requestCache.clear();
  fetchMock = vi.fn();
  vi.stubGlobal("fetch", fetchMock);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

const params = Promise.resolve({ id: "cal-1" });

describe("getCalendarForServer", () => {
  it("makes an anonymous, uncached request and camel-cases the body", async () => {
    fetchMock.mockResolvedValue(respond(apiCalendar));
    const data = await getCalendarForServer("cal-1");

    expect(data?.calendar.ownerId).toBe("o1");
    expect(data?.participants[0].calendarId).toBe("cal-1");

    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe(`${API_BASE_URL}/calendars/cal-1`);
    expect(init.cache).toBe("no-store");
    expect(init.credentials).toBe("omit");
    // Nothing that identifies the viewer is forwarded.
    const headers = new Headers(init.headers);
    expect([...headers.keys()].map((k) => k.toLowerCase())).toEqual(["accept"]);
  });

  it("encodes the id into the path", async () => {
    fetchMock.mockResolvedValue(respond(apiCalendar));
    await getCalendarForServer("../evil?x=1");
    expect(fetchMock.mock.calls[0][0]).toBe(`${API_BASE_URL}/calendars/..%2Fevil%3Fx%3D1`);
  });

  it.each([404, 410, 500])("returns null on HTTP %s", async (status) => {
    fetchMock.mockResolvedValue(respond({ error: "x" }, status));
    expect(await getCalendarForServer("cal-1")).toBeNull();
  });

  it("returns null when the API is unreachable", async () => {
    fetchMock.mockRejectedValue(new TypeError("fetch failed"));
    expect(await getCalendarForServer("cal-1")).toBeNull();
  });
});

describe("initialCalendarData", () => {
  it("only passes through calendars readable without credentials", () => {
    expect(initialCalendarData(null)).toBeNull();
    expect(
      initialCalendarData({
        calendar: { id: "c", hasPassphrase: true },
        requiresPassphrase: true,
      } as never),
    ).toBeNull();
    const open = { calendar: { id: "c" }, participants: [], availability: [] } as never;
    expect(initialCalendarData(open)).toBe(open);
  });
});

describe("/calendars/[id] server component", () => {
  it("metadata and page share a single API fetch per request", async () => {
    fetchMock.mockResolvedValue(respond(apiCalendar));

    const metadata = await generateMetadata({ params });
    const element = await CalendarPage({ params });

    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(metadata.title).toBe("Trip — FreeWhen");
    expect(element.props.initialData.calendar.name).toBe("Trip");
    expect(element.props.initialToday).toMatch(/^\d{4}-\d{2}-\d{2}$/);
  });

  it("passes no initial data for a passphrase-protected calendar", async () => {
    fetchMock.mockResolvedValue(respond(gatedStub));

    const metadata = await generateMetadata({ params });
    const element = await CalendarPage({ params });

    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(element.props.initialData).toBeNull();
    expect(element.props.initialToday).toBeNull();
    // Metadata reveals nothing beyond the generic fallback.
    expect(metadata.title).toBe("FreeWhen");
  });

  it("passes no initial data when the API fails", async () => {
    fetchMock.mockResolvedValue(respond({ error: "boom" }, 500));
    const element = await CalendarPage({ params });
    expect(element.props.initialData).toBeNull();
  });
});
