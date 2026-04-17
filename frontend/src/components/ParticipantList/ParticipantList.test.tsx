import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
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
    const { container } = render(
      <ParticipantList participants={[]} currentParticipantId={null} />,
    );
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
    render(
      <ParticipantList
        participants={[participant()]}
        currentParticipantId={null}
      />,
    );
    expect(screen.getByText("Participants")).toBeInTheDocument();
  });
});
