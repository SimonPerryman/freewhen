import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import FinalisedBanner from "./FinalisedBanner";

describe("FinalisedBanner", () => {
  it("renders the formatted date and default emoji", () => {
    render(<FinalisedBanner date="2026-04-26" />);

    expect(screen.getByText(/Confirmed/i)).toBeInTheDocument();
    expect(screen.getByText(/Sunday 26 April/)).toBeInTheDocument();
    expect(screen.getByText("📅")).toBeInTheDocument();
  });

  it("renders a custom emoji when provided", () => {
    render(<FinalisedBanner date="2026-04-26" emoji="🎉" />);
    expect(screen.getByText("🎉")).toBeInTheDocument();
  });

  it("hides Unfinalise button for non-owners", () => {
    render(<FinalisedBanner date="2026-04-26" onUnfinalise={() => {}} />);
    expect(screen.queryByText(/Unfinalise/)).not.toBeInTheDocument();
  });

  it("shows Unfinalise button for owners and triggers confirm flow", async () => {
    const onUnfinalise = vi.fn().mockResolvedValue(undefined);
    render(<FinalisedBanner date="2026-04-26" canUnfinalise onUnfinalise={onUnfinalise} />);

    fireEvent.click(screen.getByText("Unfinalise"));
    expect(onUnfinalise).not.toHaveBeenCalled();

    const confirmBtn = screen.getByText(/Reopen calendar\?/);
    fireEvent.click(confirmBtn);
    expect(onUnfinalise).toHaveBeenCalled();
  });

  it("cancel reverts the confirm state", () => {
    const onUnfinalise = vi.fn();
    render(<FinalisedBanner date="2026-04-26" canUnfinalise onUnfinalise={onUnfinalise} />);

    fireEvent.click(screen.getByText("Unfinalise"));
    fireEvent.click(screen.getByLabelText("Cancel"));

    expect(onUnfinalise).not.toHaveBeenCalled();
    expect(screen.getByText("Unfinalise")).toBeInTheDocument();
  });
});
