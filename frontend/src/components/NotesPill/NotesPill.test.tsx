import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import NotesPill, { type NotesPillNote } from "./NotesPill";

function n(id: string, name: string, text: string, colour = "#abcdef"): NotesPillNote {
  return { id, name, colour, text };
}

describe("NotesPill", () => {
  it("renders nothing when there are no notes", () => {
    const { container } = render(<NotesPill notes={[]} />);
    expect(container.innerHTML).toBe("");
  });

  it("n=1 renders a count pill labelled '1 note' and opens a popover with the single note", () => {
    render(<NotesPill notes={[n("a", "Sam", "after 6pm")]} date="2026-05-09" calendarId="cal-1" />);

    const trigger = screen.getByRole("button", { name: "1 note on this day. Show it." });
    expect(trigger).toHaveTextContent("1 note");
    expect(trigger).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByRole("group")).not.toBeInTheDocument();

    fireEvent.click(trigger);

    expect(trigger).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByRole("group", { name: "Notes on this day" })).toBeInTheDocument();
    expect(screen.getByText("Sam")).toBeInTheDocument();
    expect(screen.getByText("after 6pm")).toBeInTheDocument();
  });

  it("n>1 renders a count pill labelled '{n} notes' and opens with all rows", () => {
    const notes = [
      n("a", "Sam", "after 6pm"),
      n("b", "Tess", "only if we move dinner"),
      n("c", "Jo", "anytime"),
    ];
    render(<NotesPill notes={notes} date="2026-05-09" />);

    const trigger = screen.getByRole("button", { name: "3 notes on this day. Show them." });
    expect(trigger).toHaveTextContent("3 notes");

    fireEvent.click(trigger);

    expect(screen.getByRole("group", { name: "Notes on this day" })).toBeInTheDocument();
    expect(screen.getByText("Sam")).toBeInTheDocument();
    expect(screen.getByText("Tess")).toBeInTheDocument();
    expect(screen.getByText("Jo")).toBeInTheDocument();
  });

  it("Escape closes the popover", () => {
    render(<NotesPill notes={[n("a", "Sam", "x"), n("b", "Tess", "y")]} date="2026-05-09" />);
    fireEvent.click(screen.getByRole("button", { name: /notes on this day/ }));
    expect(screen.getByRole("group")).toBeInTheDocument();

    fireEvent.keyDown(document, { key: "Escape" });
    expect(screen.queryByRole("group")).not.toBeInTheDocument();
  });

  it("outside pointerdown closes the popover", () => {
    render(
      <div>
        <button data-testid="outside">outside</button>
        <NotesPill notes={[n("a", "Sam", "x")]} date="2026-05-09" />
      </div>,
    );
    fireEvent.click(screen.getByRole("button", { name: /note on this day/ }));
    expect(screen.getByRole("group")).toBeInTheDocument();

    fireEvent.pointerDown(screen.getByTestId("outside"));
    expect(screen.queryByRole("group")).not.toBeInTheDocument();
  });

  it("clicking a popover row calls onOpenDay with the date and closes the popover", () => {
    const onOpenDay = vi.fn();
    render(
      <NotesPill notes={[n("a", "Sam", "after 6pm")]} date="2026-05-09" onOpenDay={onOpenDay} />,
    );

    fireEvent.click(screen.getByRole("button", { name: /note on this day/ }));
    const row = screen.getByRole("button", { name: "Open Sat 9 May to read Sam's note" });
    fireEvent.click(row);

    expect(onOpenDay).toHaveBeenCalledWith("2026-05-09");
    expect(screen.queryByRole("group")).not.toBeInTheDocument();
  });

  it("renders rows as non-interactive list items when onOpenDay is not provided", () => {
    render(<NotesPill notes={[n("a", "Sam", "after 6pm")]} date="2026-05-09" />);
    fireEvent.click(screen.getByRole("button", { name: /note on this day/ }));

    // Only the trigger button exists; row is a plain <li>
    const buttons = screen.getAllByRole("button");
    expect(buttons).toHaveLength(1);
  });

  it("uses the participant colour for the swatch", () => {
    render(<NotesPill notes={[n("a", "Sam", "x", "#ff00ff")]} date="2026-05-09" />);
    fireEvent.click(screen.getByRole("button"));

    const group = screen.getByRole("group");
    const swatch = group.querySelector('[aria-hidden="true"]') as HTMLElement;
    expect(swatch).not.toBeNull();
    expect(swatch.style.backgroundColor).toBe("#ff00ff");
  });
});
