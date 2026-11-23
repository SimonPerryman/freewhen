import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
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

  it("holds the date only: no unfinalise or reopen controls", () => {
    render(<FinalisedBanner date="2026-04-26" />);
    expect(screen.queryByText(/Unfinalise|Reopen|Change the date|owner only/)).toBeNull();
  });
});
