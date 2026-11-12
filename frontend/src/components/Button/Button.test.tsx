import { readFileSync } from "node:fs";
import { join } from "node:path";
import { createRef } from "react";
import { describe, it, expect, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import Button, { buttonClassName } from "./Button";

describe("Button", () => {
  it("renders a type=button by default so it never submits a form by accident", () => {
    const onSubmit = vi.fn((e: React.FormEvent) => e.preventDefault());
    render(
      <form onSubmit={onSubmit}>
        <Button>Cancel</Button>
      </form>,
    );
    const button = screen.getByRole("button", { name: "Cancel" });
    expect(button).toHaveAttribute("type", "button");
    fireEvent.click(button);
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("submits when type=submit", () => {
    const onSubmit = vi.fn((e: React.FormEvent) => e.preventDefault());
    render(
      <form onSubmit={onSubmit}>
        <Button type="submit">Save</Button>
      </form>,
    );
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(onSubmit).toHaveBeenCalledTimes(1);
  });

  it("applies variant, size and caller classes", () => {
    render(
      <Button variant="danger" size="sm" className="extra">
        Delete
      </Button>,
    );
    const button = screen.getByRole("button", { name: "Delete" });
    expect(button.className).toContain("danger");
    expect(button.className).toContain("sm");
    expect(button.className).toContain("extra");
  });

  it("defaults to the secondary md style", () => {
    expect(buttonClassName()).toMatch(/secondary/);
    expect(buttonClassName()).toMatch(/md/);
    expect(buttonClassName({ variant: "primary", fullWidth: true })).toMatch(/fullWidth/);
  });

  it("does not fire onClick when disabled", () => {
    const onClick = vi.fn();
    render(
      <Button disabled onClick={onClick}>
        Save
      </Button>,
    );
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(onClick).not.toHaveBeenCalled();
  });

  it("is disabled and aria-busy while loading", () => {
    const onClick = vi.fn();
    render(
      <Button loading onClick={onClick}>
        Saving…
      </Button>,
    );
    const button = screen.getByRole("button", { name: "Saving…" });
    expect(button).toBeDisabled();
    expect(button).toHaveAttribute("aria-busy", "true");
    fireEvent.click(button);
    expect(onClick).not.toHaveBeenCalled();
  });

  it("omits aria-busy when not loading", () => {
    render(<Button>Save</Button>);
    expect(screen.getByRole("button", { name: "Save" })).not.toHaveAttribute("aria-busy");
  });

  it("forwards refs and arbitrary button props", () => {
    const ref = createRef<HTMLButtonElement>();
    render(
      <Button ref={ref} aria-label="Close" data-testid="close">
        ×
      </Button>,
    );
    expect(ref.current).toBe(screen.getByRole("button", { name: "Close" }));
    expect(ref.current).toHaveAttribute("data-testid", "close");
  });
});

// jsdom doesn't apply CSS Modules, so the disabled look is checked at the
// source: explicit tokens (not an opacity fade, which left a bright accent
// reading as enabled in dark mode), defined for both themes, with a label
// that stays legible against the fill.
describe("Button disabled styling", () => {
  const buttonCss = readFileSync(
    join(process.cwd(), "src/components/Button/Button.module.css"),
    "utf8",
  );
  const globalsCss = readFileSync(join(process.cwd(), "src/app/globals.css"), "utf8");

  function block(css: string, selector: string): string {
    const start = css.indexOf(`${selector} {`);
    expect(start, `${selector} block`).toBeGreaterThanOrEqual(0);
    return css.slice(start, css.indexOf("}", start));
  }

  function hex(theme: string, name: string): string {
    const m = new RegExp(`--${name}:\\s*(#[0-9a-f]{6})`, "i").exec(theme);
    expect(m, `--${name}`).not.toBeNull();
    return m![1];
  }

  function luminance(color: string): number {
    const [r, g, b] = [1, 3, 5].map((i) => {
      const c = parseInt(color.slice(i, i + 2), 16) / 255;
      return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
    });
    return 0.2126 * r + 0.7152 * g + 0.0722 * b;
  }

  function contrast(a: string, b: string): number {
    const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x);
    return (hi + 0.05) / (lo + 0.05);
  }

  it("uses explicit disabled tokens instead of fading the variant", () => {
    const rule = block(buttonCss, ".button:disabled");
    expect(rule).toContain("var(--disabled-bg)");
    expect(rule).toContain("var(--disabled-text)");
    expect(rule).toContain("var(--disabled-border)");
    expect(rule).not.toMatch(/opacity/);
  });

  it.each([
    ["light", ":root"],
    ["dark", '[data-color-scheme="dark"]'],
  ])("defines legible disabled tokens in %s mode", (_name, selector) => {
    const theme = block(globalsCss, selector);
    const text = hex(theme, "disabled-text");
    const bg = hex(theme, "disabled-bg");
    expect(contrast(text, bg)).toBeGreaterThanOrEqual(3);
  });
});
