import type { Participant } from "./types";

export const COLOUR_PALETTE = [
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

export function participantInitial(name: string): string {
  const trimmed = name.trim();
  if (!trimmed) return "";
  const [first] = new Intl.Segmenter().segment(trimmed);
  return (first?.segment ?? "").toUpperCase();
}

export function textOnColour(hex: string): "#fff" | "#000" {
  const m = /^#([0-9a-f]{6})$/i.exec(hex.trim());
  if (!m) return "#000";
  const n = parseInt(m[1], 16);
  const r = ((n >> 16) & 0xff) / 255;
  const g = ((n >> 8) & 0xff) / 255;
  const b = (n & 0xff) / 255;
  const lin = (c: number) => (c <= 0.03928 ? c / 12.92 : Math.pow((c + 0.055) / 1.055, 2.4));
  const L = 0.2126 * lin(r) + 0.7152 * lin(g) + 0.0722 * lin(b);
  return L > 0.5 ? "#000" : "#fff";
}

export function classifyDay(
  participants: Participant[],
  freeParticipantIds: Set<string>,
): { everyoneFree: boolean } {
  const everyoneFree = participants.length >= 2 && freeParticipantIds.size === participants.length;
  return { everyoneFree };
}
