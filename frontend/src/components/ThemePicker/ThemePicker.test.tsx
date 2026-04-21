import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import ThemePicker from "./ThemePicker";

describe("ThemePicker", () => {
  it("marks the active theme as checked", () => {
    render(<ThemePicker currentTheme="ocean" onChange={() => {}} />);
    const ocean = screen.getByRole("radio", { name: "Ocean" });
    expect(ocean.getAttribute("aria-checked")).toBe("true");
    const peach = screen.getByRole("radio", { name: "Peach" });
    expect(peach.getAttribute("aria-checked")).toBe("false");
  });

  it("calls onChange when a different swatch is clicked", async () => {
    const onChange = vi.fn(() => Promise.resolve());
    render(<ThemePicker currentTheme="peach" onChange={onChange} />);
    fireEvent.click(screen.getByRole("radio", { name: "Forest" }));
    await waitFor(() => expect(onChange).toHaveBeenCalledWith("forest"));
  });

  it("ignores clicks on the already-active theme", () => {
    const onChange = vi.fn();
    render(<ThemePicker currentTheme="peach" onChange={onChange} />);
    fireEvent.click(screen.getByRole("radio", { name: "Peach" }));
    expect(onChange).not.toHaveBeenCalled();
  });

  it("falls back to the default theme when given an unknown slug", () => {
    render(<ThemePicker currentTheme={undefined} onChange={() => {}} />);
    const peach = screen.getByRole("radio", { name: "Peach" });
    expect(peach.getAttribute("aria-checked")).toBe("true");
  });
});
