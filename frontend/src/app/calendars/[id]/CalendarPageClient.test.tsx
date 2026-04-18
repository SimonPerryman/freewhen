import { render, screen, waitFor } from "@testing-library/react";
import { vi, describe, it, expect, beforeEach, type Mock } from "vitest";
import type { CalendarWithDetails, Participant } from "@/lib/types";

// --- Mock @/lib/api ---

const mockGetCalendar = vi.fn();
const mockAddParticipant = vi.fn();
const mockSetAvailability = vi.fn();

// ApiError must be defined inline in the factory since vi.mock is hoisted
vi.mock("@/lib/api", () => {
  class ApiError extends Error {
    status: number;
    constructor(status: number, message: string) {
      super(message);
      this.name = "ApiError";
      this.status = status;
    }
  }
  return {
    getCalendar: (...args: unknown[]) => mockGetCalendar(...args),
    addParticipant: (...args: unknown[]) => mockAddParticipant(...args),
    setAvailability: (...args: unknown[]) => mockSetAvailability(...args),
    ApiError,
  };
});

// --- Mock child components ---

let capturedOnJoin: ((name: string) => Promise<void>) | null = null;
let capturedJoinError: string | null = null;
let _capturedDefaultName: string | undefined;

vi.mock("@/components/JoinForm/JoinForm", () => ({
  default: (props: { onJoin: (name: string) => Promise<void>; error: string | null; defaultName?: string }) => {
    capturedOnJoin = props.onJoin;
    capturedJoinError = props.error;
    _capturedDefaultName = props.defaultName;
    return <div data-testid="join-form" />;
  },
}));

vi.mock("@/components/CalendarGrid/CalendarGrid", () => ({
  default: () => <div data-testid="calendar-grid" />,
}));

vi.mock("@/components/ParticipantList/ParticipantList", () => ({
  default: () => <div data-testid="participant-list" />,
}));

vi.mock("@/components/ShareButton/ShareButton", () => ({
  default: () => <div data-testid="share-button" />,
}));

vi.mock("@/components/SummaryView/SummaryView", () => ({
  default: () => <div data-testid="summary-view" />,
}));

vi.mock("@/components/BestDay/BestDay", () => ({
  default: () => <div data-testid="best-day" />,
}));

// --- Mock CSS module ---

vi.mock("./page.module.css", () => ({
  default: new Proxy({}, { get: (_target, prop) => String(prop) }),
}));

// --- localStorage mock ---

const localStorageMock = {
  getItem: vi.fn() as Mock,
  setItem: vi.fn() as Mock,
  removeItem: vi.fn() as Mock,
};

Object.defineProperty(window, "localStorage", {
  value: localStorageMock,
  writable: true,
});

// --- Import component under test after mocks ---

import CalendarPageClient from "./CalendarPageClient";

// --- Test data ---

const CALENDAR_ID = "cal-123";

function makeCalendarData(overrides: Partial<CalendarWithDetails> = {}): CalendarWithDetails {
  return {
    calendar: { id: CALENDAR_ID, name: "Team Meetup", createdAt: "2026-01-01T00:00:00Z" },
    participants: [],
    availability: [],
    ...overrides,
  };
}

function makeParticipant(overrides: Partial<Participant> = {}): Participant {
  return {
    id: "p-1",
    calendarId: CALENDAR_ID,
    name: "Alice",
    colour: "#ff0000",
    ...overrides,
  };
}

// --- Tests ---

describe("CalendarPageClient", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    capturedOnJoin = null;
    capturedJoinError = null;
    _capturedDefaultName = undefined;
    localStorageMock.getItem.mockReturnValue(null);
  });

  it("loads calendar and renders heading", async () => {
    mockGetCalendar.mockResolvedValue(makeCalendarData());

    render(<CalendarPageClient calendarId={CALENDAR_ID} />);

    await waitFor(() => {
      expect(screen.getByText("Team Meetup")).toBeInTheDocument();
    });

    expect(mockGetCalendar).toHaveBeenCalledWith(CALENDAR_ID);
  });

  it("shows join form when no stored participant", async () => {
    mockGetCalendar.mockResolvedValue(makeCalendarData());

    render(<CalendarPageClient calendarId={CALENDAR_ID} />);

    await waitFor(() => {
      expect(screen.getByTestId("join-form")).toBeInTheDocument();
    });
  });

  it("recognises stored participant from localStorage and skips join form", async () => {
    const participant = makeParticipant();
    const calendarData = makeCalendarData({
      participants: [participant],
      availability: [],
    });

    localStorageMock.getItem.mockReturnValue(
      JSON.stringify({ participantId: participant.id, name: participant.name }),
    );
    mockGetCalendar.mockResolvedValue(calendarData);

    render(<CalendarPageClient calendarId={CALENDAR_ID} />);

    await waitFor(() => {
      expect(screen.getByText("Team Meetup")).toBeInTheDocument();
    });

    expect(screen.queryByTestId("join-form")).not.toBeInTheDocument();
  });

  it("join flow: submitting name calls API, stores to localStorage, shows grid", async () => {
    const calendarData = makeCalendarData();
    mockGetCalendar.mockResolvedValue(calendarData);

    const newParticipant = makeParticipant({ id: "p-new", name: "Bob" });
    mockAddParticipant.mockResolvedValue(newParticipant);

    render(<CalendarPageClient calendarId={CALENDAR_ID} />);

    await waitFor(() => {
      expect(capturedOnJoin).not.toBeNull();
    });

    // Invoke the captured onJoin callback
    await capturedOnJoin!("Bob");

    expect(mockAddParticipant).toHaveBeenCalledWith(CALENDAR_ID, { name: "Bob" });

    expect(localStorageMock.setItem).toHaveBeenCalledWith(
      `freewhen:${CALENDAR_ID}`,
      JSON.stringify({ participantId: "p-new", name: "Bob" }),
    );

    // After joining, the join form should disappear
    await waitFor(() => {
      expect(screen.queryByTestId("join-form")).not.toBeInTheDocument();
    });

    // Grid should still be visible
    expect(screen.getByTestId("calendar-grid")).toBeInTheDocument();
  });

  it("join flow: duplicate name shows 'already taken' error", async () => {
    mockGetCalendar.mockResolvedValue(makeCalendarData());

    render(<CalendarPageClient calendarId={CALENDAR_ID} />);

    await waitFor(() => {
      expect(capturedOnJoin).not.toBeNull();
    });

    // Simulate a 409 conflict from the API — import ApiError from the mocked module
    const { ApiError } = await import("@/lib/api");
    mockAddParticipant.mockRejectedValue(new ApiError(409, "Name taken"));

    // The handleJoin callback re-throws, so we catch it here
    await expect(capturedOnJoin!("Alice")).rejects.toThrow();

    // The captured error prop is updated on re-render
    await waitFor(() => {
      expect(capturedJoinError).toBe("That name is already taken");
    });
  });
});
