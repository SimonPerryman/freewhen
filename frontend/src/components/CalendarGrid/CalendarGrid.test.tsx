import { describe, it, expect, vi, beforeEach, type Mock } from "vitest";
import type { ComponentProps } from "react";
import { fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import CalendarGrid, { computeHeatPercent } from "./CalendarGrid";
import { COLOUR_PALETTE, textOnColour } from "@/lib/overlap";
import type { Participant } from "@/lib/types";

type CalendarGridProps = ComponentProps<typeof CalendarGrid>;

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
  let onToggle: Mock<CalendarGridProps["onToggle"]>;

  beforeEach(() => {
    onToggle = vi.fn<CalendarGridProps["onToggle"]>();
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
    // Only free slots carry the participant colour; the rest are empty outlines
    expect((freeSlots[0] as HTMLElement).style.backgroundColor).not.toBe("");
    expect((faintSlots[0] as HTMLElement).style.backgroundColor).toBe("");
  });

  it("groups the bulk tools in a Quick fill disclosure", () => {
    const ps = makeParticipants(1);
    render(
      <CalendarGrid
        participants={ps}
        freeByDate={new Map()}
        currentParticipantId={ps[0].id}
        onToggle={onToggle}
        onRangeToggle={vi.fn()}
        quickFillExtra={<button type="button">Import extra</button>}
        joined={true}
      />,
    );
    const details = screen.getByText("Quick fill").closest("details")!;
    expect(details).toContainElement(screen.getByRole("button", { name: "Weekends" }));
    expect(details).toContainElement(screen.getByRole("button", { name: "Import extra" }));
  });

  it("Quick fill summary has a decorative chevron and option names stay plain", () => {
    const ps = makeParticipants(1);
    render(
      <CalendarGrid
        participants={ps}
        freeByDate={new Map()}
        currentParticipantId={ps[0].id}
        onToggle={onToggle}
        onRangeToggle={vi.fn()}
        joined={true}
      />,
    );
    const summary = screen.getByText("Quick fill").closest("summary")!;
    expect(summary.querySelector("svg")).toHaveAttribute("aria-hidden", "true");
    // The week glyphs are aria-hidden, so the accessible names don't change.
    for (const name of ["Weekends", "Weekdays"]) {
      const button = screen.getByRole("button", { name });
      expect(button.querySelector("svg")).toHaveAttribute("aria-hidden", "true");
    }
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
    // Pin "today" to a non-Monday so the current week has at least one past
    // cell (the grid always starts on the Monday of the current week).
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2030-01-03T12:00:00Z")); // Thursday
    try {
      const ps = makeParticipants(3);
      const past = pastDateStr(2);
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
    } finally {
      vi.useRealTimers();
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

  it("shows Today button only after navigating forward, and jumps back when clicked", async () => {
    const user = userEvent.setup();
    render(
      <CalendarGrid
        participants={[]}
        freeByDate={new Map()}
        currentParticipantId={null}
        onToggle={onToggle}
        joined={false}
      />,
    );

    expect(screen.queryByRole("button", { name: /jump back to today/i })).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /next week/i }));
    const todayBtn = screen.getByRole("button", { name: /jump back to today/i });
    expect(todayBtn).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /next week/i }));
    await user.click(screen.getByRole("button", { name: /next week/i }));
    expect(screen.getByRole("button", { name: /previous week/i })).toBeEnabled();

    await user.click(screen.getByRole("button", { name: /jump back to today/i }));
    expect(screen.queryByRole("button", { name: /jump back to today/i })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: /previous week/i })).toBeDisabled();
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

  it("toolbar buttons only render when joined, not locked, and onRangeToggle provided", () => {
    const onRangeToggle = vi.fn();
    const ps = makeParticipants(1);
    const { rerender } = render(
      <CalendarGrid
        participants={ps}
        freeByDate={new Map()}
        currentParticipantId={ps[0].id}
        onToggle={onToggle}
        onRangeToggle={onRangeToggle}
        joined={true}
      />,
    );
    expect(screen.getByRole("button", { name: /weekends/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /weekdays/i })).toBeInTheDocument();

    // not joined → no toolbar
    rerender(
      <CalendarGrid
        participants={ps}
        freeByDate={new Map()}
        currentParticipantId={ps[0].id}
        onToggle={onToggle}
        onRangeToggle={onRangeToggle}
        joined={false}
      />,
    );
    expect(screen.queryByRole("button", { name: /weekends/i })).not.toBeInTheDocument();

    // locked → no toolbar
    rerender(
      <CalendarGrid
        participants={ps}
        freeByDate={new Map()}
        currentParticipantId={ps[0].id}
        onToggle={onToggle}
        onRangeToggle={onRangeToggle}
        joined={true}
        finalisedDate={futureDateStr(2)}
      />,
    );
    expect(screen.queryByRole("button", { name: /weekends/i })).not.toBeInTheDocument();

    // no onRangeToggle → no toolbar
    rerender(
      <CalendarGrid
        participants={ps}
        freeByDate={new Map()}
        currentParticipantId={ps[0].id}
        onToggle={onToggle}
        joined={true}
      />,
    );
    expect(screen.queryByRole("button", { name: /weekends/i })).not.toBeInTheDocument();
  });

  it("quick action: weekends selects all weekend cells when any unselected", async () => {
    const onRangeToggle = vi.fn();
    const ps = makeParticipants(1);
    const user = userEvent.setup();

    render(
      <CalendarGrid
        participants={ps}
        freeByDate={new Map()}
        currentParticipantId={ps[0].id}
        onToggle={onToggle}
        onRangeToggle={onRangeToggle}
        selectedDates={new Set()}
        joined={true}
      />,
    );

    await user.click(screen.getByRole("button", { name: /weekends/i }));

    expect(onRangeToggle).toHaveBeenCalledTimes(1);
    const [dates, target] = onRangeToggle.mock.calls[0];
    expect(target).toBe(true);
    // Every date must be a Sat or Sun
    for (const d of dates) {
      const [y, m, day] = d.split("-").map(Number);
      const dt = new Date(y, m - 1, day);
      const dow = dt.getDay();
      expect(dow === 0 || dow === 6).toBe(true);
    }
    expect(dates.length).toBeGreaterThan(0);
  });

  it("quick action: weekdays deselects when all matching visible are selected", async () => {
    const onRangeToggle = vi.fn();
    const ps = makeParticipants(1);
    const user = userEvent.setup();

    // Pre-compute all visible weekdays (future + non-finalised) and seed them as selected
    const today = new Date();
    today.setHours(0, 0, 0, 0);
    const monday = new Date(today);
    const day = monday.getDay();
    const diff = day === 0 ? -6 : 1 - day;
    monday.setDate(monday.getDate() + diff);
    const todayStr = (() => {
      const y = today.getFullYear();
      const m = String(today.getMonth() + 1).padStart(2, "0");
      const d = String(today.getDate()).padStart(2, "0");
      return `${y}-${m}-${d}`;
    })();

    const weekdays = new Set<string>();
    for (let i = 0; i < 35; i++) {
      const d = new Date(monday);
      d.setDate(d.getDate() + i);
      const dow = d.getDay();
      const y = d.getFullYear();
      const m = String(d.getMonth() + 1).padStart(2, "0");
      const day2 = String(d.getDate()).padStart(2, "0");
      const s = `${y}-${m}-${day2}`;
      if (s < todayStr) continue;
      if (dow !== 0 && dow !== 6) weekdays.add(s);
    }

    render(
      <CalendarGrid
        participants={ps}
        freeByDate={new Map()}
        currentParticipantId={ps[0].id}
        onToggle={onToggle}
        onRangeToggle={onRangeToggle}
        selectedDates={weekdays}
        joined={true}
      />,
    );

    await user.click(screen.getByRole("button", { name: /weekdays/i }));

    expect(onRangeToggle).toHaveBeenCalledTimes(1);
    const [dates, target] = onRangeToggle.mock.calls[0];
    expect(target).toBe(false);
    expect(new Set(dates)).toEqual(weekdays);
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

  it("renders a note dot when a participant has a note on a date", () => {
    const ps = makeParticipants(2);
    const date = futureDateStr(2);
    const noteAuthorsByDate = new Map([[date, new Set([ps[0].id])]]);

    render(
      <CalendarGrid
        participants={ps}
        freeByDate={new Map()}
        noteAuthorsByDate={noteAuthorsByDate}
        currentParticipantId={ps[0].id}
        onToggle={onToggle}
        joined={true}
      />,
    );

    const dots = screen.getAllByTestId("note-dot");
    expect(dots).toHaveLength(1);
    // Single-author rule: colour matches the author's participant colour.
    expect(dots[0].getAttribute("style")).toContain(ps[0].colour);
  });

  it("note dot is neutral when more than one participant has a note", () => {
    const ps = makeParticipants(3);
    const date = futureDateStr(2);
    const noteAuthorsByDate = new Map([[date, new Set([ps[0].id, ps[1].id])]]);

    render(
      <CalendarGrid
        participants={ps}
        freeByDate={new Map()}
        noteAuthorsByDate={noteAuthorsByDate}
        currentParticipantId={ps[0].id}
        onToggle={onToggle}
        joined={true}
      />,
    );

    const dots = screen.getAllByTestId("note-dot");
    expect(dots).toHaveLength(1);
    // Neutral rule: no inline --note-dot-colour → falls through to the CSS
    // default (var(--text-secondary)).
    const style = dots[0].getAttribute("style") ?? "";
    expect(style).not.toContain(ps[0].colour);
    expect(style).not.toContain(ps[1].colour);
  });

  it("tap on an unselected cell toggles (marks free)", async () => {
    const ps = makeParticipants(1);
    const futureDate = futureDateStr(3);
    const user = userEvent.setup();
    const onOpenDay = vi.fn();

    render(
      <CalendarGrid
        participants={ps}
        freeByDate={new Map()}
        currentParticipantId={ps[0].id}
        onToggle={onToggle}
        onOpenDay={onOpenDay}
        selectedDates={new Set()}
        joined={true}
      />,
    );

    const d = new Date();
    d.setDate(d.getDate() + 3);
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
    const cell = screen.getByRole("button", {
      name: new RegExp(`${monthName} ${d.getDate()}, ${d.getFullYear()}`),
    });
    await user.click(cell);
    expect(onToggle).toHaveBeenCalledWith(futureDate);
    expect(onOpenDay).not.toHaveBeenCalled();
  });

  it("renders each participant's initial inside their slot (n=3)", () => {
    const names = ["Alex", "Bo", "Cam"];
    const ps = names.map((n, i) => makeParticipant(`p${i}`, i, n));
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

    // Pick any rendered cell's slot for each participant and assert the initial is inside.
    for (const [i, expected] of [
      [0, "A"],
      [1, "B"],
      [2, "C"],
    ] as const) {
      const slot = screen.getAllByTestId(`slot-${ps[i].id}`)[0];
      expect(within(slot).getByText(expected)).toBeTruthy();
    }
  });

  it("renders initials for n=1 and n=9 layouts", () => {
    for (const n of [1, 9] as const) {
      const names = ["Alex", "Bo", "Cam", "Dee", "Ed", "Fi", "Gus", "Hal", "Ivy"].slice(0, n);
      const ps = names.map((nm, i) => makeParticipant(`p${i}-${n}`, i, nm));
      const freeByDate = new Map([[futureDateStr(3), new Set([ps[0].id])]]);

      const { unmount } = render(
        <CalendarGrid
          participants={ps}
          freeByDate={freeByDate}
          currentParticipantId={ps[0].id}
          onToggle={onToggle}
          joined={true}
        />,
      );

      for (let i = 0; i < n; i++) {
        const expected = names[i][0];
        const slot = screen.getAllByTestId(`slot-${ps[i].id}`)[0];
        expect(within(slot).getByText(expected)).toBeTruthy();
      }
      unmount();
    }
  });

  it("initial glyph is aria-hidden and coloured for contrast against slot colour", () => {
    const ps = [makeParticipant("a", 0, "Alex")]; // COLOUR_PALETTE[0] (dark blue)
    const freeByDate = new Map([[futureDateStr(3), new Set([ps[0].id])]]);

    render(
      <CalendarGrid
        participants={ps}
        freeByDate={freeByDate}
        currentParticipantId={ps[0].id}
        onToggle={onToggle}
        joined={true}
      />,
    );

    const slot = screen
      .getAllByTestId(`slot-${ps[0].id}`)
      .find((s) => s.className.includes("slotFree"))!;
    const initial = within(slot).getByText("A");
    expect(initial.getAttribute("aria-hidden")).toBe("true");
    const actual = (initial as HTMLElement).style.color;
    const expected = textOnColour(ps[0].colour);
    const normalised =
      actual === "rgb(255, 255, 255)" ? "#fff" : actual === "rgb(0, 0, 0)" ? "#000" : actual;
    expect(normalised).toBe(expected);
  });

  it("renders a Clear button in the toolbar when onClear is provided", () => {
    const ps = makeParticipants(1);
    render(
      <CalendarGrid
        participants={ps}
        freeByDate={new Map()}
        currentParticipantId={ps[0].id}
        onToggle={onToggle}
        onRangeToggle={vi.fn()}
        onClear={vi.fn()}
        canClear={true}
        joined={true}
      />,
    );
    expect(screen.getByRole("button", { name: /^clear$/i })).toBeEnabled();
  });

  it("Clear button is hidden when canClear is false", () => {
    const ps = makeParticipants(1);
    render(
      <CalendarGrid
        participants={ps}
        freeByDate={new Map()}
        currentParticipantId={ps[0].id}
        onToggle={onToggle}
        onRangeToggle={vi.fn()}
        onClear={vi.fn()}
        canClear={false}
        joined={true}
      />,
    );
    expect(screen.queryByRole("button", { name: /^clear$/i })).toBeNull();
  });

  it("clicking Clear calls onClear", async () => {
    const onClear = vi.fn();
    const user = userEvent.setup();
    const ps = makeParticipants(1);
    render(
      <CalendarGrid
        participants={ps}
        freeByDate={new Map()}
        currentParticipantId={ps[0].id}
        onToggle={onToggle}
        onRangeToggle={vi.fn()}
        onClear={onClear}
        canClear={true}
        joined={true}
      />,
    );
    await user.click(screen.getByRole("button", { name: /^clear$/i }));
    expect(onClear).toHaveBeenCalledTimes(1);
  });

  it("Clear button uses the secondary visual style (margin-left: auto grouping)", () => {
    const ps = makeParticipants(1);
    render(
      <CalendarGrid
        participants={ps}
        freeByDate={new Map()}
        currentParticipantId={ps[0].id}
        onToggle={onToggle}
        onRangeToggle={vi.fn()}
        onClear={vi.fn()}
        canClear={true}
        joined={true}
      />,
    );
    const clearBtn = screen.getByRole("button", { name: /^clear$/i });
    expect(clearBtn.className).toMatch(/toolbarButtonSecondary/);
  });

  it("tap on a cell the current user has marked free opens the day sheet", async () => {
    const ps = makeParticipants(1);
    const futureDate = futureDateStr(3);
    const user = userEvent.setup();
    const onOpenDay = vi.fn();

    render(
      <CalendarGrid
        participants={ps}
        freeByDate={new Map()}
        currentParticipantId={ps[0].id}
        onToggle={onToggle}
        onOpenDay={onOpenDay}
        selectedDates={new Set([futureDate])}
        joined={true}
      />,
    );

    const d = new Date();
    d.setDate(d.getDate() + 3);
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
    const cell = screen.getByRole("button", {
      name: new RegExp(`${monthName} ${d.getDate()}, ${d.getFullYear()}`),
    });
    await user.click(cell);
    expect(onOpenDay).toHaveBeenCalledWith(futureDate);
    expect(onToggle).not.toHaveBeenCalled();
  });

  describe("preference heat", () => {
    function findCell(date: string): HTMLElement {
      const d = new Date(`${date}T00:00:00`);
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
      return screen.getByRole("button", {
        name: new RegExp(`${monthName} ${d.getDate()}, ${d.getFullYear()}`),
      });
    }

    it("does not render the legacy vote badge", () => {
      const ps = makeParticipants(2);
      const futureDate = futureDateStr(3);
      render(
        <CalendarGrid
          participants={ps}
          freeByDate={new Map([[futureDate, new Set([ps[0].id])]])}
          voteCountsByDate={new Map([[futureDate, 2]])}
          currentParticipantId={null}
          onToggle={onToggle}
          joined={false}
        />,
      );
      expect(screen.queryAllByTestId("vote-badge")).toHaveLength(0);
    });

    it("computeHeatPercent folds free + preference using half-weight preference", () => {
      // No participants → no heat
      expect(computeHeatPercent(0, 0, 0)).toBe(0);
      // Free votes alone: 1/2 → 0.5 * 14 = 7
      expect(computeHeatPercent(2, 1, 0)).toBe(7);
      // Free + half-weight preference: 0.5 + 0.5 * 0.5 = 0.75 → 11 (rounded)
      expect(computeHeatPercent(2, 1, 1)).toBe(11);
      // Preference only nudges from cold: 0.5 * 1/4 = 0.125 → 2
      expect(computeHeatPercent(4, 0, 1)).toBe(2);
      // Capped at the 14% accent-mix ceiling even when preferences pile on
      expect(computeHeatPercent(2, 2, 2)).toBe(14);
      expect(computeHeatPercent(2, 2, 10)).toBe(14);
    });

    it("includes preference count in the aria-label", () => {
      const ps = makeParticipants(2);
      const futureDate = futureDateStr(3);
      render(
        <CalendarGrid
          participants={ps}
          freeByDate={new Map([[futureDate, new Set([ps[0].id])]])}
          voteCountsByDate={new Map([[futureDate, 2]])}
          currentParticipantId={null}
          onToggle={onToggle}
          joined={false}
        />,
      );
      const cell = findCell(futureDate);
      expect(cell.getAttribute("aria-label")).toContain("Preferred by 2 people.");
    });
  });

  describe("instruction strip", () => {
    beforeEach(() => {
      try {
        localStorage.clear();
      } catch {
        /* ignore */
      }
    });

    it("is not shown once the calendar is finalised", () => {
      const ps = makeParticipants(1);
      render(
        <CalendarGrid
          participants={ps}
          freeByDate={new Map<string, Set<string>>()}
          currentParticipantId={ps[0].id}
          onToggle={onToggle}
          joined
          finalisedDate="2099-01-01"
        />,
      );
      expect(screen.queryByTestId("instruction-strip")).toBeNull();
    });

    it("disappears after the first tap and stays gone", async () => {
      const ps = makeParticipants(1);
      const props = {
        participants: ps,
        freeByDate: new Map<string, Set<string>>(),
        currentParticipantId: ps[0].id,
        onToggle,
        joined: true,
      };
      const { unmount } = render(<CalendarGrid {...props} />);
      expect(screen.getByTestId("instruction-strip")).toBeTruthy();
      const cell = screen
        .getAllByTestId("day-cell")
        .find((c) => c.tagName === "BUTTON" && !(c as HTMLButtonElement).disabled);
      fireEvent.click(cell as HTMLElement);
      expect(screen.queryByTestId("instruction-strip")).toBeNull();
      unmount();
      render(<CalendarGrid {...props} />);
      expect(screen.queryByTestId("instruction-strip")).toBeNull();
    });

    it("renders when currentParticipantId is set", () => {
      const ps = makeParticipants(1);
      render(
        <CalendarGrid
          participants={ps}
          freeByDate={new Map()}
          currentParticipantId={ps[0].id}
          onToggle={onToggle}
          joined={true}
        />,
      );
      expect(screen.getByTestId("instruction-strip")).toBeTruthy();
    });

    it("does not render for spectator (no participant)", () => {
      render(
        <CalendarGrid
          participants={[]}
          freeByDate={new Map()}
          currentParticipantId={null}
          onToggle={onToggle}
          joined={false}
        />,
      );
      expect(screen.queryByTestId("instruction-strip")).toBeNull();
    });
  });

  describe("first-mark coachmark", () => {
    beforeEach(() => {
      try {
        localStorage.clear();
      } catch {
        /* ignore */
      }
    });

    it("appears after first toggling a cell on", async () => {
      const user = userEvent.setup();
      const ps = makeParticipants(1);
      render(
        <CalendarGrid
          calendarId="cal-1"
          participants={ps}
          freeByDate={new Map()}
          currentParticipantId={ps[0].id}
          onToggle={onToggle}
          joined={true}
        />,
      );
      const cells = screen
        .getAllByTestId("day-cell")
        .filter((c) => c.tagName === "BUTTON" && !(c as HTMLButtonElement).disabled);
      await user.click(cells[0]);
      // 220ms delay before reveal
      await new Promise((r) => setTimeout(r, 260));
      expect(screen.queryByTestId("coachmark-prefer-note")).toBeTruthy();
    });

    it("does not re-appear after dismissal flag is set", async () => {
      const user = userEvent.setup();
      const ps = makeParticipants(1);
      localStorage.setItem("freewhen.coachmark.detailSheet.cal-1.p0", "1");
      render(
        <CalendarGrid
          calendarId="cal-1"
          participants={ps}
          freeByDate={new Map()}
          currentParticipantId={ps[0].id}
          onToggle={onToggle}
          joined={true}
        />,
      );
      const cells = screen
        .getAllByTestId("day-cell")
        .filter((c) => c.tagName === "BUTTON" && !(c as HTMLButtonElement).disabled);
      await user.click(cells[0]);
      await new Promise((r) => setTimeout(r, 260));
      expect(screen.queryByTestId("coachmark-prefer-note")).toBeNull();
    });
  });
});

describe("CoachmarkPreferNote (via CalendarGrid)", () => {
  beforeEach(() => {
    try {
      localStorage.clear();
    } catch {
      /* ignore */
    }
  });

  it("Got it dismisses and writes the localStorage flag", async () => {
    const user = userEvent.setup();
    const ps = [{ id: "p0", calendarId: "cal-1", name: "P0", colour: COLOUR_PALETTE[0] }];
    const onToggleLocal = vi.fn();
    render(
      <CalendarGrid
        calendarId="cal-1"
        participants={ps}
        freeByDate={new Map()}
        currentParticipantId="p0"
        onToggle={onToggleLocal}
        joined={true}
      />,
    );
    const cells = screen
      .getAllByTestId("day-cell")
      .filter((c) => c.tagName === "BUTTON" && !(c as HTMLButtonElement).disabled);
    await user.click(cells[0]);
    await new Promise((r) => setTimeout(r, 260));
    const dismiss = screen.getByText("Got it");
    await user.click(dismiss);
    expect(screen.queryByTestId("coachmark-prefer-note")).toBeNull();
    expect(localStorage.getItem("freewhen.coachmark.detailSheet.cal-1.p0")).toBe("1");
  });
});
