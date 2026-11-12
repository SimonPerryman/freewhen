import { describe, it, expect, vi, beforeEach } from "vitest";
import {
  createCalendar,
  duplicateCalendar,
  getCalendar,
  addParticipant,
  setAvailability,
  finaliseCalendar,
  unfinaliseCalendar,
  renameParticipant,
  verifyMagicLink,
  updateParticipantEmail,
  importAvailabilityDays,
  fetchImportSuggestions,
  claimParticipant,
  getMe,
  ApiError,
} from "./api";
import { getParticipantToken, setParticipantToken } from "./participantToken";

const mockFetch = vi.fn();
vi.stubGlobal("fetch", mockFetch);

function jsonResponse(body: unknown, status = 200) {
  return {
    ok: status >= 200 && status < 300,
    status,
    headers: new Headers({ "X-Request-ID": "test-req-id" }),
    json: () => Promise.resolve(body),
  };
}

beforeEach(() => {
  mockFetch.mockReset();
  localStorage.clear();
});

describe("participant token", () => {
  function headersOf(call: number): Record<string, string> {
    return (mockFetch.mock.calls[call][1] as { headers: Record<string, string> }).headers;
  }

  it("stores the token returned on anonymous join, keyed by calendar", async () => {
    mockFetch.mockResolvedValue({
      ...jsonResponse({ id: "p1", calendar_id: "abc", name: "Alice", colour: "#ff0000" }, 201),
      headers: new Headers({ "X-Participant-Token": "tok-abc" }),
    });

    await addParticipant("abc", { name: "Alice" });

    expect(getParticipantToken("abc")).toBe("tok-abc");
    expect(getParticipantToken("other")).toBeNull();
  });

  it("sends the stored token on participant-scoped writes for that calendar only", async () => {
    setParticipantToken("abc", "tok-abc");
    mockFetch.mockResolvedValue(jsonResponse({ ok: true }));

    await setAvailability("abc", { participantId: "p1", dates: [] });
    await renameParticipant("abc", "p1", "Alicia");
    await updateParticipantEmail("abc", "p1", null);
    await importAvailabilityDays("abc", "p1", []);
    await setAvailability("xyz", { participantId: "p9", dates: [] });

    for (let i = 0; i < 4; i++) {
      expect(headersOf(i)["X-Participant-Token"]).toBe("tok-abc");
    }
    expect(headersOf(4)["X-Participant-Token"]).toBeUndefined();
  });

  it("sends the token (and calendar access) on claim", async () => {
    setParticipantToken("abc", "tok-abc");
    mockFetch.mockResolvedValue(
      jsonResponse({ id: "p1", calendar_id: "abc", name: "Alice", colour: "#ff0000" }),
    );

    await claimParticipant("abc", "p1");

    expect(headersOf(0)["X-Participant-Token"]).toBe("tok-abc");
  });

  it("replaces an older token when a new one is issued", async () => {
    setParticipantToken("abc", "old");
    mockFetch.mockResolvedValue({
      ...jsonResponse({ id: "p2", calendar_id: "abc", name: "Bob", colour: "#ff0000" }, 201),
      headers: new Headers({ "X-Participant-Token": "new" }),
    });

    await addParticipant("abc", { name: "Bob" });

    expect(getParticipantToken("abc")).toBe("new");
  });
});

