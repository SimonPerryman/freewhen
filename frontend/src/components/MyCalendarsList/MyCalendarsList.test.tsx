import { describe, expect, it, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import MyCalendarsList from "./MyCalendarsList";
import { ToastProvider } from "@/lib/toast";
import type { MyCalendarListItem } from "@/lib/types";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), refresh: vi.fn() }),
}));

const deleteCalendarMock = vi.fn<(id: string) => Promise<void>>(async () => undefined);
const leaveCalendarMock = vi.fn<(id: string) => Promise<void>>(async () => undefined);
vi.mock("@/lib/api", () => ({
  deleteCalendar: (id: string) => deleteCalendarMock(id),
  leaveCalendar: (id: string) => leaveCalendarMock(id),
}));

function renderWithToast(ui: React.ReactElement) {
  return render(<ToastProvider>{ui}</ToastProvider>);
}

function item(overrides: Partial<MyCalendarListItem> = {}): MyCalendarListItem {
  return {
    id: "cal1",
    name: "Trip",
    emoji: "🎉",
    createdAt: "2026-04-01T10:00:00Z",
    updatedAt: new Date(Date.now() - 60 * 60 * 1000).toISOString(),
    finalisedDate: null,
    finalisedAt: null,
    participantCount: 3,
    myRole: "joiner",
    ...overrides,
  };
}

