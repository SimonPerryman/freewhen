import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import type * as DateLocale from "./dateLocale";

// vitest.setup.ts defaults the hook to en-GB for component tests; test the real one.
const { DateLocaleProvider, useDateLocale } =
  await vi.importActual<typeof DateLocale>("./dateLocale");
import { formatShortDay } from "./dates";

function Day() {
  return <span>{formatShortDay("2026-12-03", useDateLocale())}</span>;
}

describe("DateLocaleProvider", () => {
  it("gives components the provided locale", () => {
    render(
      <DateLocaleProvider locale="en-US">
        <Day />
      </DateLocaleProvider>,
    );
    expect(screen.getByText("Thu, Dec 3")).toBeInTheDocument();
  });

  it("is undefined (the runtime locale) without a provider", () => {
    let seen: string | undefined = "unset";
    function Probe() {
      seen = useDateLocale();
      return null;
    }
    render(<Probe />);
    expect(seen).toBeUndefined();
  });
});
