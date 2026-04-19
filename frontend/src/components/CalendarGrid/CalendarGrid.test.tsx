import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import CalendarGrid from "./CalendarGrid";
import { COLOUR_PALETTE } from "@/lib/overlap";
import type { Participant } from "@/lib/types";

function futureDateStr(daysAhead: number): string {
  const d = new Date();
  d.setDate(d.getDate() + daysAhead);
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${y}-${m}-${day}`;
}

function pastDateStr(daysBack: number): string {
  const d = new Date();
  d.setDate(d.getDate() - daysBack);
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${y}-${m}-${day}`;
}

function makeParticipant(id: string, index: number, name?: string): Participant {
  return {
    id,
    calendarId: "cal-1",
    name: name ?? `P${index}`,
    colour: COLOUR_PALETTE[index % COLOUR_PALETTE.length],
  };
}

function makeParticipants(n: number): Participant[] {
  return Array.from({ length: n }, (_, i) => makeParticipant(`p${i}`, i));
}

describe("CalendarGrid", () => {
  let onToggle: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    onToggle = vi.fn();
  });

  it("renders 35 day cell positions (5 weeks)", () => {
    render(
      <CalendarGrid
        participants={[]}
        freeByDate={new Map()}
        currentParticipantId={null}
        onToggle={onToggle}
        joined={false}
      />,
    );

    const cells = screen.getAllByTestId("day-cell");
    expect(cells).toHaveLength(35);
  });

  it("n=0: cells have no slot container", () => {
    render(
      <CalendarGrid
        participants={[]}
        freeByDate={new Map()}
        currentParticipantId={null}
        onToggle={onToggle}
        joined={false}
      />,
    );

    expect(screen.queryAllByTestId("slot-grid")).toHaveLength(0);
  });

  it("n=1: one slot per cell, free class when free, faint when not", () => {
    const ps = makeParticipants(1);
    const futureDate = futureDateStr(3);
    const freeByDate = new Map([[futureDate, new Set([ps[0].id])]]);

    render(
      <CalendarGrid
        participants={ps}
        freeByDate={freeByDate}
        currentParticipantId={ps[0].id}
        onToggle={onToggle}
        joined={true}
      />,
    );

    // Each non-past cell has one slot for the single participant
    const allSlots = screen.getAllByTestId(`slot-${ps[0].id}`);
    expect(allSlots.length).toBeGreaterThan(0);
    // The slot on the free date should have the free class
    const freeSlots = allSlots.filter((s) => s.className.includes("slotFree"));
    expect(freeSlots).toHaveLength(1);
    // The rest should be faint
    const faintSlots = allSlots.filter((s) => s.className.includes("slotFaint"));
    expect(faintSlots.length).toBe(allSlots.length - 1);
  });

  it("n=1: no everyone-free highlight even when sole participant is free", () => {
    const ps = makeParticipants(1);
    const futureDate = futureDateStr(3);
    const freeByDate = new Map([[futureDate, new Set([ps[0].id])]]);

    const { container } = render(
      <CalendarGrid
        participants={ps}
        freeByDate={freeByDate}
        currentParticipantId={ps[0].id}
        onToggle={onToggle}
        joined={true}
      />,
    );

    // everyoneFree class should not appear with only 1 participant
    const everyoneFreeEls = container.querySelectorAll('[class*="everyoneFree"]');
    expect(everyoneFreeEls).toHaveLength(0);
  });

  it("n=3: three slots rendered per cell", () => {
    const ps = makeParticipants(3);
    const futureDate = futureDateStr(3);
    const freeByDate = new Map([[futureDate, new Set([ps[0].id, ps[2].id])]]);

    render(
      <CalendarGrid
        participants={ps}
        freeByDate={freeByDate}
        currentParticipantId={ps[0].id}
        onToggle={onToggle}
        joined={true}
      />,
    );

    // Each non-past cell should have 3 slots
    const slotGrids = screen.getAllByTestId("slot-grid");
    expect(slotGrids.length).toBeGreaterThan(0);
    // Each grid should have 3 participant slots
    const firstGrid = slotGrids[0];
    const slots = firstGrid.querySelectorAll("[data-testid^='slot-p']");
    expect(slots).toHaveLength(3);
  });

  it("n=5: 3x2 grid with 5 slots + 1 empty", () => {
    const ps = makeParticipants(5);
    const futureDate = futureDateStr(3);
    const freeByDate = new Map([[futureDate, new Set([ps[0].id])]]);

    render(
      <CalendarGrid
        participants={ps}
        freeByDate={freeByDate}
        currentParticipantId={ps[0].id}
        onToggle={onToggle}
        joined={true}
      />,
    );

    const slotGrids = screen.getAllByTestId("slot-grid");
    const firstGrid = slotGrids[0];
    // 5 participant slots + 1 empty = 6 children
    expect(firstGrid.children).toHaveLength(6);
    // Verify grid template
    expect(firstGrid.style.gridTemplateColumns).toBe("repeat(3, 1fr)");
    expect(firstGrid.style.gridTemplateRows).toBe("repeat(2, 1fr)");
  });

  it("n=9: 3x3 grid with 9 slots, no empty", () => {
    const ps = makeParticipants(9);
    const futureDate = futureDateStr(3);
    const freeByDate = new Map([[futureDate, new Set([ps[0].id])]]);

    render(
      <CalendarGrid
        participants={ps}
        freeByDate={freeByDate}
        currentParticipantId={ps[0].id}
        onToggle={onToggle}
        joined={true}
      />,
    );

    const slotGrids = screen.getAllByTestId("slot-grid");
    const firstGrid = slotGrids[0];
    // 9 participant slots, 0 empty
    expect(firstGrid.children).toHaveLength(9);
    expect(firstGrid.style.gridTemplateColumns).toBe("repeat(3, 1fr)");
    expect(firstGrid.style.gridTemplateRows).toBe("repeat(3, 1fr)");
  });

  it("everyone-free class applied when all participants free and N >= 2", () => {
    const ps = makeParticipants(3);
    const futureDate = futureDateStr(3);
    const freeByDate = new Map([[futureDate, new Set(ps.map((p) => p.id))]]);

    const { container } = render(
      <CalendarGrid
        participants={ps}
        freeByDate={freeByDate}
        currentParticipantId={ps[0].id}
        onToggle={onToggle}
        joined={true}
      />,
    );

    const everyoneFreeEls = container.querySelectorAll('[class*="everyoneFree"]');
    expect(everyoneFreeEls.length).toBeGreaterThan(0);
  });

  it("aria-label includes date and participant names", () => {
    const ps = [makeParticipant("a", 0, "Alice"), makeParticipant("b", 1, "Bob")];
    const futureDate = futureDateStr(3);
    const freeByDate = new Map([[futureDate, new Set(["a"])]]);

    render(
      <CalendarGrid
        participants={ps}
        freeByDate={freeByDate}
        currentParticipantId={"a"}
        onToggle={onToggle}
        joined={true}
      />,
    );

    const button = screen.getByRole("button", {
      name: new RegExp(`Free: Alice.*Not free: Bob`),
    });
    expect(button).toBeTruthy();
  });

  it("past dates render as non-interactive placeholders", () => {
    const ps = makeParticipants(3);
    const past = pastDateStr(5);
    const freeByDate = new Map([[past, new Set([ps[0].id])]]);

    render(
      <CalendarGrid
        participants={ps}
        freeByDate={freeByDate}
        currentParticipantId={ps[0].id}
        onToggle={onToggle}
        joined={true}
      />,
    );

    // Grid still holds 35 positions total
    const cells = screen.getAllByTestId("day-cell");
    expect(cells).toHaveLength(35);

    // Placeholder cells are divs (not buttons), aria-hidden, with no content
    const placeholders = cells.filter((c) => c.tagName === "DIV");
    expect(placeholders.length).toBeGreaterThan(0);
    for (const ph of placeholders) {
      expect(ph.getAttribute("aria-hidden")).toBe("true");
      expect(ph.children).toHaveLength(0);
    }
  });

  it("clicking a cell calls onToggle(date)", async () => {
    const ps = makeParticipants(3);
    const futureDate = futureDateStr(3);
    const user = userEvent.setup();

    render(
      <CalendarGrid
        participants={ps}
        freeByDate={new Map()}
        currentParticipantId={ps[0].id}
        onToggle={onToggle}
        joined={true}
      />,
    );

    // aria-label format is "Apr 20, 2026 Not free: P0, P1, P2."
    // Find the button containing the future date's day number
    const d = new Date();
    d.setDate(d.getDate() + 3);
    const dayNum = d.getDate();
    const monthName = [
      "Jan",
      "Feb",
      "Mar",
      "Apr",
      "May",
      "Jun",
      "Jul",
      "Aug",
      "Sep",
      "Oct",
      "Nov",
      "Dec",
    ][d.getMonth()];
    const year = d.getFullYear();
    const labelPattern = new RegExp(`${monthName} ${dayNum}, ${year}`);
    const button = screen.getByRole("button", { name: labelPattern });
    await user.click(button);
    expect(onToggle).toHaveBeenCalledWith(futureDate);
  });

  it("disables day buttons when user has not joined", () => {
    render(
      <CalendarGrid
        participants={[]}
        freeByDate={new Map()}
        currentParticipantId={null}
        onToggle={onToggle}
        joined={false}
      />,
    );

    const buttons = screen.getAllByRole("button", { name: /\w+ \d+, \d{4}/ });
    for (const button of buttons) {
      expect(button).toBeDisabled();
    }
  });

  it("shows navigation arrows and disables back at week 0", () => {
    render(
      <CalendarGrid
        participants={[]}
        freeByDate={new Map()}
        currentParticipantId={null}
        onToggle={onToggle}
        joined={false}
      />,
    );

    expect(screen.getByRole("button", { name: /previous week/i })).toBeDisabled();
    expect(screen.getByRole("button", { name: /next week/i })).toBeEnabled();
  });

  it("disables all cells when finalised even if joined", async () => {
    const ps = makeParticipants(2);
    const date = futureDateStr(2);
    const freeByDate = new Map([[date, new Set([ps[0].id, ps[1].id])]]);

    render(
      <CalendarGrid
        participants={ps}
        freeByDate={freeByDate}
        currentParticipantId={ps[0].id}
        onToggle={onToggle}
        joined={true}
        finalisedDate={date}
      />,
    );

    const buttons = screen.getAllByRole("button", { name: /\w+ \d+, \d{4}/ });
    for (const button of buttons) {
      expect(button).toBeDisabled();
    }

    const user = userEvent.setup();
    // Try to click the finalised cell — should not fire onToggle
    await user.click(buttons[0]);
    expect(onToggle).not.toHaveBeenCalled();
  });

  it("marks the finalised cell with the finalised class", () => {
    const ps = makeParticipants(1);
    const date = futureDateStr(2);
    const freeByDate = new Map([[date, new Set([ps[0].id])]]);

    render(
      <CalendarGrid
        participants={ps}
        freeByDate={freeByDate}
        currentParticipantId={ps[0].id}
        onToggle={onToggle}
        joined={true}
        finalisedDate={date}
      />,
    );

    const finalisedCell = screen.getByRole("button", { name: /finalised/i });
    expect(finalisedCell.className).toMatch(/finalised/);
  });
});
