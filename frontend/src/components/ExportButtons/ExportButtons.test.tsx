import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import ExportButtons from "./ExportButtons";

describe("ExportButtons", () => {
  it("links to the backend export endpoints", () => {
    render(<ExportButtons calendarId="abc123" />);

    const google = screen.getByText("Add to Google Calendar") as HTMLAnchorElement;
    expect(google.href).toContain("/calendars/abc123/export/google");

    const ics = screen.getByText("Download .ics") as HTMLAnchorElement;
    expect(ics.href).toContain("/calendars/abc123/export.ics");
    expect(ics.hasAttribute("download")).toBe(true);
  });
});
