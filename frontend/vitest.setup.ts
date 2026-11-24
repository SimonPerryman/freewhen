import "@testing-library/jest-dom/vitest";
import { afterEach, vi } from "vitest";
import { cleanup } from "@testing-library/react";

afterEach(() => {
  cleanup();
});

// Components format dates in the viewer's locale (lib/dates.ts). Tests assert
// en-GB strings ("Thu 3 Dec"), so default the date locale to en-GB instead of
// the machine's: Linux CI runners default to en-US. A DateLocaleProvider in a
// test still wins.
vi.mock("@/lib/dateLocale", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/dateLocale")>();
  return { ...actual, useDateLocale: () => actual.useDateLocale() ?? "en-GB" };
});