describe("MyCalendarsList", () => {
  it("renders skeletons while loading", () => {
    renderWithToast(<MyCalendarsList status="loading" calendars={[]} hasMore={false} />);
    expect(screen.getByLabelText("Loading calendars")).toBeTruthy();
  });

  it("renders the empty state with a CTA", () => {
    renderWithToast(<MyCalendarsList status="ready" calendars={[]} hasMore={false} />);
    expect(screen.getByText(/haven't created or joined/i)).toBeTruthy();
    const cta = screen.getByRole("link", { name: /create a calendar/i });
    expect(cta.getAttribute("href")).toBe("/");
  });

  it("renders an Owner pill only for owned calendars", () => {
    renderWithToast(
      <MyCalendarsList
        status="ready"
        calendars={[item({ id: "a", myRole: "owner" }), item({ id: "b", myRole: "joiner" })]}
        hasMore={false}
      />,
    );
    const owners = screen.getAllByText("Owner");
    expect(owners.length).toBe(1);
  });

  it("renders a finalised badge when finalisedDate is set", () => {
    renderWithToast(
      <MyCalendarsList
        status="ready"
        calendars={[item({ finalisedDate: "2026-05-02" })]}
        hasMore={false}
      />,
    );
    expect(screen.getByText(/May/)).toBeTruthy();
  });

  it("links each card to the calendar page", () => {
    renderWithToast(
      <MyCalendarsList status="ready" calendars={[item({ id: "abc123" })]} hasMore={false} />,
    );
    const link = screen.getByRole("link", { name: /Trip/ });
    expect(link.getAttribute("href")).toBe("/calendars/abc123");
  });

  it("renders the cap footer when hasMore is true", () => {
    renderWithToast(<MyCalendarsList status="ready" calendars={[item()]} hasMore={true} />);
    expect(screen.getByText(/100 most recent calendars/i)).toBeTruthy();
  });

  it("omits the cap footer when hasMore is false", () => {
    renderWithToast(<MyCalendarsList status="ready" calendars={[item()]} hasMore={false} />);
    expect(screen.queryByText(/100 most recent calendars/i)).toBeNull();
  });

  it("shows an error message with a retry button", () => {
    const onRetry = vi.fn();
    renderWithToast(
      <MyCalendarsList status="error" calendars={[]} hasMore={false} onRetry={onRetry} />,
    );
    const button = screen.getByRole("button", { name: /try again/i });
    fireEvent.click(button);
    expect(onRetry).toHaveBeenCalledOnce();
  });

  it("uses a placeholder emoji when none is set", () => {
    renderWithToast(
      <MyCalendarsList status="ready" calendars={[item({ emoji: "" })]} hasMore={false} />,
    );
    expect(screen.getByText("📅")).toBeTruthy();
  });

  it("renders a Leave option for non-owners", () => {
    renderWithToast(
      <MyCalendarsList status="ready" calendars={[item({ myRole: "joiner" })]} hasMore={false} />,
    );
    fireEvent.click(screen.getByRole("button", { name: /actions for trip/i }));
    expect(screen.getByRole("menuitem", { name: "Leave" })).toBeTruthy();
  });

  it("opens a menu with Duplicate, Rename, and Delete for owners", () => {
    renderWithToast(
      <MyCalendarsList status="ready" calendars={[item({ myRole: "owner" })]} hasMore={false} />,
    );
    const trigger = screen.getByRole("button", { name: /actions for trip/i });
    fireEvent.click(trigger);
    expect(screen.getByRole("menuitem", { name: "Duplicate" })).toBeTruthy();
    expect(screen.getByRole("menuitem", { name: "Rename" })).toBeTruthy();
    expect(screen.getByRole("menuitem", { name: "Delete" })).toBeTruthy();
  });

  it("opens the duplicate sheet when Duplicate is selected", () => {
    renderWithToast(
      <MyCalendarsList status="ready" calendars={[item({ myRole: "owner" })]} hasMore={false} />,
    );
    fireEvent.click(screen.getByRole("button", { name: /actions for trip/i }));
    fireEvent.click(screen.getByRole("menuitem", { name: "Duplicate" }));
    expect(screen.getByRole("dialog", { name: /duplicate this calendar/i })).toBeTruthy();
  });

  it("Delete optimistically removes the row and fires after the undo window", async () => {
    vi.useFakeTimers();
    deleteCalendarMock.mockClear();
    try {
      renderWithToast(
        <MyCalendarsList
          status="ready"
          calendars={[item({ id: "del1", myRole: "owner", name: "ToDel" })]}
          hasMore={false}
        />,
      );
      fireEvent.click(screen.getByRole("button", { name: /actions for todel/i }));
      fireEvent.click(screen.getByRole("menuitem", { name: "Delete" }));
      // Row gone immediately (empty state shows because it was the only one).
      expect(screen.queryByText("ToDel")).toBeNull();
      // API not called yet.
      expect(deleteCalendarMock).not.toHaveBeenCalled();
      // Advance past the undo window. The destructive timer fires
      // synchronously under fake timers; the API call kicks off immediately.
      vi.advanceTimersByTime(7000);
      expect(deleteCalendarMock).toHaveBeenCalledWith("del1");
    } finally {
      vi.useRealTimers();
    }
  });

  it("Undo cancels a Delete before the API fires", () => {
    vi.useFakeTimers();
    deleteCalendarMock.mockClear();
    try {
      renderWithToast(
        <MyCalendarsList
          status="ready"
          calendars={[item({ id: "keep", myRole: "owner", name: "Keep" })]}
          hasMore={false}
        />,
      );
      fireEvent.click(screen.getByRole("button", { name: /actions for keep/i }));
      fireEvent.click(screen.getByRole("menuitem", { name: "Delete" }));
      const undo = screen.getByRole("button", { name: /undo/i });
      fireEvent.click(undo);
      vi.advanceTimersByTime(10000);
      expect(deleteCalendarMock).not.toHaveBeenCalled();
      expect(screen.getByText("Keep")).toBeTruthy();
    } finally {
      vi.useRealTimers();
    }
  });

  it("opens the rename sheet when Rename is selected", () => {
    renderWithToast(
      <MyCalendarsList status="ready" calendars={[item({ myRole: "owner" })]} hasMore={false} />,
    );
    fireEvent.click(screen.getByRole("button", { name: /actions for trip/i }));
    fireEvent.click(screen.getByRole("menuitem", { name: "Rename" }));
    expect(screen.getByRole("dialog", { name: /rename calendar/i })).toBeTruthy();
  });
});
