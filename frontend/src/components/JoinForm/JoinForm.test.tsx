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
    await user.click(screen.getByRole("button", { name: /get notified/i }));
    await user.type(screen.getByLabelText(/email/i), "  alice@example.com  ");
    await user.click(screen.getByRole("button", { name: /join/i }));

    expect(onJoin).toHaveBeenCalledWith("Alice", "alice@example.com");
  });

  it("puts every field before the submit button (DOM and tab order)", async () => {
    const user = userEvent.setup();
    render(<JoinForm onJoin={onJoin} error={null} defaultName="Alice" />);
    const name = screen.getByLabelText(/your name/i);
    await user.click(screen.getByRole("button", { name: /get notified/i }));
    const email = screen.getByLabelText(/email/i);
    const submit = screen.getByRole("button", { name: /join/i });

    const order = Array.from((name as HTMLInputElement).form!.elements);
    expect(order).toEqual([name, email, submit]);

    await user.click(name);
    await user.tab();
    expect(email).toHaveFocus();
    await user.tab();
    expect(submit).toHaveFocus();
  });

  it("hides the email field when hideEmail is set", () => {
    render(<JoinForm onJoin={onJoin} error={null} hideEmail />);
    expect(screen.queryByLabelText(/email/i)).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /get notified/i })).not.toBeInTheDocument();
  });

  it("shows only the name field until the notify link is tapped", async () => {
    const user = userEvent.setup();
    render(<JoinForm onJoin={onJoin} error={null} />);
    expect(screen.queryByLabelText(/email/i)).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: /get notified when a date's picked/i }));
    expect(screen.getByLabelText(/email/i)).toBeInTheDocument();
  });

  it("keeps Join enabled and validates on submit", async () => {
    const user = userEvent.setup();
    render(<JoinForm onJoin={onJoin} error={null} />);
    const join = screen.getByRole("button", { name: /join/i });
    expect(join).toBeEnabled();
    await user.click(join);
    expect(onJoin).not.toHaveBeenCalled();
    expect(screen.getByRole("alert")).toHaveTextContent(/enter your name/i);
    await user.type(screen.getByLabelText(/your name/i), "A");
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
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
    expect(screen.queryByText(/left/)).not.toBeInTheDocument();
  });

  it("shows the counter once the user is within 20 chars of the cap", () => {
    const name = "a".repeat(30);
    render(<JoinForm onJoin={onJoin} error={null} defaultName={name} />);
    expect(screen.getByText("20 left")).toBeInTheDocument();
  });

  it("caps the input at the participant-name limit via maxLength", () => {
    render(<JoinForm onJoin={onJoin} error={null} />);
    const input = screen.getByLabelText(/your name/i) as HTMLInputElement;
    expect(input.maxLength).toBe(50);
  });
});
