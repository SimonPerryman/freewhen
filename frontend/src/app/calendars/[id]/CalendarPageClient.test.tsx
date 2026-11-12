import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { vi, describe, it, expect, beforeEach, type Mock } from "vitest";
import type { CalendarWithDetails, Participant } from "@/lib/types";

// --- Mock analytics ---
vi.mock("@/lib/analytics", () => ({
  default: { init: vi.fn(), identify: vi.fn(), reset: vi.fn(), capture: vi.fn() },
}));

// --- Mock next/navigation ---
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), refresh: vi.fn() }),
}));

// --- Mock @/lib/api ---

const mockGetCalendar = vi.fn();
const mockAddParticipant = vi.fn();
const mockSetAvailability = vi.fn();
const mockRenameCalendar = vi.fn();
const mockRemoveParticipant = vi.fn();

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
  class CalendarUnavailableError extends ApiError {
    constructor(message: string) {
      super(410, message);
      this.name = "CalendarUnavailableError";
    }
  }
  return {
    getCalendar: (...args: unknown[]) => mockGetCalendar(...args),
    addParticipant: (...args: unknown[]) => mockAddParticipant(...args),
    setAvailability: (...args: unknown[]) => mockSetAvailability(...args),
    renameCalendar: (...args: unknown[]) => mockRenameCalendar(...args),
    removeParticipant: (...args: unknown[]) => mockRemoveParticipant(...args),
    finaliseCalendar: vi.fn(),
    unfinaliseCalendar: vi.fn(),
    claimParticipant: vi.fn(),
    getMe: vi.fn().mockResolvedValue(null),
    logout: vi.fn().mockResolvedValue(undefined),
    fetchImportSuggestions: vi.fn().mockResolvedValue(null),
    ackCalendarImportConsent: vi.fn().mockResolvedValue(undefined),
    importAvailabilityDays: vi.fn().mockResolvedValue(undefined),
    googleCalendarConnectUrl: (id: string) => `/auth/google/calendar/connect?calendar_id=${id}`,
    ApiError,
    CalendarUnavailableError,
  };
});

// --- Mock child components ---

let capturedOnJoin: ((name: string) => Promise<void>) | null = null;
let capturedJoinError: string | null = null;
let _capturedDefaultName: string | undefined;

vi.mock("@/components/JoinForm/JoinForm", () => ({
  default: (props: {
    onJoin: (name: string) => Promise<void>;
    error: string | null;
    defaultName?: string;
  }) => {
    capturedOnJoin = props.onJoin;
    capturedJoinError = props.error;
    _capturedDefaultName = props.defaultName;
    return <div data-testid="join-form" />;
  },
}));

interface MockedCalendarGridProps {
  onToggle: (date: string) => void;
  onOpenDay?: (date: string) => void;
  onRangeToggle?: (dates: string[], target: boolean) => void;
  onClear?: () => void;
  canClear?: boolean;
  selectedDates?: Set<string>;
  noteAuthorsByDate?: Map<string, Set<string>>;
}

let capturedGridProps: MockedCalendarGridProps | null = null;

vi.mock("@/components/CalendarGrid/CalendarGrid", () => ({
  default: (props: MockedCalendarGridProps) => {
    capturedGridProps = props;
    return <div data-testid="calendar-grid" />;
  },
}));

interface MockedDaySheetProps {
  date: string;
  currentNote: string;
  isSelected: boolean;
  finalised: boolean;
  onSaveNote: (text: string) => void;
  onRemoveAvailability: () => void;
  onClose: () => void;
}

let capturedSheetProps: MockedDaySheetProps | null = null;

vi.mock("@/components/DayDetailSheet/DayDetailSheet", () => ({
  default: (props: MockedDaySheetProps) => {
    capturedSheetProps = props;
    return <div data-testid="day-detail-sheet" />;
  },
}));

interface MockedParticipantListProps {
  canRemove?: boolean;
  onRemove?: (p: { id: string; name: string }) => void;
  participants: Array<{ id: string; name: string }>;
}

let capturedParticipantListProps: MockedParticipantListProps | null = null;

