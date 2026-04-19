import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import SummaryView from "./SummaryView";
import type { Participant } from "@/lib/types";

function makeParticipant(id: string): Participant {
  return { id, calendarId: "cal-1", name: `P-${id}`, colour: "#000" };
}

// Use a fixed "now" so computeSummary sees our dates as future
function futureDateStr(daysAhead: number): string {
  const d = new Date(2025, 5, 15);
  d.setDate(d.getDate() + daysAhead);
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${y}-${m}-${day}`;
}

beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(new Date(2025, 5, 15));
});

describe("SummaryView", () => {
  it("returns null with fewer than 2 participants", () => {
    const { container } = render(
      <SummaryView participants={[makeParticipant("a")]} freeByDate={new Map()} />,
    );
    expect(container.innerHTML).toBe("");
  });

  it("renders everyone-free section with formatted dates", () => {
    const participants = [makeParticipant("a"), makeParticipant("b")];
    const date = futureDateStr(1); // 2025-06-16
    const freeByDate = new Map([[date, new Set(["a", "b"])]]);

    render(<SummaryView participants={participants} freeByDate={freeByDate} />);

    expect(screen.getByText("Summary")).toBeInTheDocument();
    expect(screen.getByText("Days everyone is free")).toBeInTheDocument();
    expect(screen.getByText("Mon 16 Jun")).toBeInTheDocument();
  });

  it("shows 'No days yet' when no everyone-free dates", () => {
    const participants = [makeParticipant("a"), makeParticipant("b")];
    const date = futureDateStr(1);
    const freeByDate = new Map([[date, new Set(["a"])]]); // only 1 of 2

    render(<SummaryView participants={participants} freeByDate={freeByDate} />);

    expect(screen.getByText("No days yet")).toBeInTheDocument();
  });

  it("renders most-free section with counts", () => {
    const participants = [makeParticipant("a"), makeParticipant("b"), makeParticipant("c")];
    const date = futureDateStr(2); // 2025-06-17
    const freeByDate = new Map([[date, new Set(["a", "b"])]]);

    render(<SummaryView participants={participants} freeByDate={freeByDate} />);

    expect(screen.getByText("Days most people are free")).toBeInTheDocument();
    expect(screen.getByText("Tue 17 Jun")).toBeInTheDocument();
    expect(screen.getByText("— 2/3 free")).toBeInTheDocument();
  });

  it("hides most-free sub-section when empty", () => {
    const participants = [makeParticipant("a"), makeParticipant("b")];
    const date = futureDateStr(1);
    const freeByDate = new Map([[date, new Set(["a", "b"])]]); // everyone free, no partial

    render(<SummaryView participants={participants} freeByDate={freeByDate} />);

    expect(screen.queryByText("Days most people are free")).not.toBeInTheDocument();
  });

  it("hides finalise button when canFinalise is false", () => {
    const participants = [makeParticipant("a"), makeParticipant("b")];
    const date = futureDateStr(1);
    const freeByDate = new Map([[date, new Set(["a", "b"])]]);

    render(<SummaryView participants={participants} freeByDate={freeByDate} />);

    expect(screen.queryByText("Finalise")).not.toBeInTheDocument();
  });

  it("shows finalise button on each row when canFinalise is true", () => {
    const participants = [makeParticipant("a"), makeParticipant("b"), makeParticipant("c")];
    const allFreeDate = futureDateStr(1);
    const partialFreeDate = futureDateStr(2);
    const freeByDate = new Map([
      [allFreeDate, new Set(["a", "b", "c"])],
      [partialFreeDate, new Set(["a", "b"])],
    ]);

    render(
      <SummaryView
        participants={participants}
        freeByDate={freeByDate}
        canFinalise
        onFinalise={() => {}}
      />,
    );

    // One on the everyone-free row, one on the most-free row
    const buttons = screen.getAllByText("Finalise");
    expect(buttons).toHaveLength(2);
  });

  it("two-step confirm flow calls onFinalise with the date", async () => {
    const participants = [makeParticipant("a"), makeParticipant("b")];
    const date = futureDateStr(1); // Mon 16 Jun
    const freeByDate = new Map([[date, new Set(["a", "b"])]]);
    const onFinalise = vi.fn().mockResolvedValue(undefined);

    render(
      <SummaryView
        participants={participants}
        freeByDate={freeByDate}
        canFinalise
        onFinalise={onFinalise}
      />,
    );

    fireEvent.click(screen.getByText("Finalise"));
    expect(onFinalise).not.toHaveBeenCalled();

    // Confirmation button appears with date label
    const confirmBtn = screen.getByText(/Confirm Mon 16 Jun/);
    fireEvent.click(confirmBtn);

    expect(onFinalise).toHaveBeenCalledWith(date);
  });

  it("cancel reverts the confirmation without calling onFinalise", () => {
    const participants = [makeParticipant("a"), makeParticipant("b")];
    const date = futureDateStr(1);
    const freeByDate = new Map([[date, new Set(["a", "b"])]]);
    const onFinalise = vi.fn();

    render(
      <SummaryView
        participants={participants}
        freeByDate={freeByDate}
        canFinalise
        onFinalise={onFinalise}
      />,
    );

    fireEvent.click(screen.getByText("Finalise"));
    fireEvent.click(screen.getByLabelText("Cancel"));

    expect(onFinalise).not.toHaveBeenCalled();
    expect(screen.getByText("Finalise")).toBeInTheDocument();
  });
});
