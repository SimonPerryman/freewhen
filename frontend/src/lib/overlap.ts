import type { Participant } from "./types";

export const COLOUR_PALETTE = [
  "#E63946",
  "#457B9D",
  "#2A9D8F",
  "#E9C46A",
  "#F4A261",
  "#264653",
  "#A855F7",
  "#06B6D4",
  "#EC4899",
  "#84CC16",
];

export function layoutSlots(n: number): { cols: number; rows: number } {
  if (n === 0) return { cols: 0, rows: 0 };
  if (n <= 4) return { cols: 1, rows: n };
  const cols = Math.ceil(Math.sqrt(n));
  const rows = Math.ceil(n / cols);
  return { cols, rows };
}

export function slotPosition(i: number, cols: number): { col: number; row: number } {
  return { col: i % cols, row: Math.floor(i / cols) };
}

export function sortParticipants(participants: Participant[]): Participant[] {
  return [...participants].sort((a, b) => {
    const ai = COLOUR_PALETTE.indexOf(a.colour);
    const bi = COLOUR_PALETTE.indexOf(b.colour);
    const aIdx = ai === -1 ? COLOUR_PALETTE.length : ai;
    const bIdx = bi === -1 ? COLOUR_PALETTE.length : bi;
    return aIdx - bIdx;
  });
}

export function classifyDay(
  participants: Participant[],
  freeParticipantIds: Set<string>,
): { everyoneFree: boolean } {
  const everyoneFree = participants.length >= 2 && freeParticipantIds.size === participants.length;
  return { everyoneFree };
}
