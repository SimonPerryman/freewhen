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

    expect(screen.queryByText(/^Finalise/)).not.toBeInTheDocument();
  });

  it("shows a primary Finalise for the top date and a chip on the other rows", () => {
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

    // The top date gets the full-width primary with the date in its label;
    // the other row keeps the small chip.
    expect(screen.getByRole("button", { name: "Finalise Mon 16 Jun" })).toBeInTheDocument();
    expect(screen.getAllByText("Finalise")).toHaveLength(1);
  });

  it("chip rows confirm with a short label so the row stays on one line", () => {
    const participants = [makeParticipant("a"), makeParticipant("b"), makeParticipant("c")];
    const freeByDate = new Map([
      [futureDateStr(1), new Set(["a", "b", "c"])],
      [futureDateStr(2), new Set(["a", "b"])],
    ]);
    const onFinalise = vi.fn();

    render(
      <SummaryView
        participants={participants}
        freeByDate={freeByDate}
        canFinalise
        onFinalise={onFinalise}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Finalise Tue 17 Jun" }));
    fireEvent.click(screen.getByRole("button", { name: "Confirm?" }));
    expect(onFinalise).toHaveBeenCalledWith(futureDateStr(2));
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

    fireEvent.click(screen.getByRole("button", { name: "Finalise Mon 16 Jun" }));
    expect(onFinalise).not.toHaveBeenCalled();

    // Confirmation button appears with date label
    fireEvent.click(screen.getByRole("button", { name: "Confirm Mon 16 Jun?" }));

    expect(onFinalise).toHaveBeenCalledWith(date);
  });

  it("renders a preference pill on rows with voters and not on rows without", () => {
    const participants = [makeParticipant("a"), makeParticipant("b"), makeParticipant("c")];
    const everyoneFree = futureDateStr(1); // Mon 16 Jun
    const mostFree = futureDateStr(2); // Tue 17 Jun
    const freeByDate = new Map([
      [everyoneFree, new Set(["a", "b", "c"])],
      [mostFree, new Set(["a", "b"])],
    ]);
    const preferredByDate = new Map<string, Set<string>>([[mostFree, new Set(["a"])]]);

    render(
      <SummaryView
        participants={participants}
        freeByDate={freeByDate}
        preferredByDate={preferredByDate}
      />,
    );

    // Static n=1 pill on the most-free row carries the voter's name.
    expect(screen.getByText("P-a")).toBeInTheDocument();
    // No pill on the everyone-free row (no voters there).
    expect(screen.queryByRole("button", { name: /prefer/ })).not.toBeInTheDocument();
  });

  it("renders a count-pill button when more than one voter prefers a date", () => {
    const participants = [makeParticipant("a"), makeParticipant("b"), makeParticipant("c")];
    const date = futureDateStr(1);
    const freeByDate = new Map([[date, new Set(["a", "b", "c"])]]);
    const preferredByDate = new Map<string, Set<string>>([[date, new Set(["a", "b"])]]);

    render(
      <SummaryView
        participants={participants}
        freeByDate={freeByDate}
        preferredByDate={preferredByDate}
      />,
    );

    expect(
      screen.getByRole("button", { name: "2 people prefer this day. Show who." }),
    ).toBeInTheDocument();
  });

  it("renders a notes pill on rows with notes and not on rows without", () => {
    const participants = [makeParticipant("a"), makeParticipant("b"), makeParticipant("c")];
    const everyoneFree = futureDateStr(1);
    const mostFree = futureDateStr(2);
    const freeByDate = new Map([
      [everyoneFree, new Set(["a", "b", "c"])],
      [mostFree, new Set(["a", "b"])],
    ]);
    const notesByDateAndAuthor = new Map<string, Map<string, string>>([
      [everyoneFree, new Map([["a", "after 6pm"]])],
    ]);

    render(
      <SummaryView
        participants={participants}
        freeByDate={freeByDate}
        notesByDateAndAuthor={notesByDateAndAuthor}
      />,
    );

    expect(
      screen.getByRole("button", { name: "1 note on this day. Show it." }),
    ).toBeInTheDocument();
  });

  it("renders both prefer and notes pills on the same row", () => {
    const participants = [makeParticipant("a"), makeParticipant("b"), makeParticipant("c")];
    const date = futureDateStr(1);
    const freeByDate = new Map([[date, new Set(["a", "b", "c"])]]);
    const preferredByDate = new Map<string, Set<string>>([[date, new Set(["a", "b"])]]);
    const notesByDateAndAuthor = new Map<string, Map<string, string>>([
      [date, new Map([["c", "anytime works"]])],
    ]);

    render(
      <SummaryView
        participants={participants}
        freeByDate={freeByDate}
        preferredByDate={preferredByDate}
        notesByDateAndAuthor={notesByDateAndAuthor}
      />,
    );

    expect(
      screen.getByRole("button", { name: "2 people prefer this day. Show who." }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "1 note on this day. Show it." }),
    ).toBeInTheDocument();
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

    fireEvent.click(screen.getByRole("button", { name: "Finalise Mon 16 Jun" }));
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));

    expect(onFinalise).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Finalise Mon 16 Jun" })).toBeInTheDocument();
  });
});