vi.mock("@/components/ParticipantList/ParticipantList", () => ({
  default: (props: MockedParticipantListProps) => {
    capturedParticipantListProps = props;
    return <div data-testid="participant-list" />;
  },
}));

vi.mock("@/components/InvitePrompt/InvitePrompt", () => ({
  default: () => <div data-testid="invite-prompt" />,
}));

let mockUser: { id: string; email: string; name: string } | null = null;

let capturedShowToast: ReturnType<typeof vi.fn> | null = null;
let capturedDismiss: ReturnType<typeof vi.fn> | null = null;

vi.mock("@/lib/toast", () => ({
  ToastProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  useToast: () => {
    if (!capturedShowToast) capturedShowToast = vi.fn();
    if (!capturedDismiss) capturedDismiss = vi.fn();
    return { showToast: capturedShowToast, dismiss: capturedDismiss };
  },
}));

vi.mock("@/components/AuthProvider/AuthProvider", () => ({
  default: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  useAuth: () => ({
    user: mockUser,
    isLoading: false,
    isAuthenticated: !!mockUser,
    checkAuth: vi.fn(async () => mockUser),
    clearAuth: vi.fn(),
    signOut: vi.fn(),
  }),
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

import { StrictMode } from "react";
import type React from "react";
import AuthProvider from "@/components/AuthProvider/AuthProvider";
import CalendarPageClient from "./CalendarPageClient";

// --- Test data ---

const CALENDAR_ID = "cal-123";

function makeCalendarData(overrides: Partial<CalendarWithDetails> = {}): CalendarWithDetails {
  return {
    calendar: {
      id: CALENDAR_ID,
      name: "Team Meetup",
      emoji: "✨",
      ownerId: "owner-1",
      createdAt: "2026-01-01T00:00:00Z",
      updatedAt: "2026-01-01T00:00:00Z",
    },
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
    capturedParticipantListProps = null;
    capturedGridProps = null;
    capturedSheetProps = null;
    mockUser = null;
    capturedShowToast = null;
    capturedDismiss = null;
    localStorageMock.getItem.mockReturnValue(null);
  });

  it("loads calendar and renders heading", async () => {
    mockGetCalendar.mockResolvedValue(makeCalendarData());

    render(
      <AuthProvider>
        <CalendarPageClient calendarId={CALENDAR_ID} />
      </AuthProvider>,
    );

    await waitFor(() => {
      expect(screen.getByText("Team Meetup")).toBeInTheDocument();
    });

    expect(mockGetCalendar).toHaveBeenCalledWith(CALENDAR_ID);
  });

  it("omits the emoji element when the calendar has no emoji set", async () => {
    mockGetCalendar.mockResolvedValue(
      makeCalendarData({
        calendar: {
          id: CALENDAR_ID,
          name: "Team Meetup",
          ownerId: "owner-1",
          createdAt: "2026-01-01T00:00:00Z",
          updatedAt: "2026-01-01T00:00:00Z",
        },
      }),
    );

    render(
      <AuthProvider>
        <CalendarPageClient calendarId={CALENDAR_ID} />
      </AuthProvider>,
    );

    await waitFor(() => {
      expect(screen.getByText("Team Meetup")).toBeInTheDocument();
    });

    // Heading present but no emoji span — query for any of the curated set.
    expect(screen.queryByText("✨")).not.toBeInTheDocument();
    expect(screen.queryByText("🦊")).not.toBeInTheDocument();
  });

  it("shows join form when no stored participant", async () => {
    mockGetCalendar.mockResolvedValue(makeCalendarData());

    render(
      <AuthProvider>
        <CalendarPageClient calendarId={CALENDAR_ID} />
      </AuthProvider>,
    );

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

    render(
      <AuthProvider>
        <CalendarPageClient calendarId={CALENDAR_ID} />
      </AuthProvider>,
    );

    await waitFor(() => {
      expect(screen.getByText("Team Meetup")).toBeInTheDocument();
    });

    expect(screen.queryByTestId("join-form")).not.toBeInTheDocument();
  });

  it("prefills join form from global freewhen:lastName when no stored participant and not signed in", async () => {
    mockGetCalendar.mockResolvedValue(makeCalendarData());
    localStorageMock.getItem.mockImplementation((key: string) =>
      key === "freewhen:lastName" ? "Sam" : null,
    );

    render(
      <AuthProvider>
        <CalendarPageClient calendarId={CALENDAR_ID} />
      </AuthProvider>,
    );

    await waitFor(() => {
      expect(screen.getByTestId("join-form")).toBeInTheDocument();
    });
    await waitFor(() => {
      expect(_capturedDefaultName).toBe("Sam");
    });
  });

  it("signed-in user name wins over freewhen:lastName fallback", async () => {
    mockUser = { id: "user-1", email: "alice@example.com", name: "Alice" };
    mockGetCalendar.mockResolvedValue(makeCalendarData());
    localStorageMock.getItem.mockImplementation((key: string) =>
      key === "freewhen:lastName" ? "Sam" : null,
    );

    render(
      <AuthProvider>
        <CalendarPageClient calendarId={CALENDAR_ID} />
      </AuthProvider>,
    );

    await waitFor(() => {
      expect(screen.getByTestId("join-form")).toBeInTheDocument();
    });
    await waitFor(() => {
      expect(_capturedDefaultName).toBe("Alice");
    });
  });

  it("successful join writes freewhen:lastName to localStorage", async () => {
    mockGetCalendar.mockResolvedValue(makeCalendarData());
    mockAddParticipant.mockResolvedValue(makeParticipant({ id: "p-new", name: "Sammy" }));

    render(
      <AuthProvider>
        <CalendarPageClient calendarId={CALENDAR_ID} />
      </AuthProvider>,
    );

    await waitFor(() => {
      expect(capturedOnJoin).not.toBeNull();
    });

    await capturedOnJoin!("Sammy");

    expect(localStorageMock.setItem).toHaveBeenCalledWith("freewhen:lastName", "Sammy");
  });

  it("join flow: submitting name calls API, stores to localStorage, shows grid", async () => {
    const calendarData = makeCalendarData();
    mockGetCalendar.mockResolvedValue(calendarData);

    const newParticipant = makeParticipant({ id: "p-new", name: "Bob" });
    mockAddParticipant.mockResolvedValue(newParticipant);

    render(
      <AuthProvider>
        <CalendarPageClient calendarId={CALENDAR_ID} />
      </AuthProvider>,
    );

    await waitFor(() => {
      expect(capturedOnJoin).not.toBeNull();
    });

    // Invoke the captured onJoin callback
    await capturedOnJoin!("Bob");

    expect(mockAddParticipant).toHaveBeenCalledWith(CALENDAR_ID, { name: "Bob" });

    expect(localStorageMock.setItem).toHaveBeenCalledWith(
      `freewhen:${CALENDAR_ID}`,
      JSON.stringify({ participantId: "p-new", name: "Bob", colour: "#ff0000" }),
    );

    // After joining, the join form should disappear
    await waitFor(() => {
      expect(screen.queryByTestId("join-form")).not.toBeInTheDocument();
    });

    // Grid should still be visible
    expect(screen.getByTestId("calendar-grid")).toBeInTheDocument();
  });

  it("renders a friendly not-found panel with a home link on 404", async () => {
    const { ApiError } = await import("@/lib/api");
    mockGetCalendar.mockRejectedValue(new ApiError(404, "not found"));

    render(
      <AuthProvider>
        <CalendarPageClient calendarId={CALENDAR_ID} />
      </AuthProvider>,
    );

    await waitFor(() => {
      expect(screen.getByRole("heading", { name: "Calendar not found" })).toBeInTheDocument();
    });

    const cta = screen.getByRole("link", { name: "Back to home" });
    expect(cta).toHaveAttribute("href", "/");
  });

  it("renders a retry panel on generic load failure and retries on click", async () => {
    mockGetCalendar.mockRejectedValueOnce(new Error("network down"));

    render(
      <AuthProvider>
        <CalendarPageClient calendarId={CALENDAR_ID} />
      </AuthProvider>,
    );

    await waitFor(() => {
      expect(
        screen.getByRole("heading", { name: "Couldn't load this calendar" }),
      ).toBeInTheDocument();
    });

    // Second attempt succeeds
    mockGetCalendar.mockResolvedValueOnce(makeCalendarData());

    const retry = screen.getByRole("button", { name: "Try again" });
    retry.click();

    await waitFor(() => {
      expect(screen.getByText("Team Meetup")).toBeInTheDocument();
    });

    expect(mockGetCalendar).toHaveBeenCalledTimes(2);
  });

  it("does not show the rename pencil when the user is not the owner", async () => {
    mockGetCalendar.mockResolvedValue(makeCalendarData());

    render(
      <AuthProvider>
        <CalendarPageClient calendarId={CALENDAR_ID} />
      </AuthProvider>,
    );

    await waitFor(() => {
      expect(screen.getByText("Team Meetup")).toBeInTheDocument();
    });

    expect(screen.queryByRole("button", { name: /Rename calendar/i })).not.toBeInTheDocument();
  });

  it("owner can rename the calendar inline", async () => {
    mockUser = { id: "owner-1", email: "owner@example.com", name: "Owner" };
    mockGetCalendar.mockResolvedValue(makeCalendarData());
    mockRenameCalendar.mockResolvedValue({
      id: CALENDAR_ID,
      name: "Renamed Trip",
      ownerId: "owner-1",
      createdAt: "2026-01-01T00:00:00Z",
    });

    render(
      <AuthProvider>
        <CalendarPageClient calendarId={CALENDAR_ID} />
      </AuthProvider>,
    );

    await waitFor(() => {
      expect(screen.getByText("Team Meetup")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: "Calendar settings" }));
    // The settings sheet is lazy-loaded (next/dynamic), so it appears async.
    fireEvent.click(await screen.findByRole("button", { name: /Rename calendar/i }));

    const input = (await screen.findByRole("textbox", {
      name: "Calendar name",
    })) as HTMLInputElement;
    fireEvent.change(input, { target: { value: "Renamed Trip" } });
    fireEvent.keyDown(input, { key: "Enter" });

    await waitFor(() => {
      expect(mockRenameCalendar).toHaveBeenCalledWith(CALENDAR_ID, "Renamed Trip");
    });

    await waitFor(() => {
      expect(screen.getByText("Renamed Trip")).toBeInTheDocument();
    });
  });

  it("owner forwards canRemove and onRemove to ParticipantList", async () => {
    mockUser = { id: "owner-1", email: "owner@example.com", name: "Owner" };
    mockGetCalendar.mockResolvedValue(
      makeCalendarData({
        participants: [
          makeParticipant({ id: "p-1", name: "Alice" }),
          makeParticipant({ id: "p-2", name: "Bob" }),
        ],
      }),
    );

    render(
      <AuthProvider>
        <CalendarPageClient calendarId={CALENDAR_ID} />
      </AuthProvider>,
    );

    await waitFor(() => {
      expect(capturedParticipantListProps?.canRemove).toBe(true);
    });

    expect(typeof capturedParticipantListProps?.onRemove).toBe("function");
  });

  it("non-owner does not get canRemove on ParticipantList", async () => {
    mockGetCalendar.mockResolvedValue(
      makeCalendarData({
        participants: [
          makeParticipant({ id: "p-1", name: "Alice" }),
          makeParticipant({ id: "p-2", name: "Bob" }),
        ],
      }),
    );

    render(
      <AuthProvider>
        <CalendarPageClient calendarId={CALENDAR_ID} />
      </AuthProvider>,
    );

    await waitFor(() => {
      expect(capturedParticipantListProps).not.toBeNull();
    });

    expect(capturedParticipantListProps?.canRemove).toBe(false);
  });

  it("range toggle: updates selectedDates and calls setAvailability once with merged set", async () => {
    const participant = makeParticipant();
    const calendarData = makeCalendarData({
      participants: [participant],
      availability: [],
    });

    localStorageMock.getItem.mockReturnValue(
      JSON.stringify({ participantId: participant.id, name: participant.name }),
    );
    mockGetCalendar.mockResolvedValue(calendarData);
    mockSetAvailability.mockResolvedValue(undefined);

    render(
      <AuthProvider>
        <CalendarPageClient calendarId={CALENDAR_ID} />
      </AuthProvider>,
    );

    await waitFor(() => {
      expect(capturedGridProps?.onRangeToggle).toBeDefined();
    });

    // Select three consecutive dates via range toggle
    const dates = ["2030-01-01", "2030-01-02", "2030-01-03"];
    capturedGridProps!.onRangeToggle!(dates, true);

    await waitFor(() => {
      expect(mockSetAvailability).toHaveBeenCalledTimes(1);
    });
    expect(mockSetAvailability).toHaveBeenCalledWith(CALENDAR_ID, {
      participantId: participant.id,
      dates: expect.arrayContaining(dates),
    });

    // Deselect one of them; selectedDates should now omit it
    mockSetAvailability.mockClear();
    capturedGridProps!.onRangeToggle!(["2030-01-02"], false);

    await waitFor(() => {
      expect(mockSetAvailability).toHaveBeenCalledTimes(1);
    });
    const call = mockSetAvailability.mock.calls[0];
    expect(call[1].dates).toEqual(expect.arrayContaining(["2030-01-01", "2030-01-03"]));
    expect(call[1].dates).not.toContain("2030-01-02");
  });

  it("under StrictMode: single toggle fires exactly one setAvailability call", async () => {
    const participant = makeParticipant();
    const calendarData = makeCalendarData({
      participants: [participant],
      availability: [],
    });

    localStorageMock.getItem.mockReturnValue(
      JSON.stringify({ participantId: participant.id, name: participant.name }),
    );
    mockGetCalendar.mockResolvedValue(calendarData);

    // Hold the save pending so the second StrictMode-invoked updater would have
    // time to re-queue if the bug regressed.
    let resolveSave: (() => void) | null = null;
    mockSetAvailability.mockImplementation(
      () =>
        new Promise<void>((resolve) => {
          resolveSave = () => resolve();
        }),
    );

    render(
      <StrictMode>
        <AuthProvider>
          <CalendarPageClient calendarId={CALENDAR_ID} />
        </AuthProvider>
      </StrictMode>,
    );

    await waitFor(() => {
      expect(capturedGridProps?.onToggle).toBeDefined();
    });

    capturedGridProps!.onToggle("2030-01-01");

    await waitFor(() => {
      expect(mockSetAvailability).toHaveBeenCalledTimes(1);
    });

    resolveSave!();
    // Wait a tick to let any (bug-induced) follow-up PUT fire.
    await new Promise((r) => setTimeout(r, 20));
    expect(mockSetAvailability).toHaveBeenCalledTimes(1);
  });

  it("under StrictMode: range toggle fires exactly one setAvailability call", async () => {
    const participant = makeParticipant();
    const calendarData = makeCalendarData({
      participants: [participant],
      availability: [],
    });

    localStorageMock.getItem.mockReturnValue(
      JSON.stringify({ participantId: participant.id, name: participant.name }),
    );
    mockGetCalendar.mockResolvedValue(calendarData);

    let resolveSave: (() => void) | null = null;
    mockSetAvailability.mockImplementation(
      () =>
        new Promise<void>((resolve) => {
          resolveSave = () => resolve();
        }),
    );

    render(
      <StrictMode>
        <AuthProvider>
          <CalendarPageClient calendarId={CALENDAR_ID} />
        </AuthProvider>
      </StrictMode>,
    );

    await waitFor(() => {
      expect(capturedGridProps?.onRangeToggle).toBeDefined();
    });

    capturedGridProps!.onRangeToggle!(["2030-01-04", "2030-01-05", "2030-01-06"], true);

    await waitFor(() => {
      expect(mockSetAvailability).toHaveBeenCalledTimes(1);
    });

    resolveSave!();
    await new Promise((r) => setTimeout(r, 20));
    expect(mockSetAvailability).toHaveBeenCalledTimes(1);
  });

  it("join flow: duplicate name shows 'already taken' error", async () => {
    mockGetCalendar.mockResolvedValue(makeCalendarData());

    render(
      <AuthProvider>
        <CalendarPageClient calendarId={CALENDAR_ID} />
      </AuthProvider>,
    );

    await waitFor(() => {
      expect(capturedOnJoin).not.toBeNull();
    });

    // Simulate a 409 conflict from the API — import ApiError from the mocked module
    const { ApiError } = await import("@/lib/api");
    mockAddParticipant.mockRejectedValue(new ApiError(409, "Name taken"));

    // handleJoin swallows the error after surfacing it via the joinError prop
    await expect(capturedOnJoin!("Alice")).resolves.toBeUndefined();

    // The captured error prop is updated on re-render
    await waitFor(() => {
      expect(capturedJoinError).toBe("That name is already taken");
    });
  });

  it("toggling dates without editing notes: PUT omits the notes field entirely", async () => {
    const participant = makeParticipant();
    const calendarData = makeCalendarData({
      participants: [participant],
      availability: [],
    });

    localStorageMock.getItem.mockReturnValue(
      JSON.stringify({ participantId: participant.id, name: participant.name }),
    );
    mockGetCalendar.mockResolvedValue(calendarData);
    mockSetAvailability.mockResolvedValue(undefined);

    render(
      <AuthProvider>
        <CalendarPageClient calendarId={CALENDAR_ID} />
      </AuthProvider>,
    );

    await waitFor(() => {
      expect(capturedGridProps?.onToggle).toBeDefined();
    });

    capturedGridProps!.onToggle("2030-05-01");

    await waitFor(() => {
      expect(mockSetAvailability).toHaveBeenCalledTimes(1);
    });
    const [, body] = mockSetAvailability.mock.calls[0];
    expect(body).toEqual({
      participantId: participant.id,
      dates: ["2030-05-01"],
    });
    // Critical: absent `notes` key triggers the server's "don't touch" path.
    expect("notes" in body).toBe(false);
  });

  it("editing a note fires a setAvailability call with only the edited key", async () => {
    const participant = makeParticipant();
    const calendarData = makeCalendarData({
      participants: [participant],
      availability: [
        {
          participantId: participant.id,
          calendarId: CALENDAR_ID,
          date: "2030-06-01",
        },
        {
          participantId: participant.id,
          calendarId: CALENDAR_ID,
          date: "2030-06-02",
        },
      ],
    });

    localStorageMock.getItem.mockReturnValue(
      JSON.stringify({ participantId: participant.id, name: participant.name }),
    );
    mockGetCalendar.mockResolvedValue(calendarData);
    mockSetAvailability.mockResolvedValue(undefined);

    render(
      <AuthProvider>
        <CalendarPageClient calendarId={CALENDAR_ID} />
      </AuthProvider>,
    );

    await waitFor(() => {
      expect(capturedGridProps?.onOpenDay).toBeDefined();
    });

    // Open the sheet and edit a note on 2030-06-01.
    capturedGridProps!.onOpenDay!("2030-06-01");

    await waitFor(() => {
      expect(capturedSheetProps?.onSaveNote).toBeDefined();
    });

    capturedSheetProps!.onSaveNote("only after 6pm");

    await waitFor(() => {
      expect(mockSetAvailability).toHaveBeenCalled();
    });
    const [, body] = mockSetAvailability.mock.calls[0];
    expect(body.participantId).toBe(participant.id);
    expect(body.dates).toEqual(expect.arrayContaining(["2030-06-01", "2030-06-02"]));
    // The notes map contains ONLY the edited key; the backend's partial-override
    // rule preserves the untouched 2030-06-02.
    expect(body.notes).toEqual({ "2030-06-01": "only after 6pm" });
  });

  it("clear availability: removes future dates + notes, fires one PUT with remaining set, shows toast", async () => {
    const participant = makeParticipant();
    const today = new Date();
    const future = (daysAhead: number) => {
      const d = new Date(today);
      d.setDate(d.getDate() + daysAhead);
      const y = d.getFullYear();
      const m = String(d.getMonth() + 1).padStart(2, "0");
      const day = String(d.getDate()).padStart(2, "0");
      return `${y}-${m}-${day}`;
    };
    const past = (daysBack: number) => {
      const d = new Date(today);
      d.setDate(d.getDate() - daysBack);
      const y = d.getFullYear();
      const m = String(d.getMonth() + 1).padStart(2, "0");
      const day = String(d.getDate()).padStart(2, "0");
      return `${y}-${m}-${day}`;
    };

    const f1 = future(2);
    const f2 = future(5);
    const p1 = past(2);

    const calendarData = makeCalendarData({
      participants: [participant],
      availability: [
        { participantId: participant.id, calendarId: CALENDAR_ID, date: f1, note: "noted" },
        { participantId: participant.id, calendarId: CALENDAR_ID, date: f2 },
        { participantId: participant.id, calendarId: CALENDAR_ID, date: p1 },
      ],
    });

    localStorageMock.getItem.mockReturnValue(
      JSON.stringify({ participantId: participant.id, name: participant.name }),
    );
    mockGetCalendar.mockResolvedValue(calendarData);
    mockSetAvailability.mockResolvedValue(undefined);

    render(
      <AuthProvider>
        <CalendarPageClient calendarId={CALENDAR_ID} />
      </AuthProvider>,
    );

    await waitFor(() => {
      expect(capturedGridProps?.onClear).toBeDefined();
    });
    expect(capturedGridProps?.canClear).toBe(true);

    capturedGridProps!.onClear!();

    await waitFor(() => {
      expect(mockSetAvailability).toHaveBeenCalledTimes(1);
    });
    const [, body] = mockSetAvailability.mock.calls[0];
    expect(body.dates).not.toContain(f1);
    expect(body.dates).not.toContain(f2);

    expect(capturedShowToast).toHaveBeenCalledTimes(1);
    const toastArg = capturedShowToast!.mock.calls[0][0] as {
      message: string;
      action?: { label: string; onClick: () => void };
    };
    expect(toastArg.message).toMatch(/Cleared 2 dates/);
    expect(toastArg.action?.label).toBe("Undo");

    // Undo: should restore both the selection and notes in a second PUT.
    mockSetAvailability.mockClear();
    toastArg.action!.onClick();

    await waitFor(() => {
      expect(mockSetAvailability).toHaveBeenCalledTimes(1);
    });
    const [, restored] = mockSetAvailability.mock.calls[0];
    expect(restored.dates).toEqual(expect.arrayContaining([f1, f2]));
  });

  it("clear availability: canClear is false when no future dates selected", async () => {
    const participant = makeParticipant();
    const calendarData = makeCalendarData({
      participants: [participant],
      availability: [],
    });

    localStorageMock.getItem.mockReturnValue(
      JSON.stringify({ participantId: participant.id, name: participant.name }),
    );
    mockGetCalendar.mockResolvedValue(calendarData);

    render(
      <AuthProvider>
        <CalendarPageClient calendarId={CALENDAR_ID} />
      </AuthProvider>,
    );

    await waitFor(() => {
      expect(capturedGridProps?.onClear).toBeDefined();
    });
    expect(capturedGridProps?.canClear).toBe(false);
  });

  it("deselecting a date that had a note: PUT omits notes (row delete drops the note)", async () => {
    const participant = makeParticipant();
    const calendarData = makeCalendarData({
      participants: [participant],
      availability: [
        {
          participantId: participant.id,
          calendarId: CALENDAR_ID,
          date: "2030-07-01",
          note: "existing note",
        },
      ],
    });

    localStorageMock.getItem.mockReturnValue(
      JSON.stringify({ participantId: participant.id, name: participant.name }),
    );
    mockGetCalendar.mockResolvedValue(calendarData);
    mockSetAvailability.mockResolvedValue(undefined);

    render(
      <AuthProvider>
        <CalendarPageClient calendarId={CALENDAR_ID} />
      </AuthProvider>,
    );

    await waitFor(() => {
      expect(capturedGridProps?.onToggle).toBeDefined();
    });

    // Simulate a "deselect" — the page treats a toggle on a currently-selected
    // date as a removal. Here we call onToggle with the noted date, which
    // should remove the selection and fire one PUT with no `notes` field.
    capturedGridProps!.onToggle("2030-07-01");

    await waitFor(() => {
      expect(mockSetAvailability).toHaveBeenCalled();
    });
    const [, body] = mockSetAvailability.mock.calls[0];
    expect(body.dates).not.toContain("2030-07-01");
    expect("notes" in body).toBe(false);
  });
});
