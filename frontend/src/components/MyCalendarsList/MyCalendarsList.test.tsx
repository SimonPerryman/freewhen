import { describe, expect, it, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import MyCalendarsList from "./MyCalendarsList";
import type { MyCalendarListItem } from "@/lib/types";

function item(overrides: Partial<MyCalendarListItem> = {}): MyCalendarListItem {
  return {
    id: "cal1",
    name: "Trip",
    emoji: "🎉",
    visibility: "open",
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
    render(<MyCalendarsList status="loading" calendars={[]} hasMore={false} />);
    expect(screen.getByLabelText("Loading calendars")).toBeTruthy();
  });

  it("renders the empty state with a CTA", () => {
    render(<MyCalendarsList status="ready" calendars={[]} hasMore={false} />);
    expect(screen.getByText(/haven't created or joined/i)).toBeTruthy();
    const cta = screen.getByRole("link", { name: /create a calendar/i });
    expect(cta.getAttribute("href")).toBe("/");
  });

  it("renders an Owner pill only for owned calendars", () => {
    render(
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
    render(
      <MyCalendarsList
        status="ready"
        calendars={[item({ finalisedDate: "2026-05-02" })]}
        hasMore={false}
      />,
    );
    expect(screen.getByText(/May/)).toBeTruthy();
  });

  it("links each card to the calendar page", () => {
    render(<MyCalendarsList status="ready" calendars={[item({ id: "abc123" })]} hasMore={false} />);
    const link = screen.getByRole("link", { name: /Trip/ });
    expect(link.getAttribute("href")).toBe("/calendars/abc123");
  });

  it("renders the cap footer when hasMore is true", () => {
    render(<MyCalendarsList status="ready" calendars={[item()]} hasMore={true} />);
    expect(screen.getByText(/100 most recent calendars/i)).toBeTruthy();
  });

  it("omits the cap footer when hasMore is false", () => {
    render(<MyCalendarsList status="ready" calendars={[item()]} hasMore={false} />);
    expect(screen.queryByText(/100 most recent calendars/i)).toBeNull();
  });

  it("shows an error message with a retry button", () => {
    const onRetry = vi.fn();
    render(<MyCalendarsList status="error" calendars={[]} hasMore={false} onRetry={onRetry} />);
    const button = screen.getByRole("button", { name: /try again/i });
    fireEvent.click(button);
    expect(onRetry).toHaveBeenCalledOnce();
  });

  it("uses a placeholder emoji when none is set", () => {
    render(<MyCalendarsList status="ready" calendars={[item({ emoji: "" })]} hasMore={false} />);
    expect(screen.getByText("📅")).toBeTruthy();
  });
});
