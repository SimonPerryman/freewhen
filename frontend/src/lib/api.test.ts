import { describe, it, expect, vi, beforeEach } from "vitest";
import { createCalendar, getCalendar, addParticipant, setAvailability, ApiError } from "./api";

const mockFetch = vi.fn();
vi.stubGlobal("fetch", mockFetch);

function jsonResponse(body: unknown, status = 200) {
  return {
    ok: status >= 200 && status < 300,
    status,
    json: () => Promise.resolve(body),
  };
}

beforeEach(() => {
  mockFetch.mockReset();
});

describe("createCalendar", () => {
  it("sends POST to /calendars with name", async () => {
    mockFetch.mockResolvedValue(
      jsonResponse({ id: "abc", name: "Trip", created_at: "2026-01-01T00:00:00Z" }, 201),
    );

    const result = await createCalendar({ name: "Trip" });

    expect(mockFetch).toHaveBeenCalledWith("http://localhost:8080/calendars", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name: "Trip" }),
    });
    expect(result).toEqual({
      id: "abc",
      name: "Trip",
      createdAt: "2026-01-01T00:00:00Z",
    });
  });

  it("throws ApiError on failure", async () => {
    mockFetch.mockResolvedValue(jsonResponse({ error: "bad request" }, 400));

    await expect(createCalendar({ name: "" })).rejects.toThrow(ApiError);
    await expect(createCalendar({ name: "" })).rejects.toMatchObject({
      status: 400,
      message: "bad request",
    });
  });
});

describe("getCalendar", () => {
  it("sends GET to /calendars/:id and converts keys", async () => {
    mockFetch.mockResolvedValue(
      jsonResponse({
        calendar: { id: "abc", name: "Trip", created_at: "2026-01-01T00:00:00Z" },
        participants: [{ id: "p1", calendar_id: "abc", name: "Alice", colour: "#ff0000" }],
        availability: [{ participant_id: "p1", calendar_id: "abc", date: "2026-03-01" }],
      }),
    );

    const result = await getCalendar("abc");

    expect(mockFetch).toHaveBeenCalledWith("http://localhost:8080/calendars/abc", {
      headers: { "Content-Type": "application/json" },
    });
    expect(result.calendar.createdAt).toBe("2026-01-01T00:00:00Z");
    expect(result.participants[0].calendarId).toBe("abc");
    expect(result.availability[0].participantId).toBe("p1");
  });

  it("throws ApiError on 404", async () => {
    mockFetch.mockResolvedValue(jsonResponse({ error: "calendar not found" }, 404));

    await expect(getCalendar("missing")).rejects.toMatchObject({
      status: 404,
      message: "calendar not found",
    });
  });
});

describe("addParticipant", () => {
  it("sends POST to /calendars/:id/participants", async () => {
    mockFetch.mockResolvedValue(
      jsonResponse({ id: "p1", calendar_id: "abc", name: "Alice", colour: "#ff0000" }, 201),
    );

    const result = await addParticipant("abc", { name: "Alice" });

    expect(mockFetch).toHaveBeenCalledWith("http://localhost:8080/calendars/abc/participants", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name: "Alice" }),
    });
    expect(result).toEqual({
      id: "p1",
      calendarId: "abc",
      name: "Alice",
      colour: "#ff0000",
    });
  });

  it("throws ApiError on 409 conflict", async () => {
    mockFetch.mockResolvedValue(jsonResponse({ error: "participant name already taken" }, 409));

    await expect(addParticipant("abc", { name: "Alice" })).rejects.toMatchObject({
      status: 409,
      message: "participant name already taken",
    });
  });
});

describe("setAvailability", () => {
  it("sends PUT to /calendars/:id/availability with snake_case body", async () => {
    mockFetch.mockResolvedValue(jsonResponse({ ok: true }));

    await setAvailability("abc", {
      participantId: "p1",
      dates: ["2026-03-01", "2026-03-02"],
    });

    expect(mockFetch).toHaveBeenCalledWith("http://localhost:8080/calendars/abc/availability", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        participant_id: "p1",
        dates: ["2026-03-01", "2026-03-02"],
      }),
    });
  });

  it("throws ApiError on 400", async () => {
    mockFetch.mockResolvedValue(jsonResponse({ error: "participant_id is required" }, 400));

    await expect(setAvailability("abc", { participantId: "", dates: [] })).rejects.toMatchObject({
      status: 400,
      message: "participant_id is required",
    });
  });
});

describe("base URL", () => {
  it("uses default localhost URL", async () => {
    mockFetch.mockResolvedValue(
      jsonResponse({ id: "abc", name: "Test", created_at: "2026-01-01T00:00:00Z" }, 201),
    );

    await createCalendar({ name: "Test" });

    expect(mockFetch.mock.calls[0][0]).toBe("http://localhost:8080/calendars");
  });
});
