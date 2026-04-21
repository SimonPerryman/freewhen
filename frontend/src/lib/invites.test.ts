import { describe, it, expect, vi, beforeEach } from "vitest";
import { addInvites, getMissing, matchInvite } from "./api";

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
});

describe("getMissing", () => {
  it("camelCases snake keys for invites, not_joined, joined_not_responded", async () => {
    mockFetch.mockResolvedValue(
      jsonResponse({
        invites: [
          {
            id: "i1",
            calendar_id: "c1",
            name: "Alice",
            matched_participant_id: "p1",
            created_at: "2026-04-21T00:00:00Z",
          },
          {
            id: "i2",
            calendar_id: "c1",
            name: "Bob",
            matched_participant_id: null,
            created_at: "2026-04-21T00:00:01Z",
          },
        ],
        not_joined: [{ invite_id: "i2", name: "Bob" }],
        joined_not_responded: [{ participant_id: "p2", name: "Charlie" }],
      }),
    );

    const view = await getMissing("c1");

    expect(view.invites).toHaveLength(2);
    expect(view.invites[0].matchedParticipantId).toBe("p1");
    expect(view.notJoined[0]).toEqual({ inviteId: "i2", name: "Bob" });
    expect(view.joinedNotResponded[0]).toEqual({ participantId: "p2", name: "Charlie" });
  });
});

describe("addInvites", () => {
  it("POSTs names array and passes through added/skipped/invalid", async () => {
    mockFetch.mockResolvedValue(
      jsonResponse(
        {
          added: [
            {
              id: "i1",
              calendar_id: "c1",
              name: "Alice",
              matched_participant_id: null,
              created_at: "2026-04-21T00:00:00Z",
            },
          ],
          skipped: [{ name: "bob", reason: "duplicate" }],
          invalid: [{ name: "x", reason: "name_too_long" }],
        },
        201,
      ),
    );

    const res = await addInvites("c1", ["Alice", "bob", "x"]);

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/calendars/c1/invites",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ names: ["Alice", "bob", "x"] }),
      }),
    );
    expect(res.added).toHaveLength(1);
    expect(res.added[0].matchedParticipantId).toBeNull();
    expect(res.skipped[0].name).toBe("bob");
    expect(res.invalid[0].reason).toBe("name_too_long");
  });
});

describe("matchInvite", () => {
  it("sends participant_id (string) for link", async () => {
    mockFetch.mockResolvedValue(
      jsonResponse({
        id: "i1",
        calendar_id: "c1",
        name: "Alice",
        matched_participant_id: "p1",
        created_at: "2026-04-21T00:00:00Z",
      }),
    );

    const res = await matchInvite("c1", "i1", "p1");

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/calendars/c1/invites/i1/match",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ participant_id: "p1" }),
      }),
    );
    expect(res.matchedParticipantId).toBe("p1");
  });

  it("sends participant_id: null for unlink", async () => {
    mockFetch.mockResolvedValue(
      jsonResponse({
        id: "i1",
        calendar_id: "c1",
        name: "Alice",
        matched_participant_id: null,
        created_at: "2026-04-21T00:00:00Z",
      }),
    );

    await matchInvite("c1", "i1", null);

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/calendars/c1/invites/i1/match",
      expect.objectContaining({
        body: JSON.stringify({ participant_id: null }),
      }),
    );
  });
});
