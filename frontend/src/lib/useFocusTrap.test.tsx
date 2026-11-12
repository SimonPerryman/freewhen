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

function TrapPanel({
  label,
  onClose,
  onEscape,
}: {
  label: string;
  onClose: () => void;
  onEscape?: () => void;
}) {
  const ref = useFocusTrap<HTMLDivElement>({ onEscape });
  return (
    <div ref={ref}>
      <button type="button">{`${label} action`}</button>
      <button type="button" onClick={onClose}>{`close ${label}`}</button>
    </div>
  );
}

// An outer trap (like the Settings sheet) that opens an inner one (like a
// confirm dialog) from its last button.
function Stacked({
  onOuterEscape,
  onInnerEscape,
  closeOnEscape = false,
}: {
  onOuterEscape?: () => void;
  onInnerEscape?: () => void;
  closeOnEscape?: boolean;
}) {
  const [inner, setInner] = useState(false);
  const ref = useFocusTrap<HTMLDivElement>({ onEscape: () => onOuterEscape?.() });
  return (
    <>
      <div ref={ref}>
        <button type="button">outer first</button>
        <button type="button" onClick={() => setInner(true)}>
          open inner
        </button>
      </div>
      {inner ? (
        <TrapPanel
          label="inner"
          onClose={() => setInner(false)}
          onEscape={() => {
            onInnerEscape?.();
            if (closeOnEscape) setInner(false);
          }}
        />
      ) : null}
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

  it("only the topmost trap handles Escape when traps are stacked", () => {
    const outer = vi.fn();
    const inner = vi.fn();
    render(<Stacked onOuterEscape={outer} onInnerEscape={inner} />);
    fireEvent.click(screen.getByRole("button", { name: "open inner" }));
    expect(screen.getByRole("button", { name: "inner action" })).toHaveFocus();

    fireEvent.keyDown(document, { key: "Escape" });
    expect(inner).toHaveBeenCalledTimes(1);
    expect(outer).not.toHaveBeenCalled();
  });

  it("only the topmost trap wraps Tab, and the lower one resumes once it closes", () => {
    render(<Stacked />);
    const opener = screen.getByRole("button", { name: "open inner" });
    act(() => opener.focus());
    fireEvent.click(opener);
    const innerAction = screen.getByRole("button", { name: "inner action" });
    // Inner trap has a single focusable: Tab must stay on it, not jump to
    // the outer trap's first element.
    fireEvent.keyDown(document, { key: "Tab" });
    expect(innerAction).toHaveFocus();

    fireEvent.click(screen.getByRole("button", { name: "close inner" }));
    // Focus goes back to the element that opened the inner trap...
    expect(opener).toHaveFocus();
    // ...and the outer trap handles Tab again (opener is its last element).
    fireEvent.keyDown(document, { key: "Tab" });
    expect(screen.getByRole("button", { name: "outer first" })).toHaveFocus();
  });

  it("Escape on the inner trap closes only it and restores focus to its opener", () => {
    render(<Stacked closeOnEscape />);
    const opener = screen.getByRole("button", { name: "open inner" });
    act(() => opener.focus());
    fireEvent.click(opener);
    fireEvent.keyDown(document, { key: "Escape" });
    expect(screen.queryByRole("button", { name: "inner action" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "outer first" })).toBeInTheDocument();
    expect(opener).toHaveFocus();
  });

  it("does not try to restore focus to an opener that has left the page", () => {
    function Gone() {
      const [opener, setOpener] = useState(true);
      const [open, setOpen] = useState(false);
      return (
        <>
          {opener ? (
            <button type="button" onClick={() => setOpen(true)}>
              opener
            </button>
          ) : null}
          {open ? (
            <TrapPanel
              label="panel"
              onClose={() => {
                setOpener(false);
                setOpen(false);
              }}
            />
          ) : null}
        </>
      );
    }
    render(<Gone />);
    const opener = screen.getByRole("button", { name: "opener" });
    act(() => opener.focus());
    fireEvent.click(opener);
    const focusSpy = vi.spyOn(opener, "focus");
    fireEvent.click(screen.getByRole("button", { name: "close panel" }));
    expect(focusSpy).not.toHaveBeenCalled();
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
