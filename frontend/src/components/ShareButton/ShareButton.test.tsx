import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, act, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
vi.mock("@/lib/analytics", () => ({
  default: { init: vi.fn(), identify: vi.fn(), reset: vi.fn(), capture: vi.fn() },
}));

import analytics from "@/lib/analytics";
import ShareButton from "./ShareButton";

beforeEach(() => {
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

describe("ShareButton (hero variant)", () => {
  it("copies URL to clipboard and shows feedback when share API is unavailable", async () => {
    const user = userEvent.setup();
    render(<ShareButton calendarName="Weekend trip" variant="hero" />);

    const button = await screen.findByRole("button", { name: /copy link/i });
    await user.click(button);

    await waitFor(() => {
      expect(button).toHaveTextContent("Copied!");
    });
  });

  it("reverts text after the feedback window", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    render(<ShareButton calendarName="Weekend trip" variant="hero" />);

    const button = await screen.findByRole("button", { name: /copy link/i });
    await user.click(button);

    await waitFor(() => {
      expect(button).toHaveTextContent("Copied!");
    });

    act(() => {
      vi.advanceTimersByTime(2000);
    });

    expect(button).toHaveTextContent("Copy link");
    vi.useRealTimers();
  });

  it("renders no share icon SVG", () => {
    const { container } = render(<ShareButton calendarName="Weekend trip" variant="hero" />);
    expect(container.querySelector("svg")).toBeNull();
  });

  it("uses native share with the pre-finalise message when available", async () => {
    const shareMock = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "share", {
      value: shareMock,
      configurable: true,
      writable: true,
    });

    const user = userEvent.setup();
    render(<ShareButton calendarName="Weekend trip" calendarEmoji="🏕️" variant="hero" />);

    const button = await screen.findByRole("button", { name: /share/i });
    await user.click(button);

    expect(shareMock).toHaveBeenCalledWith({
      title: "Weekend trip",
      text: "🏕️ Help me find a date for Weekend trip — mark when you're free:",
      url: expect.any(String),
    });

    Object.defineProperty(navigator, "share", {
      value: undefined,
      configurable: true,
      writable: true,
    });
  });

  it("uses the finalised-state message when finalisedDate is set", async () => {
    const shareMock = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "share", {
      value: shareMock,
      configurable: true,
      writable: true,
    });

    const user = userEvent.setup();
    render(
      <ShareButton
        calendarName="Weekend trip"
        calendarEmoji="🏕️"
        finalisedDate="2026-05-01"
        variant="hero"
      />,
    );

    const button = await screen.findByRole("button", { name: /share/i });
    await user.click(button);

    expect(shareMock).toHaveBeenCalledWith({
      title: "Weekend trip",
      text: "🏕️ Weekend trip is on Friday 1 May — add it to your calendar:",
      url: expect.any(String),
    });

    Object.defineProperty(navigator, "share", {
      value: undefined,
      configurable: true,
      writable: true,
    });
  });
});

describe("ShareButton (default variant)", () => {
  it("shows the pre-finalisation kicker by default", async () => {
    render(<ShareButton calendarName="Weekend trip" />);
    expect(await screen.findByText("Invite the rest of the group")).toBeInTheDocument();
    expect(screen.getByText("Send the link to the group")).toBeInTheDocument();
  });

  it("shows the finalised kicker when state='finalised'", async () => {
    render(<ShareButton calendarName="Weekend trip" state="finalised" />);
    expect(await screen.findByText("Tell everyone the date")).toBeInTheDocument();
  });

  it("renders no share icon SVG", () => {
    const { container } = render(<ShareButton calendarName="Weekend trip" />);
    expect(container.querySelector("svg")).toBeNull();
  });

  it("chip flips to 'Copied!' after clipboard fallback", async () => {
    const user = userEvent.setup();
    render(<ShareButton calendarName="Weekend trip" />);

    const button = await screen.findByRole("button");
    expect(button).toHaveTextContent("Share");
    await user.click(button);

    await waitFor(() => {
      expect(button).toHaveTextContent("Copied!");
    });
  });

  it("captures share_link_clicked with context derived from state", async () => {
    const captureSpy = vi.mocked(analytics.capture);
    captureSpy.mockClear();

    const user = userEvent.setup();
    render(<ShareButton calendarName="Weekend trip" state="finalised" />);

    const button = await screen.findByRole("button");
    await user.click(button);

    await waitFor(() => {
      expect(captureSpy).toHaveBeenCalledWith("share_link_clicked", {
        method: "clipboard",
        context: "finalised",
      });
    });
  });
});
