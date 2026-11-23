import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import CalendarHeader from "./CalendarHeader";
import type { Participant } from "@/lib/types";

function participant(id: string): Participant {
  return { id, name: id, colour: "#2563eb", calendarId: "c1" } as Participant;
}

function renderHeader(participants: Participant[], responded: string[]) {
  return render(
    <CalendarHeader
      calendarName="Ski week"
      showSettings={false}
      onOpenSettings={vi.fn()}
      streamStatus="open"
      renameDraft={null}
      renameInputRef={{ current: null }}
      onRenameDraftChange={vi.fn()}
      onRenameCommit={vi.fn()}
      onRenameCancel={vi.fn()}
      participants={participants}
      respondedIds={new Set(responded)}
    />,
  );
}

describe("CalendarHeader", () => {
  it("shows a single N of M responded readout", () => {
    renderHeader([participant("a"), participant("b")], ["b", "gone"]);
    expect(screen.getByText("1 of 2 responded")).toBeInTheDocument();
    expect(screen.queryByText(/\bpeople\b/)).not.toBeInTheDocument();
  });

  it("hides the readout when nobody has joined", () => {
    renderHeader([], []);
    expect(screen.queryByText(/responded/)).not.toBeInTheDocument();
  });
});
