import { describe, it, expect } from "vitest";
import { safeReturnPath } from "./safeReturnPath";

describe("safeReturnPath", () => {
  it.each([
    ["/", "/"],
    ["/calendars", "/calendars"],
    ["/calendars/abc?x=1#y", "/calendars/abc?x=1#y"],
    // Percent-encoded slashes stay path content on our origin.
    ["/%2F%2Fevil.com", "/%2F%2Fevil.com"],
    ["/%5Cevil.com", "/%5Cevil.com"],
  ])("allows same-origin path %j", (input, want) => {
    expect(safeReturnPath(input)).toBe(want);
  });

  it.each([
    null,
    undefined,
    "",
    "//evil.com",
    "//evil.com/path",
    "/\\evil.com",
    "/\\/evil.com",
    "\\\\evil.com",
    "/path\\evil",
    "https://evil.com",
    "http://evil.com/x",
    "javascript:alert(1)",
    "JaVaScRiPt:alert(1)",
    "data:text/html,evil",
    "evil.com",
    " /path",
    "/\t/evil.com",
    "/\n/evil.com",
    "/\r/evil.com",
    "\t//evil.com",
    "/path with space",
    "/\u0000/evil.com",
    "/\u007f",
    "/ /evil.com",
  ])("rejects %j", (input) => {
    expect(safeReturnPath(input)).toBe("/");
  });
});
