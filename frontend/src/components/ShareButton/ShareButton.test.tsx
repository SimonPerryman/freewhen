import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, act, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
vi.mock("@/lib/analytics", () => ({
  default: { init: vi.fn(), identify: vi.fn(), reset: vi.fn(), capture: vi.fn() },
}));

import ShareButton from "./ShareButton";

beforeEach(() => {
  // jsdom doesn't implement navigator.clipboard — define it with a stub
  if (!navigator.clipboard) {
    Object.defineProperty(navigator, "clipboard", {
      value: {
        writeText: vi.fn().mockResolvedValue(undefined),
      },
      configurable: true,
    });
  } else {
    vi.spyOn(navigator.clipboard, "writeText").mockResolvedValue(undefined);
  }
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("ShareButton", () => {
  it("copies URL to clipboard and shows feedback when share API is unavailable", async () => {
    const user = userEvent.setup();
    render(<ShareButton calendarName="Weekend trip" />);

    const button = await screen.findByRole("button", { name: /copy link/i });
    await user.click(button);

    await waitFor(() => {
      expect(button).toHaveTextContent("Copied!");
    });
  });

  it("reverts text after 2 seconds", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    render(<ShareButton calendarName="Weekend trip" />);

    const button = await screen.findByRole("button", { name: /copy link/i });
    await user.click(button);

    await waitFor(() => {
      expect(button).toHaveTextContent("Copied!");
    });

    act(() => {
      vi.advanceTimersByTime(2100);
    });

    expect(button).toHaveTextContent("Copy link");
    vi.useRealTimers();
  });

  it("uses native share when available", async () => {
    const shareMock = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "share", {
      value: shareMock,
      configurable: true,
      writable: true,
    });

    const user = userEvent.setup();
    render(<ShareButton calendarName="Weekend trip" />);

    const button = await screen.findByRole("button", { name: /share/i });
    await user.click(button);

    expect(shareMock).toHaveBeenCalledWith({
      title: "Weekend trip",
      text: "Mark your availability on FreeWhen",
      url: expect.any(String),
    });

    // Clean up
    Object.defineProperty(navigator, "share", {
      value: undefined,
      configurable: true,
      writable: true,
    });
  });
});
