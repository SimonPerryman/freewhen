import { beforeEach, describe, expect, it } from "vitest";
import { MAX_PARTICIPANT_NAME_LENGTH } from "./limits";
import { localToday } from "./localDate";
import {
  readLastName,
  readStoredParticipant,
  writeLastName,
  writeStoredParticipant,
} from "./participantStorage";

beforeEach(() => localStorage.clear());

describe("participantStorage", () => {
  it("round-trips the per-calendar participant record", () => {
    expect(readStoredParticipant("c1")).toBeNull();
    writeStoredParticipant("c1", { participantId: "p1", name: "Alice", colour: "#f00" });
    expect(localStorage.getItem("freewhen:c1")).toBe(
      JSON.stringify({ participantId: "p1", name: "Alice", colour: "#f00" }),
    );
    expect(readStoredParticipant("c1")).toEqual({
      participantId: "p1",
      name: "Alice",
      colour: "#f00",
    });
    expect(readStoredParticipant("c2")).toBeNull();
  });

  it("treats a corrupt record as absent", () => {
    localStorage.setItem("freewhen:c1", "{not json");
    expect(readStoredParticipant("c1")).toBeNull();
  });

  it("round-trips and truncates the last-used name", () => {
    expect(readLastName()).toBe("");
    writeLastName("Sam");
    expect(localStorage.getItem("freewhen:lastName")).toBe("Sam");
    expect(readLastName()).toBe("Sam");
    writeLastName("x".repeat(MAX_PARTICIPANT_NAME_LENGTH + 5));
    expect(readLastName()).toHaveLength(MAX_PARTICIPANT_NAME_LENGTH);
  });
});

describe("localToday", () => {
  it("formats the local calendar date", () => {
    expect(localToday(new Date(2026, 0, 5, 23, 59))).toBe("2026-01-05");
    expect(localToday(new Date(2026, 11, 31, 0, 0))).toBe("2026-12-31");
  });
});
