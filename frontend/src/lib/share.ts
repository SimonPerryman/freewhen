const finalisedDateFormatter = new Intl.DateTimeFormat("en-GB", {
  weekday: "long",
  day: "numeric",
  month: "long",
});

function formatFinalisedDate(dateStr: string): string {
  const [y, m, d] = dateStr.split("-").map(Number);
  return finalisedDateFormatter.format(new Date(y, m - 1, d));
}

export interface BuildShareMessageArgs {
  name: string;
  emoji?: string | null;
  finalisedDate?: string | null;
}

export function buildShareMessage({ name, emoji, finalisedDate }: BuildShareMessageArgs): string {
  const prefix = emoji ? `${emoji} ` : "";
  const displayName = name.trim() || "this calendar";
  if (finalisedDate) {
    return `${prefix}${displayName} is on ${formatFinalisedDate(finalisedDate)} — add it to your calendar:`;
  }
  return `${prefix}Help me find a date for ${displayName} — mark when you're free:`;
}
