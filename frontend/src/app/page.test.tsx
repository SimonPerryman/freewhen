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
  localStorage.clear();
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
    expect(screen.getByRole("heading", { name: /find when/i })).toBeInTheDocument();
    expect(screen.getByLabelText(/event name/i)).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getByRole("button", { name: /^create/i })).toBeInTheDocument();
    });
  });

  it("hints that sign-in is needed only when signed out", async () => {
    getMeMock.mockRejectedValue(new Error("401"));
    renderWithAuth();
    expect(
      await screen.findByText(/you.ll sign in with your email to save it/i),
    ).toBeInTheDocument();
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
      expect(screen.getByRole("button", { name: /^create/i })).toBeInTheDocument();
    });

    await user.type(screen.getByLabelText(/event name/i), "  Trip  ");
    await user.click(screen.getByRole("button", { name: /^create/i }));

    await waitFor(() => {
      expect(createCalendarMock).toHaveBeenCalledWith({ name: "Trip" });
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
      expect(screen.getByRole("button", { name: /^create/i })).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: /^create/i }));

    expect(await screen.findByRole("alert")).toHaveTextContent("boom");
    expect(pushMock).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: /^create/i })).toBeEnabled();
  });

  it("redirects to login when not authenticated", async () => {
    getMeMock.mockResolvedValue(null);

    const user = userEvent.setup();
    renderWithAuth();

    await waitFor(() => {
      expect(screen.getByRole("button", { name: /^create/i })).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: /^create/i }));

    expect(pushMock).toHaveBeenCalledWith("/login");
  });

  it("keeps the typed name when signed out and creates the calendar after sign-in", async () => {
    getMeMock.mockResolvedValue(null);
    const user = userEvent.setup();
    const first = renderWithAuth();
    await waitFor(() => {
      expect(screen.getByRole("button", { name: /^create/i })).toBeInTheDocument();
    });
    await user.type(screen.getByLabelText(/event name/i), "  Ski week ");
    await user.click(screen.getByRole("button", { name: /^create/i }));
    expect(pushMock).toHaveBeenCalledWith("/login");
    first.unmount();

    // Back from the magic link, now signed in.
    getMeMock.mockResolvedValue({ id: "u1", email: "test@example.com", name: "Test" });
    createCalendarMock.mockResolvedValue({ id: "xyz", name: "Ski week", createdAt: "" });
    renderWithAuth();
    await waitFor(() => {
      expect(createCalendarMock).toHaveBeenCalledWith({ name: "Ski week" });
    });
    expect(pushMock).toHaveBeenLastCalledWith("/calendars/xyz");
  });

  it("does not create a duplicate on reload after the pending name was consumed", async () => {
    localStorage.setItem(
      "freewhen:pending-calendar-name",
      JSON.stringify({ name: "Trip", savedAt: Date.now() }),
    );
    getMeMock.mockResolvedValue({ id: "u1", email: "test@example.com", name: "Test" });
    createCalendarMock.mockResolvedValue({ id: "a1", name: "Trip", createdAt: "" });
    const first = renderWithAuth();
    await waitFor(() => expect(createCalendarMock).toHaveBeenCalledTimes(1));
    first.unmount();
    renderWithAuth();
    await waitFor(() => {
      expect(screen.getByRole("button", { name: /^create/i })).toBeInTheDocument();
    });
    expect(createCalendarMock).toHaveBeenCalledTimes(1);
  });

  it("shows an error and keeps the name when the resumed create fails", async () => {
    localStorage.setItem(
      "freewhen:pending-calendar-name",
      JSON.stringify({ name: "Trip", savedAt: Date.now() }),
    );
    getMeMock.mockResolvedValue({ id: "u1", email: "test@example.com", name: "Test" });
    const { ApiError } = await import("@/lib/api");
    createCalendarMock.mockRejectedValue(new ApiError(500, "boom"));
    renderWithAuth();
    expect(await screen.findByRole("alert")).toHaveTextContent("boom");
    expect(screen.getByLabelText(/event name/i)).toHaveValue("Trip");
  });

  it("ignores a stale pending name", async () => {
    localStorage.setItem(
      "freewhen:pending-calendar-name",
      JSON.stringify({ name: "Old", savedAt: Date.now() - 2 * 60 * 60 * 1000 }),
    );
    getMeMock.mockResolvedValue({ id: "u1", email: "test@example.com", name: "Test" });
    renderWithAuth();
    await waitFor(() => {
      expect(screen.getByRole("button", { name: /^create/i })).toBeInTheDocument();
    });
    expect(createCalendarMock).not.toHaveBeenCalled();
  });
});
