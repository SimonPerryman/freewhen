import type { CSSProperties } from "react";
import styles from "./HeroLiveGrid.module.css";

// Fixed pattern from the design mockup's `liveGrid` script. 21 cells = 7 cols x 3 rows.
// Values map to heat intensity: 0 = empty, 1 = h1 (light), 2 = h2 (mid), 3 = h3 (full terra).
const PATTERN: ReadonlyArray<0 | 1 | 2 | 3> = [
  1, 2, 3, 0, 2, 3, 1, 2, 3, 3, 1, 3, 3, 2, 3, 3, 2, 2, 3, 3, 2,
];

const INTENSITY_CLASS: Record<1 | 2 | 3, string> = {
  1: styles.cellH1,
  2: styles.cellH2,
  3: styles.cellH3,
};

const DAY_LABELS = ["M", "T", "W", "T", "F", "S", "S"] as const;

type CellStyle = CSSProperties & { "--i": number };

export function HeroLiveGrid() {
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
        {PATTERN.map((intensity, i) => {
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
