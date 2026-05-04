import { describe, it, expect, vi, beforeEach } from "vitest";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { ToastProvider } from "@/lib/toast";
import WhosMissingPanel from "./WhosMissingPanel";
import * as api from "@/lib/api";

vi.mock("@/lib/api", async () => {
  const actual = await vi.importActual<typeof import("@/lib/api")>("@/lib/api");
  return {
    ...actual,
    getMissing: vi.fn(),
    addInvites: vi.fn(),
    deleteInvite: vi.fn(),
    renameInvite: vi.fn(),
    matchInvite: vi.fn(),
  };
});

const mockedApi = api as unknown as {
  getMissing: ReturnType<typeof vi.fn>;
  addInvites: ReturnType<typeof vi.fn>;
  deleteInvite: ReturnType<typeof vi.fn>;
  renameInvite: ReturnType<typeof vi.fn>;
  matchInvite: ReturnType<typeof vi.fn>;
};

function renderPanel(participants: import("@/lib/types").Participant[] = []) {
  return render(
    <ToastProvider>
      <WhosMissingPanel calendarId="c1" participants={participants} />
    </ToastProvider>,
  );
}

beforeEach(() => {
  mockedApi.getMissing.mockReset();
  mockedApi.addInvites.mockReset();
  mockedApi.deleteInvite.mockReset();
  mockedApi.renameInvite.mockReset();
  mockedApi.matchInvite.mockReset();
  window.localStorage.clear();
});

describe("WhosMissingPanel", () => {
  it("is collapsed by default and loads when opened", async () => {
    mockedApi.getMissing.mockResolvedValue({
      invites: [
        { id: "i1", calendarId: "c1", name: "Alice", matchedParticipantId: null, createdAt: "" },
      ],
      notJoined: [{ inviteId: "i1", name: "Alice" }],
      joinedNotResponded: [],
    });

    renderPanel();

    // Closed — fetch not called yet.
    expect(mockedApi.getMissing).not.toHaveBeenCalled();
    expect(screen.queryByText("Not joined")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /Who's missing\?/i }));

    await waitFor(() => expect(mockedApi.getMissing).toHaveBeenCalled());
    expect(screen.getByText("Not joined")).toBeInTheDocument();
    expect(screen.getByText("Alice")).toBeInTheDocument();
  });

  it("renders 'No-one's missing' when buckets are empty but invites exist", async () => {
    mockedApi.getMissing.mockResolvedValue({
      invites: [
        { id: "i1", calendarId: "c1", name: "Alice", matchedParticipantId: "p1", createdAt: "" },
      ],
      notJoined: [],
      joinedNotResponded: [],
    });

    renderPanel();
    fireEvent.click(screen.getByRole("button", { name: /Who's missing\?/i }));
    await waitFor(() => expect(mockedApi.getMissing).toHaveBeenCalled());
    expect(screen.getByText(/No-one's missing/)).toBeInTheDocument();
  });

  it("copies name to clipboard when chip is tapped", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", {
      value: { writeText },
      configurable: true,
    });

    mockedApi.getMissing.mockResolvedValue({
      invites: [
        { id: "i1", calendarId: "c1", name: "Bob", matchedParticipantId: null, createdAt: "" },
      ],
      notJoined: [{ inviteId: "i1", name: "Bob" }],
      joinedNotResponded: [],
    });

    renderPanel();
    fireEvent.click(screen.getByRole("button", { name: /Who's missing\?/i }));
    await waitFor(() => expect(mockedApi.getMissing).toHaveBeenCalled());

    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: /Copy Bob/ }));
    });

    expect(writeText).toHaveBeenCalledWith("Bob");
  });

  it("adds invites via API and reports skipped/invalid inline", async () => {
    mockedApi.getMissing.mockResolvedValue({
      invites: [],
      notJoined: [],
      joinedNotResponded: [],
    });
    mockedApi.addInvites.mockResolvedValue({
      added: [
        { id: "i1", calendarId: "c1", name: "Alice", matchedParticipantId: null, createdAt: "" },
      ],
      skipped: [{ name: "bob", reason: "duplicate" }],
      invalid: [],
    });

    renderPanel();
    fireEvent.click(screen.getByRole("button", { name: /Who's missing\?/i }));
    await waitFor(() => expect(mockedApi.getMissing).toHaveBeenCalled());

    fireEvent.change(screen.getByPlaceholderText("Add an invite"), {
      target: { value: "Alice, bob" },
    });
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Add" }));
    });

    expect(mockedApi.addInvites).toHaveBeenCalledWith("c1", ["Alice, bob"]);
    expect(screen.getByText(/1 already on the list/)).toBeInTheDocument();
  });
});
