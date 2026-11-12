import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), refresh: vi.fn() }),
}));

vi.mock("@/lib/analytics", () => ({
  default: { init: vi.fn(), identify: vi.fn(), reset: vi.fn(), capture: vi.fn() },
}));

const getCalendarMock = vi.fn();
const addParticipantMock = vi.fn();
const setAvailabilityMock = vi.fn();

vi.mock("@/lib/api", async () => {
  const actual = await vi.importActual<typeof import("@/lib/api")>("@/lib/api");
  return {
    ...actual,
    getCalendar: (...args: unknown[]) => getCalendarMock(...args),
    addParticipant: (...args: unknown[]) => addParticipantMock(...args),
    setAvailability: (...args: unknown[]) => setAvailabilityMock(...args),
    getMe: vi.fn().mockResolvedValue(null),
  };
});

import AuthProvider from "@/components/AuthProvider/AuthProvider";
import { ToastProvider } from "@/lib/toast";
import CalendarPageClient from "./CalendarPageClient";

const mockCalendar = {
  calendar: { id: "cal-1", name: "Trip", ownerId: "user-1", createdAt: "2026-01-01T00:00:00Z" },
  participants: [
    { id: "p-1", calendarId: "cal-1", name: "Alice", colour: "#ef4444" },
    { id: "p-other", calendarId: "cal-1", name: "Carol", colour: "#10b981" },
  ],
  availability: [{ participantId: "p-1", calendarId: "cal-1", date: "2026-04-20" }],
};

beforeEach(() => {
  getCalendarMock.mockReset();
  addParticipantMock.mockReset();
  setAvailabilityMock.mockReset();
  localStorage.clear();
});

function renderPage() {
  render(
    <AuthProvider>
      <ToastProvider>
        <CalendarPageClient calendarId="cal-1" />
      </ToastProvider>
    </AuthProvider>,
  );
}

describe("CalendarPage", () => {
  it("renders loading state", async () => {
    getCalendarMock.mockReturnValue(new Promise(() => {})); // never resolves
    renderPage();
    expect(screen.getByText("Loading…")).toBeInTheDocument();
  });

  it("fetches and displays calendar data", async () => {
    getCalendarMock.mockResolvedValue(mockCalendar);
    renderPage();

    expect(await screen.findByText("Trip")).toBeInTheDocument();
    expect(screen.getByText(/Alice/)).toBeInTheDocument();
  });

  it("shows join form when user is not a participant", async () => {
    getCalendarMock.mockResolvedValue(mockCalendar);
    renderPage();

    expect(await screen.findByLabelText(/your name/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /join/i })).toBeInTheDocument();
  });

  it("skips join form when localStorage has a valid participant ID", async () => {
    localStorage.setItem("freewhen:cal-1", JSON.stringify({ participantId: "p-1", name: "Alice" }));
    getCalendarMock.mockResolvedValue(mockCalendar);
    renderPage();

    await waitFor(() => {
      expect(screen.getByText("Trip")).toBeInTheDocument();
    });
    expect(screen.queryByLabelText(/your name/i)).not.toBeInTheDocument();
    expect(screen.getByText(/Alice \(you\)/)).toBeInTheDocument();
  });

  it("handles calendar not found (404)", async () => {
    const { ApiError } = await import("@/lib/api");
    getCalendarMock.mockRejectedValue(new ApiError(404, "not found"));
    renderPage();

    expect(await screen.findByText("Calendar not found")).toBeInTheDocument();
  });

  it("joins and shows the grid", async () => {
    getCalendarMock.mockResolvedValue({
      ...mockCalendar,
      participants: [{ id: "p-other", calendarId: "cal-1", name: "Carol", colour: "#10b981" }],
      availability: [],
    });
    addParticipantMock.mockResolvedValue({
      id: "p-2",
      calendarId: "cal-1",
      name: "Bob",
      colour: "#3b82f6",
    });

    const user = userEvent.setup();
    renderPage();

    const nameInput = await screen.findByLabelText(/your name/i);
    await user.type(nameInput, "Bob");
    await user.click(screen.getByRole("button", { name: /join/i }));

    await waitFor(() => {
      expect(screen.queryByLabelText(/your name/i)).not.toBeInTheDocument();
    });
    expect(screen.getByText(/Bob \(you\)/)).toBeInTheDocument();
  });
});
