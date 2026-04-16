import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import JoinForm from "./JoinForm";

describe("JoinForm", () => {
  let onJoin: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    onJoin = vi.fn().mockResolvedValue(undefined);
  });

  it("submits trimmed name and calls onJoin", async () => {
    const user = userEvent.setup();
    render(<JoinForm onJoin={onJoin} error={null} />);

    await user.type(screen.getByLabelText(/your name/i), "  Alice  ");
    await user.click(screen.getByRole("button", { name: /join/i }));

    expect(onJoin).toHaveBeenCalledWith("Alice");
  });

  it("disables submit when input is empty", () => {
    render(<JoinForm onJoin={onJoin} error={null} />);
    expect(screen.getByRole("button", { name: /join/i })).toBeDisabled();
  });

  it("shows error state", () => {
    render(<JoinForm onJoin={onJoin} error="That name is already taken" />);
    expect(screen.getByRole("alert")).toHaveTextContent("That name is already taken");
  });

  it("pre-fills default name", () => {
    render(<JoinForm onJoin={onJoin} error={null} defaultName="Bob" />);
    expect(screen.getByLabelText(/your name/i)).toHaveValue("Bob");
  });
});
