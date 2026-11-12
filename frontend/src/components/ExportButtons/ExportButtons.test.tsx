import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import ExportButtons from "./ExportButtons";

const getCalendarAccessToken = vi.fn<(id: string) => string | null>();
vi.mock("@/lib/calendarAccess", () => ({
  getCalendarAccessToken: (id: string) => getCalendarAccessToken(id),
}));

describe("ExportButtons", () => {
  beforeEach(() => {
    getCalendarAccessToken.mockReset();
    getCalendarAccessToken.mockReturnValue(null);
  });

  it("links to the backend export endpoints", () => {
    render(<ExportButtons calendarId="abc123" />);

    const google = screen.getByText("Add to Google Calendar") as HTMLAnchorElement;
    expect(google.href).toContain("/calendars/abc123/export/google");
    expect(google.href).not.toContain("access=");

    const ics = screen.getByText("Download .ics") as HTMLAnchorElement;
    expect(ics.href).toContain("/calendars/abc123/export.ics");
    expect(ics.href).not.toContain("access=");
    expect(ics.hasAttribute("download")).toBe(true);
  });

  it("appends the stored calendar access token for gated calendars", () => {
    getCalendarAccessToken.mockReturnValue("tok/+=");
    render(<ExportButtons calendarId="abc123" />);

    expect(getCalendarAccessToken).toHaveBeenCalledWith("abc123");
    const google = screen.getByText("Add to Google Calendar") as HTMLAnchorElement;
    expect(google.href).toContain("/calendars/abc123/export/google?access=tok%2F%2B%3D");

    const ics = screen.getByText("Download .ics") as HTMLAnchorElement;
    expect(ics.href).toContain("/calendars/abc123/export.ics?access=tok%2F%2B%3D");
  });
});
