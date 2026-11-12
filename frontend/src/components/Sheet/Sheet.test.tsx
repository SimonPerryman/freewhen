import { useRef, useState } from "react";
import { describe, it, expect, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import Sheet from "./Sheet";

function Harness({
  onClose = vi.fn(),
  dismissible = true,
  focusSecond = false,
}: {
  onClose?: () => void;
  dismissible?: boolean;
  focusSecond?: boolean;
}) {
  const secondRef = useRef<HTMLButtonElement>(null);
  return (
    <Sheet
      title="Rename calendar"
      onClose={onClose}
      dismissible={dismissible}
      closeLabel="Close"
      initialFocus={focusSecond ? secondRef : undefined}
    >
      <p>Body text</p>
      <button type="button">First</button>
      <button type="button" ref={secondRef}>
        Second
      </button>
    </Sheet>
  );
}

describe("Sheet", () => {
  it("renders a modal dialog labelled by its title", () => {
    render(<Harness />);
    const dialog = screen.getByRole("dialog", { name: "Rename calendar" });
    expect(dialog).toHaveAttribute("aria-modal", "true");
    expect(screen.getByRole("heading", { level: 2, name: "Rename calendar" })).toBeInTheDocument();
  });

  it("supports alertdialog with a description", () => {
    render(
      <Sheet
        layout="dialog"
        role="alertdialog"
        title="Delete?"
        describedBy="sheet-test-body"
        onClose={vi.fn()}
      >
        <p id="sheet-test-body">This can&apos;t be undone.</p>
        <button type="button">OK</button>
      </Sheet>,
    );
    expect(screen.getByRole("alertdialog", { name: "Delete?" })).toHaveAccessibleDescription(
      "This can't be undone.",
    );
  });

  it("focuses the first focusable element (the close button) by default", () => {
    render(<Harness />);
    expect(screen.getByRole("button", { name: "Close" })).toHaveFocus();
  });

  it("focuses initialFocus when given", () => {
    render(<Harness focusSecond />);
    expect(screen.getByRole("button", { name: "Second" })).toHaveFocus();
  });

  it("traps Tab and Shift+Tab inside the panel", async () => {
    const user = userEvent.setup();
    render(
      <>
        <button type="button">Outside</button>
        <Harness focusSecond />
      </>,
    );
    const close = screen.getByRole("button", { name: "Close" });
    const second = screen.getByRole("button", { name: "Second" });
    expect(second).toHaveFocus();
    await user.tab();
    expect(close).toHaveFocus();
    await user.tab({ shift: true });
    expect(second).toHaveFocus();
  });

  it("restores focus to the opener when it unmounts", async () => {
    const user = userEvent.setup();
    function Opener() {
      const [open, setOpen] = useState(false);
      return (
        <>
          <button type="button" onClick={() => setOpen(true)}>
            Open
          </button>
          {open ? <Harness onClose={() => setOpen(false)} /> : null}
        </>
      );
    }
    render(<Opener />);
    const opener = screen.getByRole("button", { name: "Open" });
    await user.click(opener);
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(opener).toHaveFocus();
  });

  it("closes on Escape", async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    render(<Harness onClose={onClose} />);
    await user.keyboard("{Escape}");
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("closes on backdrop click but not on clicks inside the panel", () => {
    const onClose = vi.fn();
    render(<Harness onClose={onClose} />);
    const dialog = screen.getByRole("dialog");
    fireEvent.click(screen.getByText("Body text"));
    fireEvent.click(dialog);
    expect(onClose).not.toHaveBeenCalled();
    fireEvent.click(dialog.parentElement!);
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("closes from the close button", () => {
    const onClose = vi.fn();
    render(<Harness onClose={onClose} />);
    fireEvent.click(screen.getByRole("button", { name: "Close" }));
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("ignores Escape, backdrop and close button while not dismissible", async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    render(<Harness onClose={onClose} dismissible={false} focusSecond />);
    await user.keyboard("{Escape}");
    fireEvent.click(screen.getByRole("dialog").parentElement!);
    expect(screen.getByRole("button", { name: "Close" })).toBeDisabled();
    expect(onClose).not.toHaveBeenCalled();
  });
});
