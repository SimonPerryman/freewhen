import { describe, it, expect } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import PreferencePill, { type PreferencePillVoter } from "./PreferencePill";

function v(id: string, name: string, colour = "#abcdef"): PreferencePillVoter {
  return { id, name, colour };
}

describe("PreferencePill", () => {
  it("renders nothing when there are no voters", () => {
    const { container } = render(<PreferencePill voters={[]} />);
    expect(container.innerHTML).toBe("");
  });

  it("n=1 renders a static pill with the voter name and no button", () => {
    render(<PreferencePill voters={[v("a", "Sam")]} />);
    expect(screen.getByText("Sam")).toBeInTheDocument();
    expect(screen.getByText("prefers", { exact: false })).toBeInTheDocument();
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });

  it("n>1 renders a button pill that opens a popover with all voters on click", () => {
    const voters = [v("a", "Sam", "#ff0000"), v("b", "Tess", "#00ff00"), v("c", "Jo", "#0000ff")];
    render(<PreferencePill voters={voters} />);

    const trigger = screen.getByRole("button", {
      name: "3 people prefer this day. Show who.",
    });
    expect(trigger).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByRole("group")).not.toBeInTheDocument();

    fireEvent.click(trigger);

    expect(trigger).toHaveAttribute("aria-expanded", "true");
    const group = screen.getByRole("group", { name: "People who prefer this day" });
    expect(group).toBeInTheDocument();
    expect(screen.getByText("Sam")).toBeInTheDocument();
    expect(screen.getByText("Tess")).toBeInTheDocument();
    expect(screen.getByText("Jo")).toBeInTheDocument();
  });

  it("Escape closes the popover", () => {
    render(<PreferencePill voters={[v("a", "Sam"), v("b", "Tess")]} />);
    const trigger = screen.getByRole("button");
    fireEvent.click(trigger);
    expect(screen.getByRole("group")).toBeInTheDocument();

    fireEvent.keyDown(document, { key: "Escape" });
    expect(screen.queryByRole("group")).not.toBeInTheDocument();
  });

  it("clicking outside closes the popover", () => {
    render(
      <div>
        <button data-testid="outside">outside</button>
        <PreferencePill voters={[v("a", "Sam"), v("b", "Tess")]} />
      </div>,
    );
    fireEvent.click(screen.getByRole("button", { name: /people prefer/ }));
    expect(screen.getByRole("group")).toBeInTheDocument();

    fireEvent.pointerDown(screen.getByTestId("outside"));
    expect(screen.queryByRole("group")).not.toBeInTheDocument();
  });

  it("uses the participant colour for the swatch", () => {
    const { container } = render(<PreferencePill voters={[v("a", "Sam", "#ff00ff")]} />);
    const swatch = container.querySelector('[aria-hidden="true"]') as HTMLElement;
    expect(swatch).not.toBeNull();
    expect(swatch.style.backgroundColor).toBe("#ff00ff");
  });

  it("falls through to the popover path for a single long name", () => {
    const longName = "Wolfeschlegelsteinhausenbergerdorff";
    render(<PreferencePill voters={[v("a", longName)]} />);

    const trigger = screen.getByRole("button", {
      name: "1 person prefers this day. Show who.",
    });
    fireEvent.click(trigger);
    expect(screen.getByRole("group")).toBeInTheDocument();
    expect(screen.getByText(longName)).toBeInTheDocument();
  });
});
