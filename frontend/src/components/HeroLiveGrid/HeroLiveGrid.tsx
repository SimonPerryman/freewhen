"use client";

import { useEffect, useRef, useState, type CSSProperties } from "react";
import styles from "./HeroLiveGrid.module.css";

type Intensity = 0 | 1 | 2 | 3;

// Fixed pattern from the design mockup's `liveGrid` script. 21 cells = 7 cols x 3 rows.
// Values map to heat intensity: 0 = empty, 1 = h1 (light), 2 = h2 (mid), 3 = h3 (full terra).
const PATTERN: ReadonlyArray<Intensity> = [
  1, 2, 3, 0, 2, 3, 1, 2, 3, 3, 1, 3, 3, 2, 3, 3, 2, 2, 3, 3, 2,
];

const INTENSITY_CLASS: Record<1 | 2 | 3, string> = {
  1: styles.cellH1,
  2: styles.cellH2,
  3: styles.cellH3,
};

const DAY_LABELS = ["M", "T", "W", "T", "F", "S", "S"] as const;

const TICK_MS = 1800;
const GRID_COLS = 7;
const RECENT_HISTORY = 3;

type CellStyle = CSSProperties & { "--i": number };

// Weighted toward heat: empty cells stay uncommon so the grid reads as "filling in".
const INTENSITY_WEIGHTS: ReadonlyArray<{ value: Intensity; weight: number }> = [
  { value: 0, weight: 10 },
  { value: 1, weight: 30 },
  { value: 2, weight: 30 },
  { value: 3, weight: 30 },
];

function nextIntensity(current: Intensity): Intensity {
  const choices = INTENSITY_WEIGHTS.filter((c) => c.value !== current);
  const total = choices.reduce((sum, c) => sum + c.weight, 0);
  let r = Math.random() * total;
  for (const c of choices) {
    r -= c.weight;
    if (r <= 0) return c.value;
  }
  return choices[choices.length - 1]!.value;
}

function pickCellIndex(length: number, recent: ReadonlyArray<number>): number {
  const excluded = new Set<number>();
  for (const idx of recent) {
    excluded.add(idx);
    excluded.add(idx - 1);
    excluded.add(idx + 1);
    excluded.add(idx - GRID_COLS);
    excluded.add(idx + GRID_COLS);
  }
  const candidates: number[] = [];
  for (let i = 0; i < length; i++) {
    if (!excluded.has(i)) candidates.push(i);
  }
  if (candidates.length === 0) return Math.floor(Math.random() * length);
  return candidates[Math.floor(Math.random() * candidates.length)]!;
}

export function HeroLiveGrid() {
  const [cells, setCells] = useState<ReadonlyArray<Intensity>>(PATTERN);
  const recentRef = useRef<number[]>([]);

  useEffect(() => {
    const reducedMotion =
      typeof window !== "undefined" && typeof window.matchMedia === "function"
        ? window.matchMedia("(prefers-reduced-motion: reduce)")
        : null;
    if (reducedMotion?.matches) return;

    let timer: ReturnType<typeof setInterval> | null = null;

    const start = () => {
      if (timer !== null) return;
      timer = setInterval(() => {
        setCells((prev) => {
          const i = pickCellIndex(prev.length, recentRef.current);
          const copy = prev.slice() as Intensity[];
          copy[i] = nextIntensity(prev[i] ?? 0);
          const recent = recentRef.current;
          recent.push(i);
          if (recent.length > RECENT_HISTORY) recent.shift();
          return copy;
        });
      }, TICK_MS);
    };

    const stop = () => {
      if (timer !== null) {
        clearInterval(timer);
        timer = null;
      }
    };

    const sync = () => {
      if (document.visibilityState === "hidden" || reducedMotion?.matches) {
        stop();
      } else {
        start();
      }
    };

    sync();
    document.addEventListener("visibilitychange", sync);
    reducedMotion?.addEventListener?.("change", sync);

    return () => {
      stop();
      document.removeEventListener("visibilitychange", sync);
      reducedMotion?.removeEventListener?.("change", sync);
    };
  }, []);

  return (
    <div className={styles.card} aria-hidden="true">
      <div className={styles.head}>
        <span className={styles.headRange}>Apr 28 — May 4</span>
      </div>
      <div className={styles.dow}>
        {DAY_LABELS.map((label, i) => (
          <span key={i} className={styles.dowCell}>
            {label}
          </span>
        ))}
      </div>
      <div className={styles.grid}>
        {cells.map((intensity, i) => {
          const intensityClass =
            intensity === 0 ? "" : INTENSITY_CLASS[intensity];
          const style: CellStyle = { "--i": i };
          return (
            <div
              key={i}
              className={`${styles.cell} ${intensityClass}`.trim()}
              style={style}
            />
          );
        })}
      </div>
    </div>
  );
}

export default HeroLiveGrid;
