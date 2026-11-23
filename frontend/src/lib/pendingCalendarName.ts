// The calendar name typed on the landing page while signed out. It has to
// survive the magic-link round trip (the link may open in another tab or
// browser app, so localStorage rather than sessionStorage). Consumed exactly
// once: take() removes it before the caller creates anything, so a reload
// can't create a duplicate calendar.
const KEY = "freewhen:pending-calendar-name";
const MAX_AGE_MS = 60 * 60 * 1000;

export function savePendingCalendarName(name: string): void {
  try {
    localStorage.setItem(KEY, JSON.stringify({ name, savedAt: Date.now() }));
  } catch {
    // Storage unavailable: the user just retypes the name after signing in.
  }
}

export function takePendingCalendarName(): string | null {
  try {
    const raw = localStorage.getItem(KEY);
    if (raw === null) return null;
    localStorage.removeItem(KEY);
    const parsed = JSON.parse(raw) as { name?: unknown; savedAt?: unknown };
    if (typeof parsed.name !== "string" || typeof parsed.savedAt !== "number") return null;
    if (Date.now() - parsed.savedAt > MAX_AGE_MS) return null;
    const name = parsed.name.trim();
    return name === "" ? null : name;
  } catch {
    return null;
  }
}
