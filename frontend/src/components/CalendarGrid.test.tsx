import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import CalendarGrid from "./CalendarGrid";

function futureDateStr(daysAhead: number): string {
  const d = new Date();
  d.setDate(d.getDate() + daysAhead);
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${y}-${m}-${day}`;
}

describe("CalendarGrid", () => {
  let onToggle: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    onToggle = vi.fn();
  });

  it("renders 35 day cells (5 weeks)", () => {
    render(
      <CalendarGrid
        selectedDates={new Set()}
        otherAvailability={new Map()}
        onToggle={onToggle}
        userColour={null}
        joined={false}
      />,
    );

    // 7 header cells + 35 day buttons
    const buttons = screen.getAllByRole("button", { name: /\d{4}-\d{2}-\d{2}/ });
    expect(buttons).toHaveLength(35);
  });

  it("calls onToggle when a future day is clicked by a joined user", async () => {
    const futureDate = futureDateStr(3);
    const user = userEvent.setup();

    render(
      <CalendarGrid
        selectedDates={new Set()}
        otherAvailability={new Map()}
        onToggle={onToggle}
        userColour="#ff0000"
        joined={true}
      />,
    );

    const button = screen.getByRole("button", { name: new RegExp(futureDate) });
    await user.click(button);
    expect(onToggle).toHaveBeenCalledWith(futureDate);
  });

  it("disables day buttons when user has not joined", () => {
    render(
      <CalendarGrid
        selectedDates={new Set()}
        otherAvailability={new Map()}
        onToggle={onToggle}
        userColour={null}
        joined={false}
      />,
    );

    const buttons = screen.getAllByRole("button", { name: /\d{4}-\d{2}-\d{2}/ });
    for (const button of buttons) {
      expect(button).toBeDisabled();
    }
  });

  it("marks selected dates with aria-pressed", () => {
    const futureDate = futureDateStr(2);

    render(
      <CalendarGrid
        selectedDates={new Set([futureDate])}
        otherAvailability={new Map()}
        onToggle={onToggle}
        userColour="#ff0000"
        joined={true}
      />,
    );

    const button = screen.getByRole("button", { name: new RegExp(`${futureDate}.*selected`) });
    expect(button).toHaveAttribute("aria-pressed", "true");
  });

  it("shows navigation arrows and disables back at week 0", () => {
    render(
      <CalendarGrid
        selectedDates={new Set()}
        otherAvailability={new Map()}
        onToggle={onToggle}
        userColour={null}
        joined={false}
      />,
    );

    expect(screen.getByRole("button", { name: /previous week/i })).toBeDisabled();
    expect(screen.getByRole("button", { name: /next week/i })).toBeEnabled();
  });
});
