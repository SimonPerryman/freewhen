import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import DayDetailSheet from "./DayDetailSheet";
import type { Availability, Participant } from "@/lib/types";

function makeParticipant(id: string, name: string, colour = "#6366f1"): Participant {
  return { id, calendarId: "cal-1", name, colour };
}

function makeAvailability(participantId: string, date: string, note?: string): Availability {
  return {
    participantId,
    calendarId: "cal-1",
    date,
    ...(note ? { note } : {}),
  };
}

describe("DayDetailSheet", () => {
  const date = "2026-05-01";

  it("renders the list of free participants with their notes", () => {
    const me = makeParticipant("me", "Me");
    const other = makeParticipant("other", "Sarah", "#f59e0b");
    render(
      <DayDetailSheet
        date={date}
        participants={[me, other]}
        availability={[
          makeAvailability("me", date),
          makeAvailability("other", date, "only after 6pm"),
        ]}
        currentParticipantId="me"
        currentNote=""
        isSelected={true}
        finalised={false}
        onSaveNote={() => {}}
        onRemoveAvailability={() => {}}
        onClose={() => {}}
      />,
    );

    expect(screen.getByText("Sarah")).toBeInTheDocument();
    expect(screen.getByText("only after 6pm")).toBeInTheDocument();
    expect(screen.getByText("Me")).toBeInTheDocument();
    // "you" tag renders next to the current participant.
    expect(screen.getByText("you")).toBeInTheDocument();
  });

  it("counter reflects code-point length — emoji count as 1", async () => {
    const user = userEvent.setup();
    const me = makeParticipant("me", "Me");
    render(
      <DayDetailSheet
        date={date}
        participants={[me]}
        availability={[makeAvailability("me", date)]}
        currentParticipantId="me"
        currentNote=""
        isSelected={true}
        finalised={false}
        onSaveNote={() => {}}
        onRemoveAvailability={() => {}}
        onClose={() => {}}
      />,
    );

    const textarea = screen.getByRole("textbox") as HTMLTextAreaElement;
    // Three emoji that are each a single code point (BMP-only emoji chosen so
    // userEvent.type can insert them without surrogate-pair issues).
    await user.type(textarea, "🎉🎉🎉");
    expect(screen.getByText("3/140")).toBeInTheDocument();
  });

  it("save calls onSaveNote with the trimmed text", async () => {
    const user = userEvent.setup();
    const me = makeParticipant("me", "Me");
    const onSaveNote = vi.fn();
    render(
      <DayDetailSheet
        date={date}
        participants={[me]}
        availability={[makeAvailability("me", date)]}
        currentParticipantId="me"
        currentNote=""
        isSelected={true}
        finalised={false}
        onSaveNote={onSaveNote}
        onRemoveAvailability={() => {}}
        onClose={() => {}}
      />,
    );

    const textarea = screen.getByRole("textbox") as HTMLTextAreaElement;
    await user.type(textarea, "  only after 6pm  ");
    await user.click(screen.getByRole("button", { name: "Save" }));
    expect(onSaveNote).toHaveBeenCalledWith("only after 6pm");
  });

  it("input longer than 140 code points is capped on entry", async () => {
    const user = userEvent.setup();
    const me = makeParticipant("me", "Me");
    render(
      <DayDetailSheet
        date={date}
        participants={[me]}
        availability={[makeAvailability("me", date)]}
        currentParticipantId="me"
        currentNote=""
        isSelected={true}
        finalised={false}
        onSaveNote={() => {}}
        onRemoveAvailability={() => {}}
        onClose={() => {}}
      />,
    );

    const textarea = screen.getByRole("textbox") as HTMLTextAreaElement;
    // Type 145 'a' — the textarea should hold 140.
    await user.type(textarea, "a".repeat(145));
    expect(textarea.value).toHaveLength(140);
    expect(screen.getByText("140/140")).toBeInTheDocument();
  });

  it("remove-availability button calls the prop and then closes", async () => {
    const user = userEvent.setup();
    const me = makeParticipant("me", "Me");
    const onRemoveAvailability = vi.fn();
    const onClose = vi.fn();
    render(
      <DayDetailSheet
        date={date}
        participants={[me]}
        availability={[makeAvailability("me", date)]}
        currentParticipantId="me"
        currentNote=""
        isSelected={true}
        finalised={false}
        onSaveNote={() => {}}
        onRemoveAvailability={onRemoveAvailability}
        onClose={onClose}
      />,
    );

    await user.click(screen.getByRole("button", { name: /remove my availability/i }));
    expect(onRemoveAvailability).toHaveBeenCalledTimes(1);
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("Esc closes the sheet", async () => {
    const user = userEvent.setup();
    const me = makeParticipant("me", "Me");
    const onClose = vi.fn();
    render(
      <DayDetailSheet
        date={date}
        participants={[me]}
        availability={[makeAvailability("me", date)]}
        currentParticipantId="me"
        currentNote=""
        isSelected={true}
        finalised={false}
        onSaveNote={() => {}}
        onRemoveAvailability={() => {}}
        onClose={onClose}
      />,
    );
    await user.keyboard("{Escape}");
    expect(onClose).toHaveBeenCalled();
  });

  it("read-only when finalised: no editor, no remove button", () => {
    const me = makeParticipant("me", "Me");
    render(
      <DayDetailSheet
        date={date}
        participants={[me]}
        availability={[makeAvailability("me", date, "see you there")]}
        currentParticipantId="me"
        currentNote="see you there"
        isSelected={true}
        finalised={true}
        onSaveNote={() => {}}
        onRemoveAvailability={() => {}}
        onClose={() => {}}
      />,
    );

    expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /remove my availability/i }),
    ).not.toBeInTheDocument();
    expect(screen.getByText("see you there")).toBeInTheDocument();
  });

  it("newlines are stripped from input before counting", async () => {
    const user = userEvent.setup();
    const me = makeParticipant("me", "Me");
    const onSaveNote = vi.fn();
    render(
      <DayDetailSheet
        date={date}
        participants={[me]}
        availability={[makeAvailability("me", date)]}
        currentParticipantId="me"
        currentNote=""
        isSelected={true}
        finalised={false}
        onSaveNote={onSaveNote}
        onRemoveAvailability={() => {}}
        onClose={() => {}}
      />,
    );

    const textarea = screen.getByRole("textbox") as HTMLTextAreaElement;
    // Enter key submits; use shift+enter if we wanted a newline. But the
    // stripping rule says newlines should never land in the textarea even if
    // someone pastes one. Simulate by firing a change event directly.
    textarea.focus();
    await user.paste("line1\nline2");
    // Newline → space per stripNewlines().
    expect(textarea.value).not.toContain("\n");
    expect(textarea.value).toContain("line1 line2");
  });

  describe("preferences", () => {
    it("checkbox reflects isPreferred and toggle fires onToggleVote with inverted bool", async () => {
      const user = userEvent.setup();
      const onToggleVote = vi.fn();
      const me = makeParticipant("me", "Me");
      render(
        <DayDetailSheet
          date={date}
          participants={[me]}
          availability={[makeAvailability("me", date)]}
          currentParticipantId="me"
          currentNote=""
          isSelected={true}
          finalised={false}
          isPreferred={false}
          preferredParticipantIds={new Set()}
          onSaveNote={() => {}}
          onToggleVote={onToggleVote}
          onRemoveAvailability={() => {}}
          onClose={() => {}}
        />,
      );
      const checkbox = screen.getByRole("checkbox", { name: /prefer this date/i });
      expect(checkbox).not.toBeChecked();
      await user.click(checkbox);
      expect(onToggleVote).toHaveBeenCalledWith(true);
    });

    it("renders a star next to participants who preferred this date", () => {
      const me = makeParticipant("me", "Me");
      const sarah = makeParticipant("s", "Sarah");
      render(
        <DayDetailSheet
          date={date}
          participants={[me, sarah]}
          availability={[makeAvailability("me", date), makeAvailability("s", date)]}
          currentParticipantId="me"
          currentNote=""
          isSelected={true}
          finalised={false}
          isPreferred={false}
          preferredParticipantIds={new Set(["s"])}
          onSaveNote={() => {}}
          onToggleVote={() => {}}
          onRemoveAvailability={() => {}}
          onClose={() => {}}
        />,
      );
      expect(screen.getByLabelText("Sarah prefers this date")).toBeInTheDocument();
    });

    it("hides the prefer checkbox when finalised but still shows stars", () => {
      const me = makeParticipant("me", "Me");
      render(
        <DayDetailSheet
          date={date}
          participants={[me]}
          availability={[makeAvailability("me", date)]}
          currentParticipantId="me"
          currentNote=""
          isSelected={true}
          finalised={true}
          isPreferred={true}
          preferredParticipantIds={new Set(["me"])}
          onSaveNote={() => {}}
          onToggleVote={() => {}}
          onRemoveAvailability={() => {}}
          onClose={() => {}}
        />,
      );
      expect(screen.queryByRole("checkbox", { name: /prefer this date/i })).toBeNull();
      expect(screen.getByLabelText("Me prefers this date")).toBeInTheDocument();
    });
  });
});
