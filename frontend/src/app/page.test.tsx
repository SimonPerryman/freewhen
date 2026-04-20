import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

vi.mock("@/lib/analytics", () => ({
  default: { init: vi.fn(), identify: vi.fn(), reset: vi.fn(), capture: vi.fn() },
}));

const pushMock = vi.fn();
const createCalendarMock = vi.fn();
const getMeMock = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: pushMock }),
}));

vi.mock("@/lib/api", async () => {
  const actual = await vi.importActual<typeof import("@/lib/api")>("@/lib/api");
  return {
    ...actual,
    createCalendar: (...args: unknown[]) => createCalendarMock(...args),
    getMe: () => getMeMock(),
  };
});

import Home from "./page";
import AuthProvider from "@/components/AuthProvider/AuthProvider";

beforeEach(() => {
  pushMock.mockReset();
  createCalendarMock.mockReset();
  getMeMock.mockReset();
});

function renderWithAuth() {
  return render(
    <AuthProvider>
      <Home />
    </AuthProvider>,
  );
}

describe("Home (calendar creation page)", () => {
  it("renders the form", async () => {
    getMeMock.mockResolvedValue({ id: "u1", email: "test@example.com", name: "Test" });
    renderWithAuth();
    expect(screen.getByRole("heading", { name: /freewhen/i })).toBeInTheDocument();
    expect(screen.getByLabelText(/event name/i)).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getByRole("button", { name: /create calendar/i })).toBeInTheDocument();
    });
  });

  it("submits with trimmed name and navigates on success", async () => {
    getMeMock.mockResolvedValue({ id: "u1", email: "test@example.com", name: "Test" });
    createCalendarMock.mockResolvedValue({
      id: "abc123",
      name: "Trip",
      createdAt: "2026-01-01T00:00:00Z",
    });

    const user = userEvent.setup();
    renderWithAuth();

    await waitFor(() => {
      expect(screen.getByRole("button", { name: /create calendar/i })).toBeInTheDocument();
    });

    await user.type(screen.getByLabelText(/event name/i), "  Trip  ");
    await user.click(screen.getByRole("button", { name: /create calendar/i }));

    await waitFor(() => {
      expect(createCalendarMock).toHaveBeenCalledWith({ name: "Trip", visibility: "open" });
    });
    expect(pushMock).toHaveBeenCalledWith("/calendars/abc123");
  });

  it("renders API error message on failure", async () => {
    getMeMock.mockResolvedValue({ id: "u1", email: "test@example.com", name: "Test" });
    const { ApiError } = await import("@/lib/api");
    createCalendarMock.mockRejectedValue(new ApiError(500, "boom"));

    const user = userEvent.setup();
    renderWithAuth();

    await waitFor(() => {
      expect(screen.getByRole("button", { name: /create calendar/i })).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: /create calendar/i }));

    expect(await screen.findByRole("alert")).toHaveTextContent("boom");
    expect(pushMock).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: /create calendar/i })).toBeEnabled();
  });

  it("redirects to login when not authenticated", async () => {
    const { ApiError } = await import("@/lib/api");
    getMeMock.mockRejectedValue(new ApiError(401, "not authenticated"));

    const user = userEvent.setup();
    renderWithAuth();

    await waitFor(() => {
      expect(screen.getByRole("button", { name: /sign in to create/i })).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: /sign in to create/i }));

    expect(pushMock).toHaveBeenCalledWith("/login?return_url=%2F");
  });
});
