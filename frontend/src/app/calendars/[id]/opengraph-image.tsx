import { ImageResponse } from "next/og";
import { getCalendar } from "@/lib/api";
import { titlePlain } from "@/lib/title";
import { computeSummary } from "@/lib/summary";
import type { Availability, Participant } from "@/lib/types";

export const size = { width: 1200, height: 630 };
export const contentType = "image/png";
export const alt = "FreeWhen calendar";

interface Props {
  params: Promise<{ id: string }>;
}

// Heat-map fallback pattern matching the mockup. 21 cells = 3 rows × 7 cols.
const FALLBACK_PATTERN: Array<0 | 1 | 2 | 3> = [
  1, 2, 3, 0, 2, 3, 1, 2, 3, 3, 1, 3, 3, 2, 3, 3, 2, 2, 3, 3, 2,
];

const CELL_COLOURS = {
  0: "#efe9e2",
  1: "#f3c5b6",
  2: "#ee9077",
  3: "#e06449",
} as const;

const TERRA = "#e06449";
const CREAM = "#faf9f7";
const INK = "#221f1c";
const INK_2 = "#6b6058";
const BORDER = "#e8e2d8";

const DAY_FORMATTER = new Intl.DateTimeFormat("en-GB", {
  weekday: "short",
  day: "numeric",
  month: "short",
});

function formatBestDate(dateStr: string): string {
  const [y, m, d] = dateStr.split("-").map(Number);
  return DAY_FORMATTER.format(new Date(y, m - 1, d));
}

function buildFreeByDate(availability: Availability[]): Map<string, Set<string>> {
  const m = new Map<string, Set<string>>();
  for (const a of availability) {
    let s = m.get(a.date);
    if (!s) {
      s = new Set();
      m.set(a.date, s);
    }
    s.add(a.participantId);
  }
  return m;
}

// Build 21 buckets (3 rows × 7 cols) from the next 21 future dates with any
// availability marked. Each cell's intensity is bucketed into 0–3 by ratio
// of free participants vs. total.
function buildHeatBuckets(
  participants: Participant[],
  freeByDate: Map<string, Set<string>>,
): Array<0 | 1 | 2 | 3> {
  if (participants.length === 0 || freeByDate.size === 0) {
    return FALLBACK_PATTERN;
  }
  const todayStr = (() => {
    const d = new Date();
    const y = d.getFullYear();
    const mo = String(d.getMonth() + 1).padStart(2, "0");
    const da = String(d.getDate()).padStart(2, "0");
    return `${y}-${mo}-${da}`;
  })();

  const dates = [...freeByDate.keys()].filter((d) => d >= todayStr).sort();
  if (dates.length === 0) return FALLBACK_PATTERN;

  const total = participants.length;
  const out: Array<0 | 1 | 2 | 3> = [];
  for (let i = 0; i < 21; i++) {
    const dateKey = dates[i];
    if (!dateKey) {
      out.push(0);
      continue;
    }
    const free = freeByDate.get(dateKey)?.size ?? 0;
    const ratio = free / total;
    if (ratio <= 0) out.push(0);
    else if (ratio < 0.34) out.push(1);
    else if (ratio < 0.67) out.push(2);
    else out.push(3);
  }
  return out;
}

function countWeekends(freeByDate: Map<string, Set<string>>): number {
  const todayStr = new Date().toISOString().slice(0, 10);
  const weekends = new Set<string>();
  for (const date of freeByDate.keys()) {
    if (date < todayStr) continue;
    const [y, m, d] = date.split("-").map(Number);
    const dt = new Date(y, m - 1, d);
    const dow = dt.getDay();
    if (dow === 0 || dow === 6) {
      // Bucket by week start (Saturday/Sunday share).
      const monday = new Date(dt);
      const offset = dow === 0 ? -6 : 1 - dow;
      monday.setDate(monday.getDate() + offset);
      weekends.add(monday.toISOString().slice(0, 10));
    }
  }
  return weekends.size;
}

interface TitleSegment {
  text: string;
  emphasis: boolean;
}

function parseTitle(source: string): TitleSegment[] {
  const out: TitleSegment[] = [];
  if (!source) return out;
  const re = /\\\*|\*([^*\n]+)\*/g;
  let last = 0;
  for (const match of source.matchAll(re)) {
    const idx = match.index ?? 0;
    if (idx > last) out.push({ text: source.slice(last, idx), emphasis: false });
    if (match[0] === "\\*") {
      out.push({ text: "*", emphasis: false });
    } else {
      out.push({ text: match[1] ?? "", emphasis: true });
    }
    last = idx + match[0].length;
  }
  if (last < source.length) out.push({ text: source.slice(last), emphasis: false });
  // If no explicit emphasis, emphasise the last word.
  if (out.every((s) => !s.emphasis) && out.length > 0) {
    const lastIdx = out.length - 1;
    const seg = out[lastIdx];
    const trimmed = seg.text.replace(/\s+$/, "");
    const m = /\s(\S+)$/.exec(trimmed);
    if (m && m.index !== undefined) {
      const head = trimmed.slice(0, m.index + 1);
      const tail = trimmed.slice(m.index + 1);
      out.splice(lastIdx, 1, { text: head, emphasis: false }, { text: tail, emphasis: true });
    }
  }
  return out;
}

async function loadFonts(): Promise<
  Array<{ name: string; data: ArrayBuffer; weight?: 400 | 500; style?: "normal" | "italic" }>