describe("createCalendar", () => {
  it("sends POST to /calendars with name", async () => {
    mockFetch.mockResolvedValue(
      jsonResponse({ id: "abc", name: "Trip", created_at: "2026-01-01T00:00:00Z" }, 201),
    );

    const result = await createCalendar({ name: "Trip" });

    expect(mockFetch).toHaveBeenCalledWith("http://localhost:8080/calendars", {
      method: "POST",
      credentials: "include",
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

describe("getMe", () => {
  it("resolves to null when signed out (200 null)", async () => {
    mockFetch.mockResolvedValue(jsonResponse(null));
    await expect(getMe()).resolves.toBeNull();
  });

  it("resolves to the camel-cased user when signed in", async () => {
    mockFetch.mockResolvedValue(
      jsonResponse({ id: "u1", email: "a@example.com", name: "A", is_admin: false }),
    );
    await expect(getMe()).resolves.toEqual({
      id: "u1",
      email: "a@example.com",
      name: "A",
      isAdmin: false,
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
      credentials: "include",
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
      credentials: "include",
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
      credentials: "include",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        participant_id: "p1",
        dates: ["2026-03-01", "2026-03-02"],
      }),
    });
  });

  it("passes keepalive through when asked, with headers and credentials intact", async () => {
    setParticipantToken("abc", "tok-abc");
    mockFetch.mockResolvedValue(jsonResponse({ ok: true }));

    await setAvailability("abc", { participantId: "p1", dates: [] }, { keepalive: true });

    const init = mockFetch.mock.calls[0][1] as RequestInit & { headers: Record<string, string> };
    expect(init.keepalive).toBe(true);
    expect(init.credentials).toBe("include");
    expect(init.headers["Content-Type"]).toBe("application/json");
    expect(init.headers["X-Participant-Token"]).toBe("tok-abc");
  });

  it("drops keepalive for a body over the browser's keepalive budget", async () => {
    mockFetch.mockResolvedValue(jsonResponse({ ok: true }));
    const notes = { "2026-03-01": "x".repeat(60 * 1024) };

    await setAvailability("abc", { participantId: "p1", dates: [], notes }, { keepalive: true });

    const init = mockFetch.mock.calls[0][1] as RequestInit;
    expect(init.keepalive).toBeUndefined();
  });

  it("throws ApiError on 400", async () => {
    mockFetch.mockResolvedValue(jsonResponse({ error: "participant_id is required" }, 400));

    await expect(setAvailability("abc", { participantId: "", dates: [] })).rejects.toMatchObject({
      status: 400,
      message: "participant_id is required",
    });
  });
});

describe("fetchImportSuggestions", () => {
  it("returns null when nothing is pending (200 pending:false)", async () => {
    mockFetch.mockResolvedValue(jsonResponse({ pending: false, free: [], busy: [] }));
    await expect(fetchImportSuggestions("abc")).resolves.toBeNull();
  });

  it("returns null on a 404 from an older backend", async () => {
    mockFetch.mockResolvedValue(jsonResponse({ error: "no pending import" }, 404));
    await expect(fetchImportSuggestions("abc")).resolves.toBeNull();
  });

  it("returns the suggestions when an import is pending", async () => {
    mockFetch.mockResolvedValue(
      jsonResponse({ pending: true, free: ["2026-05-01"], busy: ["2026-05-02"] }),
    );
    await expect(fetchImportSuggestions("abc")).resolves.toEqual({
      pending: true,
      free: ["2026-05-01"],
      busy: ["2026-05-02"],
    });
  });

  it("still returns suggestions from an older backend without the pending field", async () => {
    mockFetch.mockResolvedValue(jsonResponse({ free: ["2026-05-01"], busy: [] }));
    await expect(fetchImportSuggestions("abc")).resolves.toEqual({
      free: ["2026-05-01"],
      busy: [],
    });
  });

  it("rethrows other errors", async () => {
    mockFetch.mockResolvedValue(jsonResponse({ error: "boom" }, 500));
    await expect(fetchImportSuggestions("abc")).rejects.toMatchObject({ status: 500 });
  });
});

describe("finaliseCalendar", () => {
  it("sends POST to /calendars/:id/finalise with date and decodes camelCase", async () => {
    mockFetch.mockResolvedValue(
      jsonResponse({
        id: "abc",
        name: "Trip",
        owner_id: "u1",
        created_at: "2026-01-01T00:00:00Z",
        finalised_date: "2026-05-02",
        finalised_at: "2026-04-19T10:00:00Z",
      }),
    );

    const result = await finaliseCalendar("abc", "2026-05-02");

    expect(mockFetch).toHaveBeenCalledWith("http://localhost:8080/calendars/abc/finalise", {
      method: "POST",
      credentials: "include",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ date: "2026-05-02" }),
    });
    expect(result.finalisedDate).toBe("2026-05-02");
    expect(result.finalisedAt).toBe("2026-04-19T10:00:00Z");
  });

  it("throws ApiError on 403", async () => {
    mockFetch.mockResolvedValue(jsonResponse({ error: "only the owner can finalise" }, 403));
    await expect(finaliseCalendar("abc", "2026-05-02")).rejects.toMatchObject({
      status: 403,
    });
  });
});

describe("unfinaliseCalendar", () => {
  it("sends DELETE to /calendars/:id/finalise", async () => {
    mockFetch.mockResolvedValue(
      jsonResponse({
        id: "abc",
        name: "Trip",
        owner_id: "u1",
        created_at: "2026-01-01T00:00:00Z",
      }),
    );

    const result = await unfinaliseCalendar("abc");

    expect(mockFetch).toHaveBeenCalledWith("http://localhost:8080/calendars/abc/finalise", {
      method: "DELETE",
      credentials: "include",
      headers: { "Content-Type": "application/json" },
    });
    expect(result.finalisedDate).toBeUndefined();
  });
});

describe("error body code field", () => {
  it("exposes the server error code on ApiError", async () => {
    mockFetch.mockResolvedValue(
      jsonResponse({ error: "this calendar is full", code: "calendar_full" }, 409),
    );

    try {
      await addParticipant("abc", { name: "Alice" });
      expect.fail("expected ApiError");
    } catch (err) {
      expect(err).toBeInstanceOf(ApiError);
      if (err instanceof ApiError) {
        expect(err.code).toBe("calendar_full");
        expect(err.status).toBe(409);
      }
    }
  });

  it("leaves code as null when the server omits it", async () => {
    mockFetch.mockResolvedValue(jsonResponse({ error: "legacy shape" }, 500));

    try {
      await addParticipant("abc", { name: "Alice" });
      expect.fail("expected ApiError");
    } catch (err) {
      if (err instanceof ApiError) {
        expect(err.code).toBeNull();
      }
    }
  });
});

describe("renameParticipant", () => {
  it("sends PATCH with name and caller participant_id", async () => {
    mockFetch.mockResolvedValue(
      jsonResponse({ id: "p1", calendar_id: "abc", name: "Alicia", colour: "#ff0000" }),
    );

    const result = await renameParticipant("abc", "p1", "Alicia");

    // No participant_id in the body: identity comes from the session or the
    // X-Participant-Token header, never a caller-supplied id.
    expect(mockFetch).toHaveBeenCalledWith("http://localhost:8080/calendars/abc/participants/p1", {
      method: "PATCH",
      credentials: "include",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name: "Alicia" }),
    });
    expect(result.name).toBe("Alicia");
  });

  it("surfaces duplicate_name as ApiError with code", async () => {
    mockFetch.mockResolvedValue(
      jsonResponse({ error: "participant name already taken", code: "duplicate_name" }, 409),
    );

    try {
      await renameParticipant("abc", "p1", "Alicia");
      expect.fail("expected ApiError");
    } catch (err) {
      expect(err).toBeInstanceOf(ApiError);
      if (err instanceof ApiError) {
        expect(err.status).toBe(409);
        expect(err.code).toBe("duplicate_name");
      }
    }
  });
});

