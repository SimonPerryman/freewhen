import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { act, render, screen, waitFor } from "@testing-library/react";
import { renderToString } from "react-dom/server";
import { hydrateRoot, type Root } from "react-dom/client";
import type { CalendarWithDetails } from "@/lib/types";

// Server-rendered first paint: CalendarPageClient receives the calendar the
// server fetched anonymously (see plans/ui/calendar-page-server-render.md).

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), refresh: vi.fn() }),
}));

vi.mock("@/lib/analytics", () => ({
  default: { init: vi.fn(), identify: vi.fn(), reset: vi.fn(), capture: vi.fn() },
}));

const getCalendarMock = vi.fn();

vi.mock("@/lib/api", async () => {
  const actual = await vi.importActual<typeof import("@/lib/api")>("@/lib/api");
  return {
    ...actual,
    getCalendar: (...args: unknown[]) => getCalendarMock(...args),
    getMe: vi.fn().mockRejectedValue(new Error("not authenticated")),
  };
});

// No real EventSource in happy-dom; record what the page subscribes to.
const streamCalls: Array<[string, string | null]> = [];
vi.mock("@/lib/useCalendarEvents", () => ({
  useCalendarEvents: (calendarId: string, participantId: string | null) => {
    streamCalls.push([calendarId, participantId]);
    return { status: calendarId ? "open" : "connecting" };
  },
}));

import AuthProvider from "@/components/AuthProvider/AuthProvider";
import { ToastProvider } from "@/lib/toast";
import { localToday } from "@/lib/localDate";
import CalendarPageClient from "./CalendarPageClient";

const serverCalendar: CalendarWithDetails = {
  calendar: {
    id: "cal-1",
    name: "Trip",
    ownerId: "user-1",
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
  },
  participants: [
    { id: "p-1", calendarId: "cal-1", name: "Alice", colour: "#ef4444" },
    { id: "p-other", calendarId: "cal-1", name: "Carol", colour: "#10b981" },
  ],
  availability: [{ participantId: "p-1", calendarId: "cal-1", date: "2099-04-20" }],
};

function page(props: { initialData?: CalendarWithDetails | null; initialToday?: string | null }) {
  return (
    <AuthProvider>
      <ToastProvider>
        <CalendarPageClient calendarId="cal-1" {...props} />
      </ToastProvider>
    </AuthProvider>
  );
}

beforeEach(() => {
  getCalendarMock.mockReset();
  streamCalls.length = 0;
  localStorage.clear();
});

describe("CalendarPageClient with server data", () => {
  it("renders the calendar on the first render with no loading state and no client fetch", async () => {
    render(page({ initialData: serverCalendar, initialToday: localToday() }));

    // Synchronously, before any effect has resolved.
    expect(screen.queryByText("Loading…")).not.toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Trip" })).toBeInTheDocument();
    expect(screen.getByText("2 people")).toBeInTheDocument();

    // A new visitor gets the join form once identity resolves (localStorage).
    expect(await screen.findByLabelText(/your name/i)).toBeInTheDocument();
    expect(getCalendarMock).not.toHaveBeenCalled();
  });

  it("never flashes the join form for a returning participant", async () => {
    localStorage.setItem("freewhen:cal-1", JSON.stringify({ participantId: "p-1", name: "Alice" }));
    render(page({ initialData: serverCalendar, initialToday: localToday() }));

    expect(screen.queryByLabelText(/your name/i)).not.toBeInTheDocument();
    expect(await screen.findByText(/Alice \(you\)/)).toBeInTheDocument();
    expect(screen.queryByLabelText(/your name/i)).not.toBeInTheDocument();
    expect(getCalendarMock).not.toHaveBeenCalled();
  });

  it("opens the live stream once, after identity is known", async () => {
    localStorage.setItem("freewhen:cal-1", JSON.stringify({ participantId: "p-1", name: "Alice" }));
    render(page({ initialData: serverCalendar, initialToday: localToday() }));
    await screen.findByText(/Alice \(you\)/);

    const subscribed = streamCalls.filter(([id]) => id !== "");
    expect(subscribed.length).toBeGreaterThan(0);
    // Every non-empty subscription already carries our participant id.
    expect(subscribed.every(([, pid]) => pid === "p-1")).toBe(true);
  });

  it("falls back to the client fetch when the server passed no data (passphrase / error)", async () => {
    getCalendarMock.mockResolvedValue(serverCalendar);
    render(page({ initialData: null, initialToday: null }));

    expect(screen.getByText("Loading…")).toBeInTheDocument();
    expect(await screen.findByRole("heading", { name: "Trip" })).toBeInTheDocument();
    expect(getCalendarMock).toHaveBeenCalledTimes(1);
  });

  it("ignores server data for a different calendar id", async () => {
    getCalendarMock.mockResolvedValue(serverCalendar);
    const other = { ...serverCalendar, calendar: { ...serverCalendar.calendar, id: "cal-2" } };
    render(page({ initialData: other, initialToday: localToday() }));
    expect(screen.getByText("Loading…")).toBeInTheDocument();
    await screen.findByRole("heading", { name: "Trip" });
    expect(getCalendarMock).toHaveBeenCalledTimes(1);
  });
});

describe("CalendarPageClient SSR + hydration", () => {
  let container: HTMLDivElement;
  let root: Root | null = null;
  let consoleError: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    container = document.createElement("div");
    document.body.appendChild(container);
    consoleError = vi.spyOn(console, "error").mockImplementation(() => {});
  });

  afterEach(() => {
    act(() => root?.unmount());
    root = null;
    container.remove();
    consoleError.mockRestore();
  });

  async function hydrate(element: React.ReactElement) {
    const recoverable: unknown[] = [];
    await act(async () => {
      root = hydrateRoot(container, element, {
        onRecoverableError: (err) => recoverable.push(err),
      });
    });
    return recoverable;
  }

  it("server HTML shows the calendar, not the loading state", () => {
    const html = renderToString(page({ initialData: serverCalendar, initialToday: localToday() }));
    expect(html).toContain("Trip");
    expect(html).not.toContain("Loading…");
    // Identity is unknown on the server: no join form in the HTML.
    expect(html.toLowerCase()).not.toContain("your name");
  });

  it("hydrates without mismatches even when the server's date differs from the browser's", async () => {
    // Simulate a server (UTC) that is a day behind the visitor.
    const d = new Date();
    d.setDate(d.getDate() - 1);
    const serverDay = localToday(d);
    const element = page({ initialData: serverCalendar, initialToday: serverDay });

    container.innerHTML = renderToString(element);
    const recoverable = await hydrate(element);

    expect(recoverable).toEqual([]);
    const hydrationErrors = consoleError.mock.calls.filter((args: unknown[]) =>
      String(args[0]).toLowerCase().includes("hydrat"),
    );
    expect(hydrationErrors).toEqual([]);
    await waitFor(() => expect(container.textContent).toContain("Trip"));
    expect(getCalendarMock).not.toHaveBeenCalled();
  });
});
