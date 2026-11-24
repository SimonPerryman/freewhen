import { formatLongDay } from "@/lib/dates";

export interface BuildShareMessageArgs {
  name: string;
  emoji?: string | null;
  finalisedDate?: string | null;
  // Date locale (lib/dates.ts); the sharer's, since they write the message.
  locale?: string;
}

export function buildShareMessage({
  name,
  emoji,
  finalisedDate,
  locale,
}: BuildShareMessageArgs): string {
  const prefix = emoji ? `${emoji} ` : "";
  const displayName = name.trim() || "this calendar";
  if (finalisedDate) {
    return `${prefix}${displayName} is on ${formatLongDay(finalisedDate, locale)} — add it to your calendar:`;
  }
  return `${prefix}Help me find a date for ${displayName} — mark when you're free:`;
}