describe("duplicateCalendar", () => {
  it("sends POST to /calendars/:id/duplicate with snake_cased body", async () => {
    mockFetch.mockResolvedValue(
      jsonResponse(
        { id: "new123", name: "Tennis (copy)", created_at: "2026-04-21T00:00:00Z" },
        201,
      ),
    );

    const result = await duplicateCalendar("src1", {
      name: "Tennis May",
      includeParticipants: false,
    });

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/calendars/src1/duplicate",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ name: "Tennis May", include_participants: false }),
      }),
    );
    expect(result.id).toBe("new123");
  });

  it("defaults to empty body when no options provided", async () => {
    mockFetch.mockResolvedValue(
      jsonResponse({ id: "new", name: "x", created_at: "2026-04-21T00:00:00Z" }, 201),
    );
    await duplicateCalendar("src1");
    expect(mockFetch.mock.calls[0][1]).toMatchObject({ body: "{}" });
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

describe("verifyMagicLink", () => {
  // The backend sends `return_url`; request() camel-cases response keys, so
  // callers read `returnUrl`. Pin that so a refactor of the key conversion
  // doesn't silently drop users on "/" after magic-link sign-in.
  it("maps the backend return_url key to returnUrl", async () => {
    mockFetch.mockResolvedValueOnce(jsonResponse({ return_url: "/calendars/abc" }));
    const result = await verifyMagicLink("tok");
    expect(result.returnUrl).toBe("/calendars/abc");
  });
});
