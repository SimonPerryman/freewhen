import { createRef } from "react";
import { describe, it, expect, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import TextField from "./TextField";

describe("TextField", () => {
  it("labels the input", () => {
    render(<TextField label="Name" />);
    const input = screen.getByLabelText("Name");
    expect(input.tagName).toBe("INPUT");
    expect(input).toHaveAttribute("type", "text");
    expect(input).not.toHaveAttribute("aria-invalid");
    expect(input).not.toHaveAttribute("aria-describedby");
  });

  it("keeps a visually hidden label accessible", () => {
    render(<TextField label="Passphrase" hideLabel />);
    expect(screen.getByLabelText("Passphrase")).toBeInTheDocument();
  });

  it("describes the input with its hint", () => {
    render(<TextField label="Email" hint="Only used for reminders." />);
    expect(screen.getByLabelText("Email")).toHaveAccessibleDescription("Only used for reminders.");
  });

  it("marks the input invalid and describes it with the error", () => {
    render(<TextField label="Name" hint="Keep it short." error="That name is too long." />);
    const input = screen.getByLabelText("Name");
    expect(input).toHaveAttribute("aria-invalid", "true");
    expect(input).toHaveAccessibleDescription("Keep it short. That name is too long.");
    expect(screen.getByRole("alert")).toHaveTextContent("That name is too long.");
  });

  it("merges a caller aria-describedby with its own ids", () => {
    render(
      <>
        <p id="outside">Outside help.</p>
        <TextField label="Name" aria-describedby="outside" error="Required." />
      </>,
    );
    expect(screen.getByLabelText("Name")).toHaveAccessibleDescription("Outside help. Required.");
  });

  it("uses a caller id and forwards refs and input props", () => {
    const ref = createRef<HTMLInputElement>();
    const onChange = vi.fn();
    render(<TextField ref={ref} id="my-field" label="Name" maxLength={10} onChange={onChange} />);
    const input = screen.getByLabelText("Name");
    expect(input).toHaveAttribute("id", "my-field");
    expect(input).toHaveAttribute("maxLength", "10");
    expect(ref.current).toBe(input);
    fireEvent.change(input, { target: { value: "Sam" } });
    expect(onChange).toHaveBeenCalledTimes(1);
  });
});
