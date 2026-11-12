import { describe, it, expect, vi, beforeEach } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";

vi.mock("@/lib/api", async () => {
  const actual = await vi.importActual<typeof import("@/lib/api")>("@/lib/api");
  return { ...actual, renameCalendar: vi.fn() };
});
vi.mock("@/lib/analytics", () => ({ default: { capture: vi.fn() } }));

import { ApiError, renameCalendar } from "@/lib/api";
import RenameCalendarSheet from "./RenameCalendarSheet";

const mockRename = vi.mocked(renameCalendar);

function setup() {
  const onClose = vi.fn();
  const onRenamed = vi.fn();
  render(
    <RenameCalendarSheet
      calendarId="cal-1"
      currentName="Team dinner"
      onClose={onClose}
      onRenamed={onRenamed}
    />,
  );
  return { onClose, onRenamed, input: screen.getByLabelText("Name") as HTMLInputElement };
}

describe("RenameCalendarSheet", () => {
  beforeEach(() => {
    mockRename.mockReset();
  });

  it("prefills and focuses the name input", () => {
    const { input } = setup();
    expect(screen.getByRole("dialog", { name: "Rename calendar" })).toBeInTheDocument();
    expect(input).toHaveValue("Team dinner");
    expect(input).toHaveFocus();
  });

  it("closes without calling the API when the name is unchanged", () => {
    const { onClose } = setup();
    fireEvent.click(screen.getByRole("button", { name: /save/i }));
    expect(onClose).toHaveBeenCalledTimes(1);
    expect(mockRename).not.toHaveBeenCalled();
  });

  it("renames with the trimmed name and reports the result", async () => {
    mockRename.mockResolvedValue({ name: "Team lunch" } as Awaited<
      ReturnType<typeof renameCalendar>
    >);
    const { input, onRenamed } = setup();
    fireEvent.change(input, { target: { value: "  Team lunch  " } });
    fireEvent.click(screen.getByRole("button", { name: /save/i }));
    await waitFor(() => expect(onRenamed).toHaveBeenCalledWith("Team lunch"));
    expect(mockRename).toHaveBeenCalledWith("cal-1", "Team lunch");
  });

  it("shows an error when the name is too long", async () => {
    mockRename.mockRejectedValue(new ApiError(400, "too long", null, "name_too_long"));
    const { input } = setup();
    fireEvent.change(input, { target: { value: "Something else" } });
    fireEvent.click(screen.getByRole("button", { name: /save/i }));
    expect(await screen.findByRole("alert")).toHaveTextContent("That name is too long.");
    expect(input).toHaveAttribute("aria-invalid", "true");
    await waitFor(() => expect(input).toHaveFocus());
  });

  it("closes on Escape and Cancel", () => {
    const { onClose } = setup();
    fireEvent.keyDown(document, { key: "Escape" });
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(onClose).toHaveBeenCalledTimes(2);
  });
});
