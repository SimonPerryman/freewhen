import { describe, it, expect, vi } from "vitest";
import { useRef, useState } from "react";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { useFocusTrap } from "./useFocusTrap";

function Trap({ onEscape }: { onEscape?: () => void }) {
  const [checked, setChecked] = useState(false);
  const inputRef = useRef<HTMLInputElement | null>(null);
  // Fresh arrow every render, as real callers do.
  const ref = useFocusTrap<HTMLDivElement>({
    onEscape: () => onEscape?.(),
    initialFocus: inputRef,
  });
  return (
    <div ref={ref}>
      <input ref={inputRef} aria-label="name" />
      <input
        type="checkbox"
        aria-label="include"
        checked={checked}
        onChange={(e) => setChecked(e.target.checked)}
      />
      <button type="button">last</button>
    </div>
  );
}

function Host({ onEscape }: { onEscape?: () => void }) {
  const [open, setOpen] = useState(false);
  return (
    <>
      <button type="button" onClick={() => setOpen((o) => !o)}>
        opener
      </button>
      {open ? <Trap onEscape={onEscape} /> : null}
    </>
  );
}

describe("useFocusTrap", () => {
  it("focuses initialFocus on mount", () => {
    render(<Trap />);
    expect(screen.getByLabelText("name")).toHaveFocus();
  });

  it("wraps Tab from last to first and Shift+Tab from first to last", () => {
    render(<Trap />);
    const first = screen.getByLabelText("name");
    const last = screen.getByRole("button", { name: "last" });

    act(() => last.focus());
    fireEvent.keyDown(document, { key: "Tab" });
    expect(first).toHaveFocus();

    fireEvent.keyDown(document, { key: "Tab", shiftKey: true });
    expect(last).toHaveFocus();
  });

  it("calls the latest onEscape on Escape", () => {
    const first = vi.fn();
    const second = vi.fn();
    const { rerender } = render(<Trap onEscape={first} />);
    rerender(<Trap onEscape={second} />);
    fireEvent.keyDown(document, { key: "Escape" });
    expect(first).not.toHaveBeenCalled();
    expect(second).toHaveBeenCalledTimes(1);
  });

  it("restores focus to the previously focused element on unmount", () => {
    render(<Host />);
    const opener = screen.getByRole("button", { name: "opener" });
    act(() => opener.focus());
    fireEvent.click(opener);
    expect(screen.getByLabelText("name")).toHaveFocus();

    act(() => opener.focus());
    fireEvent.click(opener);
    expect(screen.queryByLabelText("name")).not.toBeInTheDocument();
    expect(opener).toHaveFocus();
  });

  it("does not move focus when a re-render passes a new onEscape", () => {
    render(<Trap onEscape={() => {}} />);
    const checkbox = screen.getByLabelText("include");
    act(() => checkbox.focus());
    // Toggling state re-renders Trap, creating a new onEscape arrow.
    fireEvent.click(checkbox);
    expect(checkbox).toBeChecked();
    expect(checkbox).toHaveFocus();
  });
});
