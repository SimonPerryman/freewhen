import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

const pushMock = vi.fn();
const createCalendarMock = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: pushMock }),
}));

vi.mock("@/lib/api", async () => {
  const actual = await vi.importActual<typeof import("@/lib/api")>("@/lib/api");
  return {
    ...actual,
    createCalendar: (...args: unknown[]) => createCalendarMock(...args),
  };
});

import Home from "./page";

beforeEach(() => {
  pushMock.mockReset();
  createCalendarMock.mockReset();
});

describe("Home (calendar creation page)", () => {
  it("renders the form", () => {
    render(<Home />);
    expect(screen.getByRole("heading", { name: /freewhen/i })).toBeInTheDocument();
    expect(screen.getByLabelText(/event name/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /create calendar/i })).toBeInTheDocument();
  });

  it("submits with trimmed name and navigates on success", async () => {
    createCalendarMock.mockResolvedValue({
      id: "abc123",
      name: "Trip",
      createdAt: "2026-01-01T00:00:00Z",
    });

    const user = userEvent.setup();
    render(<Home />);

    await user.type(screen.getByLabelText(/event name/i), "  Trip  ");
    await user.click(screen.getByRole("button", { name: /create calendar/i }));

    await waitFor(() => {
      expect(createCalendarMock).toHaveBeenCalledWith({ name: "Trip" });
    });
    expect(pushMock).toHaveBeenCalledWith("/calendars/abc123");
  });

  it("renders API error message on failure", async () => {
    const { ApiError } = await import("@/lib/api");
    createCalendarMock.mockRejectedValue(new ApiError(500, "boom"));

    const user = userEvent.setup();
    render(<Home />);

    await user.click(screen.getByRole("button", { name: /create calendar/i }));

    expect(await screen.findByRole("alert")).toHaveTextContent("boom");
    expect(pushMock).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: /create calendar/i })).toBeEnabled();
  });
});
