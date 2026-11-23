import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import CharCounter from "./CharCounter";

describe("CharCounter", () => {
  it("shows nothing visible until 20 characters are left", () => {
    render(<CharCounter count={119} max={140} />);
    expect(screen.queryByText(/left/)).toBeNull();
  });

  it("shows '16 left' in the last 20", () => {
    render(<CharCounter count={124} max={140} />);
    expect(screen.getByText("16 left")).toBeInTheDocument();
  });

  it("shows text and an icon at the limit", () => {
    const { container } = render(<CharCounter count={140} max={140} />);
    expect(screen.getByText("Limit reached")).toBeInTheDocument();
    expect(container.querySelector("svg")).not.toBeNull();
  });

  it("announces at 20, 10 and 0 left only", () => {
    const { rerender } = render(<CharCounter count={100} max={140} />);
    const region = screen.getByRole("status");
    expect(region).toHaveTextContent("");
    rerender(<CharCounter count={120} max={140} />);
    expect(region).toHaveTextContent("20 characters left");
    rerender(<CharCounter count={125} max={140} />);
    expect(region).toHaveTextContent("20 characters left");
    rerender(<CharCounter count={130} max={140} />);
    expect(region).toHaveTextContent("10 characters left");
    rerender(<CharCounter count={140} max={140} />);
    expect(region).toHaveTextContent("Character limit reached");
  });
});
