import { describe, it, expect, vi, afterEach, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import BestDay from "./BestDay";
import type { Participant } from "@/lib/types";

function makeParticipant(id: string): Participant {
  return { id, calendarId: "cal-1", name: `P-${id}`, colour: "#000" };
}

beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(new Date(2026, 5, 1)); // 2026-06-01
});

afterEach(() => {
  vi.useRealTimers();
});

describe("BestDay", () => {
  it("renders nothing when fewer than 2 participants", () => {
    const { container } = render(
      <BestDay participants={[makeParticipant("a")]} freeByDate={new Map()} />,
    );
    expect(container.innerHTML).toBe("");
  });

  it("renders nothing when multiple dates have everyone free", () => {
    const participants = [makeParticipant("a"), makeParticipant("b")];
    const freeByDate = new Map([
      ["2026-06-10", new Set(["a", "b"])],
      ["2026-06-11", new Set(["a", "b"])],
    ]);

    const { container } = render(
      <BestDay participants={participants} freeByDate={freeByDate} />,
    );
    expect(container.innerHTML).toBe("");
  });

  it("renders 'Everyone's free' when exactly one date has all participants", () => {
    const participants = [makeParticipant("a"), makeParticipant("b")];
    const freeByDate = new Map([["2026-06-15", new Set(["a", "b"])]]);

    render(<BestDay participants={participants} freeByDate={freeByDate} />);

    expect(screen.getByText("Best day")).toBeInTheDocument();
    expect(screen.getByText("Monday 15 Jun")).toBeInTheDocument();
    expect(screen.getByText("Everyone's free")).toBeInTheDocument();
  });

  it("renders 'X of Y free' when no universal date but one clear winner", () => {
    const participants = [
      makeParticipant("a"),
      makeParticipant("b"),
      makeParticipant("c"),
    ];
    const freeByDate = new Map([
      ["2026-06-10", new Set(["a", "b"])],
      ["2026-06-11", new Set(["a"])],
    ]);

    render(<BestDay participants={participants} freeByDate={freeByDate} />);

    expect(screen.getByText("Best day")).toBeInTheDocument();
    expect(screen.getByText("Wednesday 10 Jun")).toBeInTheDocument();
    expect(screen.getByText("2 of 3 free")).toBeInTheDocument();
  });

  it("renders nothing when top two dates are tied", () => {
    const participants = [
      makeParticipant("a"),
      makeParticipant("b"),
      makeParticipant("c"),
    ];
    const freeByDate = new Map([
      ["2026-06-10", new Set(["a", "b"])],
      ["2026-06-11", new Set(["b", "c"])],
    ]);

    const { container } = render(
      <BestDay participants={participants} freeByDate={freeByDate} />,
    );
    expect(container.innerHTML).toBe("");
  });
});
