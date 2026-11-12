import { describe, it, expect, vi, beforeEach, type Mock } from "vitest";
import type { ComponentProps } from "react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import JoinForm from "./JoinForm";

type JoinFormProps = ComponentProps<typeof JoinForm>;

describe("JoinForm", () => {
  let onJoin: Mock<JoinFormProps["onJoin"]>;

  beforeEach(() => {
    onJoin = vi.fn<JoinFormProps["onJoin"]>().mockResolvedValue(undefined);
  });

  it("submits trimmed name and calls onJoin", async () => {
    const user = userEvent.setup();
    render(<JoinForm onJoin={onJoin} error={null} />);

    await user.type(screen.getByLabelText(/your name/i), "  Alice  ");
    await user.click(screen.getByRole("button", { name: /join/i }));

    expect(onJoin).toHaveBeenCalledWith("Alice", null);
  });

  it("forwards an optional email when provided", async () => {
    const user = userEvent.setup();
    render(<JoinForm onJoin={onJoin} error={null} />);

    await user.type(screen.getByLabelText(/your name/i), "Alice");
    await user.type(screen.getByLabelText(/email/i), "  alice@example.com  ");
    await user.click(screen.getByRole("button", { name: /join/i }));

    expect(onJoin).toHaveBeenCalledWith("Alice", "alice@example.com");
  });

  it("hides the email field when hideEmail is set", () => {
    render(<JoinForm onJoin={onJoin} error={null} hideEmail />);
    expect(screen.queryByLabelText(/email/i)).not.toBeInTheDocument();
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

  it("does not show the counter for short names", () => {
    render(<JoinForm onJoin={onJoin} error={null} defaultName="Alice" />);
    expect(screen.queryByText(/\/ 50/)).not.toBeInTheDocument();
  });

  it("shows the counter once the user is within 10 chars of the cap", () => {
    const name = "a".repeat(40);
    render(<JoinForm onJoin={onJoin} error={null} defaultName={name} />);
    expect(screen.getByText("40 / 50")).toBeInTheDocument();
  });

  it("caps the input at the participant-name limit via maxLength", () => {
    render(<JoinForm onJoin={onJoin} error={null} />);
    const input = screen.getByLabelText(/your name/i) as HTMLInputElement;
    expect(input.maxLength).toBe(50);
  });
});
