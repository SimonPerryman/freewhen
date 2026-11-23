import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { act, fireEvent, render, screen } from "@testing-library/react";

let search = new URLSearchParams();
vi.mock("next/navigation", () => ({ useSearchParams: () => search }));

const sendMagicLinkMock = vi.fn();
vi.mock("@/lib/api", async () => {
  const actual = await vi.importActual<typeof import("@/lib/api")>("@/lib/api");
  return { ...actual, sendMagicLink: (...args: unknown[]) => sendMagicLinkMock(...args) };
});

import CheckEmailPage from "./page";

beforeEach(() => {
  vi.useFakeTimers();
  sendMagicLinkMock.mockReset();
  sendMagicLinkMock.mockResolvedValue(undefined);
  search = new URLSearchParams({ email: "a@b.co", return_url: "/calendars" });
});
afterEach(() => vi.useRealTimers());

describe("CheckEmailPage", () => {
  it("shows the address, a different-email link and a cooling-down resend", () => {
    render(<CheckEmailPage />);
    expect(document.querySelector("strong")?.textContent).toBe("a@b.co");
    expect(screen.queryByRole("link", { name: /^sign in$/i })).toBeNull();
    expect(screen.getByRole("link", { name: "Use a different email" })).toHaveAttribute(
      "href",
      "/login?return_url=%2Fcalendars",
    );
    expect(screen.getByRole("button", { name: /resend link \(30s\)/i })).toBeDisabled();
  });

  it("enables resend after 30s and restarts the cooldown after sending", async () => {
    render(<CheckEmailPage />);
    for (let i = 0; i < 30; i++) await act(() => vi.advanceTimersByTimeAsync(1000));
    const button = screen.getByRole("button", { name: "Resend link" });
    expect(button).toBeEnabled();
    fireEvent.click(button);
    await act(() => Promise.resolve());
    expect(sendMagicLinkMock).toHaveBeenCalledWith("a@b.co", "/calendars");
    expect(screen.getByRole("button", { name: /resend link \(30s\)/i })).toBeDisabled();
  });
});
