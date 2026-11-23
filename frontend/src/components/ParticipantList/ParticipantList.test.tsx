import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import ParticipantList from "./ParticipantList";
import type { Participant } from "@/lib/types";

const participant = (overrides: Partial<Participant> = {}): Participant => ({
  id: "p1",
  calendarId: "c1",
  name: "Alice",
  colour: "#ff0000",
  ...overrides,
});

describe("ParticipantList", () => {
  it("renders nothing when participants is empty", () => {
    const { container } = render(<ParticipantList participants={[]} currentParticipantId={null} />);
    expect(container.firstChild).toBeNull();
  });

  it("renders participant names", () => {
    const participants = [
      participant({ id: "p1", name: "Alice" }),
      participant({ id: "p2", name: "Bob" }),
    ];
    render(<ParticipantList participants={participants} currentParticipantId={null} />);

    expect(screen.getByText("Alice")).toBeInTheDocument();
    expect(screen.getByText("Bob")).toBeInTheDocument();
  });

  it("marks the current participant with (you)", () => {
    const participants = [
      participant({ id: "p1", name: "Alice" }),
      participant({ id: "p2", name: "Bob" }),
    ];
    render(<ParticipantList participants={participants} currentParticipantId="p1" />);

    expect(screen.getByText(/Alice/)).toHaveTextContent("Alice (you)");
    expect(screen.getByText("Bob")).not.toHaveTextContent("(you)");
  });

  it("renders heading", () => {
    render(<ParticipantList participants={[participant()]} currentParticipantId={null} />);
    expect(screen.getByText("Participants")).toBeInTheDocument();
  });

  it("hides remove buttons when canRemove is false", () => {
    render(
      <ParticipantList
        participants={[participant()]}
        currentParticipantId={null}
        canRemove={false}
        onRemove={vi.fn()}
      />,
    );
    expect(screen.queryByRole("button", { name: /Remove Alice/i })).not.toBeInTheDocument();
  });

  it("renders a remove button per participant when canRemove is true", () => {
    render(
      <ParticipantList
        participants={[
          participant({ id: "p1", name: "Alice" }),
          participant({ id: "p2", name: "Bob" }),
        ]}
        currentParticipantId="p1"
        canRemove
        onRemove={vi.fn()}
      />,
    );
    expect(screen.getByRole("button", { name: "Remove Alice" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Remove Bob" })).toBeInTheDocument();
  });

  it("shows rename pencil only on the current row by default", () => {
    render(
      <ParticipantList
        participants={[
          participant({ id: "p1", name: "Alice" }),
          participant({ id: "p2", name: "Bob" }),
        ]}
        currentParticipantId="p1"
        onRename={vi.fn()}
      />,
    );
    expect(screen.getByRole("button", { name: "Rename Alice" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Rename Bob" })).not.toBeInTheDocument();
  });

  it("shows rename pencil on every row when canRenameOthers is true", () => {
    render(
      <ParticipantList
        participants={[
          participant({ id: "p1", name: "Alice" }),
          participant({ id: "p2", name: "Bob" }),
        ]}
        currentParticipantId="p1"
        canRenameOthers
        onRename={vi.fn()}
      />,
    );
    expect(screen.getByRole("button", { name: "Rename Alice" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Rename Bob" })).toBeInTheDocument();
  });

  it("calls onRename with trimmed value on Enter", async () => {
    const onRename = vi.fn().mockResolvedValue(undefined);
    const user = userEvent.setup();
    render(
      <ParticipantList
        participants={[participant({ id: "p1", name: "Alice" })]}
        currentParticipantId="p1"
        onRename={onRename}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Rename Alice" }));
    const input = screen.getByLabelText("Rename Alice") as HTMLInputElement;
    await user.clear(input);
    await user.type(input, "Alicia  {Enter}");

    expect(onRename).toHaveBeenCalledWith(expect.objectContaining({ id: "p1" }), "Alicia");
  });

  it("Escape cancels without calling onRename", async () => {
    const onRename = vi.fn();
    const user = userEvent.setup();
    render(
      <ParticipantList
        participants={[participant({ id: "p1", name: "Alice" })]}
        currentParticipantId="p1"
        onRename={onRename}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Rename Alice" }));
    const input = screen.getByLabelText("Rename Alice") as HTMLInputElement;
    await user.clear(input);
    await user.type(input, "Alicia{Escape}");

    expect(onRename).not.toHaveBeenCalled();
  });

  it("calls onRemove with the participant when its remove button is clicked", async () => {
    const onRemove = vi.fn();
    const alice = participant({ id: "p1", name: "Alice" });
    const user = userEvent.setup();
    render(
      <ParticipantList
        participants={[alice]}
        currentParticipantId={null}
        canRemove
        onRemove={onRemove}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Remove Alice" }));
    expect(onRemove).toHaveBeenCalledWith(alice);
  });
});
