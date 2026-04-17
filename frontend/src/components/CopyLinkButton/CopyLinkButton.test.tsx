import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, act, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import CopyLinkButton from "./CopyLinkButton";

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

describe("CopyLinkButton", () => {
  it("copies URL to clipboard and shows feedback", async () => {
    const user = userEvent.setup();
    render(<CopyLinkButton />);

    const button = await screen.findByRole("button", { name: /copy link/i });
    await user.click(button);

    await waitFor(() => {
      expect(button).toHaveTextContent("Copied!");
    });
  });

  it("reverts text after 2 seconds", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    render(<CopyLinkButton />);

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
});