> {
  const fraunces = "https://fonts.gstatic.com/s/fraunces/v37/6NUh8FyLNQOQZAnv9bYEvDiIdE9Ea92uemAk.woff2";
  const instrument =
    "https://fonts.gstatic.com/s/instrumentserif/v3/jizDREVNn1dOx-zrZ2X3pZvkTi182zIZj1bIkNo.woff2";
  try {
    const [a, b] = await Promise.all([
      fetch(fraunces).then((r) => r.arrayBuffer()),
      fetch(instrument).then((r) => r.arrayBuffer()),
    ]);
    return [
      { name: "Fraunces", data: a, weight: 400, style: "normal" },
      { name: "Instrument Serif", data: b, weight: 400, style: "italic" },
    ];
  } catch {
    return [];
  }
}

function GenericFallback() {
  return new ImageResponse(
    (
      <div
        style={{
          width: "100%",
          height: "100%",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          background: CREAM,
          color: INK,
          fontSize: 96,
          fontFamily: "Fraunces",
        }}
      >
        <span>Free</span>
        <span style={{ color: TERRA, fontStyle: "italic", fontFamily: "Instrument Serif" }}>
          When
        </span>
      </div>
    ),
    { ...size },
  );
}

export default async function Image({ params }: Props) {
  const { id } = await params;

  let calendarTitleSource = "";
  let participantCount = 0;
  let weekends = 0;
  let bestLabel: string | null = null;
  let cells: Array<0 | 1 | 2 | 3> = FALLBACK_PATTERN;

  try {
    const data = await getCalendar(id);
    if (data.requiresPassphrase) {
      return GenericFallback();
    }
    calendarTitleSource = data.calendar.name || "Untitled";
    participantCount = data.participants.length;
    const freeByDate = buildFreeByDate(data.availability);
    weekends = countWeekends(freeByDate);
    cells = buildHeatBuckets(data.participants, freeByDate);

    if (data.participants.length >= 2) {
      const summary = computeSummary(data.participants, freeByDate, new Date());
      const best =
        summary.everyoneFree[0] ?? (summary.mostFree.length > 0 ? summary.mostFree[0] : null);
      if (best) bestLabel = formatBestDate(best.date);
    }
  } catch {
    return GenericFallback();
  }

  const segments = parseTitle(calendarTitleSource);
  const titleForAlt = titlePlain(calendarTitleSource);
  const fonts = await loadFonts();

  return new ImageResponse(
    (
      <div
        style={{
          width: "100%",
          height: "100%",
          display: "flex",
          flexDirection: "column",
          justifyContent: "space-between",
          padding: "56px 64px",
          background: CREAM,
          fontFamily: "Fraunces",
          color: INK,
          position: "relative",
        }}
      >
        {/* Backdrop */}
        <div
          style={{
            position: "absolute",
            inset: 0,
            background:
              "radial-gradient(60% 80% at 100% 0%, rgba(224,100,73,0.18), transparent 60%), radial-gradient(40% 60% at 0% 100%, rgba(107,142,90,0.12), transparent 60%)",
          }}
        />

        {/* Wordmark */}
        <div
          style={{
            display: "flex",
            fontSize: 32,
            letterSpacing: "-0.01em",
            position: "relative",
          }}
        >
          <span>Free</span>
          <span
            style={{
              color: TERRA,
              fontStyle: "italic",
              fontFamily: "Instrument Serif",
            }}
          >
            When
          </span>
        </div>

        {/* Body */}
        <div
          style={{
            display: "flex",
            alignItems: "flex-end",
            justifyContent: "space-between",
            gap: 40,
            position: "relative",
          }}
        >
          <div
            style={{
              display: "flex",
              flexDirection: "column",
              flex: 1,
            }}
          >
            <div
              style={{
                display: "flex",
                flexWrap: "wrap",
                fontSize: 84,
                lineHeight: 0.98,
                letterSpacing: "-0.025em",
                marginBottom: 24,
              }}
              aria-label={titleForAlt}
            >
              {segments.map((seg, i) =>
                seg.emphasis ? (
                  <span
                    key={i}
                    style={{
                      fontFamily: "Instrument Serif",
                      fontStyle: "italic",
                      color: TERRA,
                    }}
                  >
                    {seg.text}
                  </span>
                ) : (
                  <span key={i}>{seg.text}</span>
                ),
              )}
            </div>
            <div
              style={{
                display: "flex",
                gap: 28,
                fontSize: 24,
                color: INK_2,
                fontFamily: "Fraunces",
              }}
            >
              <span style={{ display: "flex", gap: 8 }}>
                <b style={{ color: INK, fontWeight: 500 }}>{participantCount}</b>
                <span>{participantCount === 1 ? "person" : "people"}</span>
              </span>
              <span style={{ display: "flex", gap: 8 }}>
                <b style={{ color: INK, fontWeight: 500 }}>{weekends}</b>
                <span>{weekends === 1 ? "weekend" : "weekends"}</span>
              </span>
              {bestLabel ? (
                <span style={{ display: "flex", gap: 8 }}>
                  <span>Best so far ·</span>
                  <b style={{ color: INK, fontWeight: 500 }}>{bestLabel}</b>
                </span>
              ) : null}
            </div>
          </div>

          {/* Heat-map */}
          <div
            style={{
              display: "flex",
              flexDirection: "column",
              width: 460,
              background: "rgba(255,255,255,0.6)",
              border: `1px solid ${BORDER}`,
              borderRadius: 18,
              padding: 20,
            }}
          >
            <div
              style={{
                display: "flex",
                flexWrap: "wrap",
                gap: 6,
              }}
            >
              {cells.map((bucket, i) => (
                <div
                  key={i}
                  style={{
                    width: 56,
                    height: 36,
                    borderRadius: 4,
                    background: CELL_COLOURS[bucket],
                  }}
                />
              ))}
            </div>
          </div>
        </div>
      </div>
    ),
    { ...size, fonts: fonts.length > 0 ? fonts : undefined },
  );
}
