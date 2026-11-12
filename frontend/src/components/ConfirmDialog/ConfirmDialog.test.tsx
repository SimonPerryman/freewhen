import { describe, it, expect, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import ConfirmDialog from "./ConfirmDialog";

function setup(props: Partial<React.ComponentProps<typeof ConfirmDialog>> = {}) {
  const onConfirm = vi.fn();
  const onCancel = vi.fn();
  render(
    <ConfirmDialog
      title="Delete calendar?"
      message="This can't be undone."
      confirmLabel="Delete"
      onConfirm={onConfirm}
      onCancel={onCancel}
      {...props}
    />,
  );
  return { onConfirm, onCancel };
}

describe("ConfirmDialog", () => {
  it("renders an accessible alertdialog with title and message", () => {
    setup();
    const dialog = screen.getByRole("alertdialog", { name: "Delete calendar?" });
    expect(dialog).toHaveAccessibleDescription("This can't be undone.");
  });

  it("focuses the confirm button initially", () => {
    setup();
    expect(screen.getByRole("button", { name: "Delete" })).toHaveFocus();
  });

  it("calls onConfirm and onCancel from the buttons", () => {
    const { onConfirm, onCancel } = setup();
    fireEvent.click(screen.getByRole("button", { name: "Delete" }));
    expect(onConfirm).toHaveBeenCalledTimes(1);
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(onCancel).toHaveBeenCalledTimes(1);
  });

  it("cancels on Escape and on backdrop click, not on dialog click", () => {
    const { onCancel } = setup();
    fireEvent.keyDown(document, { key: "Escape" });
    expect(onCancel).toHaveBeenCalledTimes(1);

    const dialog = screen.getByRole("alertdialog");
    fireEvent.click(dialog);
    expect(onCancel).toHaveBeenCalledTimes(1);

    fireEvent.click(dialog.parentElement!);
    expect(onCancel).toHaveBeenCalledTimes(2);
  });

  it("uses custom labels", () => {
    setup({ confirmLabel: "Leave", cancelLabel: "Stay" });
    expect(screen.getByRole("button", { name: "Leave" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Stay" })).toBeInTheDocument();
  });
});
