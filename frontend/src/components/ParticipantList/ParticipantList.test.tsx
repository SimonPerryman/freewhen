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
