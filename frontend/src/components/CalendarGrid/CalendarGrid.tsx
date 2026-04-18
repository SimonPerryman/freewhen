"use client";

import { useState, useMemo, useCallback, useRef } from "react";
import type { Participant } from "@/lib/types";
import { layoutSlots, sortParticipants, classifyDay } from "@/lib/overlap";
import styles from "./CalendarGrid.module.css";

const DAY_NAMES = ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"];
const DAY_LETTERS = ["M", "T", "W", "T", "F", "S", "S"];
const MONTH_SHORT = [
  "Jan",
  "Feb",
  "Mar",
  "Apr",
  "May",
  "Jun",
  "Jul",
  "Aug",
  "Sep",
  "Oct",
  "Nov",
  "Dec",
];
const WEEKS_VISIBLE = 5;

interface CalendarGridProps {
  participants: Participant[];
  freeByDate: Map<string, Set<string>>;
  currentParticipantId: string | null;
  onToggle: (date: string) => void;
  joined: boolean;
}

function formatDate(d: Date): string {
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${y}-${m}-${day}`;
}

function formatDateLabel(d: Date): string {
  return `${MONTH_SHORT[d.getMonth()]} ${d.getDate()}, ${d.getFullYear()}`;
}

function getToday(): string {
  return formatDate(new Date());
}

function getMonday(date: Date): Date {
  const d = new Date(date);
  d.setHours(0, 0, 0, 0);
  const day = d.getDay();
  const diff = day === 0 ? -6 : 1 - day;
  d.setDate(d.getDate() + diff);
  return d;
}

function addDays(date: Date, days: number): Date {
  const d = new Date(date);
  d.setDate(d.getDate() + days);
  return d;
}

function buildAriaLabel(dateLabel: string, sorted: Participant[], freeIds: Set<string>): string {
  if (sorted.length === 0) return dateLabel;
  const free = sorted.filter((p) => freeIds.has(p.id)).map((p) => p.name);
  const notFree = sorted.filter((p) => !freeIds.has(p.id)).map((p) => p.name);
  const parts = [dateLabel];
  if (free.length > 0) parts.push(`Free: ${free.join(", ")}.`);
  if (notFree.length > 0) parts.push(`Not free: ${notFree.join(", ")}.`);
  return parts.join(" ");
}

const SWIPE_THRESHOLD = 50;

export default function CalendarGrid({
  participants,
  freeByDate,
  currentParticipantId: _currentParticipantId,
  onToggle,
  joined,
}: CalendarGridProps) {
  const [weekOffset, setWeekOffset] = useState(0);
  const [pulsing, setPulsing] = useState<string | null>(null);
  const pulseTimer = useRef<ReturnType<typeof setTimeout>>(null);
  const swipeStartX = useRef<number | null>(null);
  const swipeStartY = useRef<number | null>(null);

  const handleTouchStart = useCallback((e: React.TouchEvent) => {
    swipeStartX.current = e.touches[0].clientX;
    swipeStartY.current = e.touches[0].clientY;
  }, []);

  const handleTouchEnd = useCallback(
    (e: React.TouchEvent) => {
      if (swipeStartX.current === null || swipeStartY.current === null) return;
      const dx = e.changedTouches[0].clientX - swipeStartX.current;
      const dy = e.changedTouches[0].clientY - swipeStartY.current;
      swipeStartX.current = null;
      swipeStartY.current = null;

      // Only trigger if horizontal swipe is dominant and exceeds threshold
      if (Math.abs(dx) < SWIPE_THRESHOLD || Math.abs(dy) > Math.abs(dx)) return;

      if (dx < 0) {
        setWeekOffset((o) => o + 1);
      } else if (weekOffset > 0) {
        setWeekOffset((o) => o - 1);
      }
    },
    [weekOffset],
  );

  const handleDayClick = useCallback(
    (dateStr: string) => {
      if (pulseTimer.current) clearTimeout(pulseTimer.current);
      setPulsing(dateStr);
      pulseTimer.current = setTimeout(() => setPulsing(null), 300);
      onToggle(dateStr);
    },
    [onToggle],
  );

  const today = getToday();
  const baseMonday = getMonday(new Date());

  const sorted = useMemo(() => sortParticipants(participants), [participants]);
  const layout = useMemo(() => layoutSlots(sorted.length), [sorted.length]);

  const days = useMemo(() => {
    const start = addDays(baseMonday, weekOffset * 7);
    const result: Date[] = [];
    for (let i = 0; i < WEEKS_VISIBLE * 7; i++) {
      result.push(addDays(start, i));
    }
    return result;
  }, [weekOffset, baseMonday.getTime()]);

  const rangeLabel = useMemo(() => {
    const first = days[0];
    const last = days[days.length - 1];
    const fMonth = MONTH_SHORT[first.getMonth()];
    const lMonth = MONTH_SHORT[last.getMonth()];
    const fYear = first.getFullYear();
    const lYear = last.getFullYear();

    if (fYear !== lYear) {
      return `${fMonth} ${first.getDate()}, ${fYear} – ${lMonth} ${last.getDate()}, ${lYear}`;
    }
    if (fMonth !== lMonth) {
      return `${fMonth} ${first.getDate()} – ${lMonth} ${last.getDate()}, ${fYear}`;
    }
    return `${fMonth} ${first.getDate()} – ${last.getDate()}, ${fYear}`;
  }, [days]);

  // Compute cell min-height based on slot count
  const cellMinHeight = useMemo(() => {
    if (layout.rows === 0) return 56;
    const slotHeight = 6;
    const gapSize = 1;
    const dateLine = 20;
    const padding = 8;
    const computed = dateLine + layout.rows * slotHeight + (layout.rows - 1) * gapSize + padding;
    return Math.max(56, Math.min(120, computed));
  }, [layout.rows]);

  const totalSlots = layout.cols * layout.rows;

  return (
    <div
      className={styles.container}
      onTouchStart={handleTouchStart}
      onTouchEnd={handleTouchEnd}
    >
      <div className={styles.nav}>
        <button
          type="button"
          className={styles.navButton}
          onClick={() => setWeekOffset((o) => o - 1)}
          disabled={weekOffset <= 0}
          aria-label="Previous week"
        >
          ‹
        </button>
        <span className={styles.rangeLabel}>{rangeLabel}</span>
        <button
          type="button"
          className={styles.navButton}
          onClick={() => setWeekOffset((o) => o + 1)}
          aria-label="Next week"
        >
          ›
        </button>
      </div>

      <div className={styles.grid} key={weekOffset}>
        {DAY_NAMES.map((name, i) => (
          <div key={name} className={styles.dayHeader}>
            <span className={styles.dayHeaderShort}>{DAY_LETTERS[i]}</span>
            <span className={styles.dayHeaderFull}>{name}</span>
          </div>
        ))}

        {days.map((date, i) => {
          const dateStr = formatDate(date);
          const isPast = dateStr < today;
          const isToday = dateStr === today;
          const showMonth = date.getDate() === 1;
          const freeIds = freeByDate.get(dateStr) ?? new Set<string>();
          const { everyoneFree } = classifyDay(sorted, freeIds);

          // Heat map: intensity from 0 (no one free) to 1 (everyone free)
          const heatRatio = sorted.length > 0 && !isPast ? freeIds.size / sorted.length : 0;
          const heatPercent = Math.round(heatRatio * 14); // max 14% mix for subtle tint

          const classNames = [styles.day, styles.cellEntrance];
          if (isPast) classNames.push(styles.past);
          if (isToday) classNames.push(styles.today);
          if (joined && !isPast) classNames.push(styles.clickable);
          if (everyoneFree && !isPast) classNames.push(styles.everyoneFree);
          if (pulsing === dateStr) classNames.push(styles.toggle);

          const dateLabel = formatDateLabel(date);
          const ariaLabel = isPast ? dateLabel : buildAriaLabel(dateLabel, sorted, freeIds);
          const titleText = isPast ? undefined : buildAriaLabel(dateLabel, sorted, freeIds);

          const cellStyle: React.CSSProperties = {
            minHeight: `${cellMinHeight}px`,
            ["--entrance-delay" as string]: `${i * 15}ms`,
          };
          if (heatPercent > 0 && !everyoneFree) {
            cellStyle.backgroundColor = `color-mix(in srgb, var(--accent) ${heatPercent}%, var(--card-bg))`;
          }

          return (
            <button
              key={dateStr}
              type="button"
              className={classNames.join(" ")}
              onClick={() => {
                if (joined && !isPast) handleDayClick(dateStr);
              }}
              disabled={!joined || isPast}
              style={cellStyle}
              aria-label={ariaLabel}
              title={titleText}
            >
              {showMonth ? (
                <span className={styles.monthLabel}>{MONTH_SHORT[date.getMonth()]}</span>
              ) : null}
              <span className={styles.dayNumber}>{date.getDate()}</span>
              {!isPast && sorted.length > 0 ? (
                <div
                  className={styles.slotGrid}
                  data-testid="slot-grid"
                  style={{
                    gridTemplateColumns: `repeat(${layout.cols}, 1fr)`,
                    gridTemplateRows: `repeat(${layout.rows}, 1fr)`,
                  }}
                >
                  {sorted.map((p) => {
                    const isFree = freeIds.has(p.id);
                    return (
                      <span
                        key={p.id}
                        className={isFree ? styles.slotFree : styles.slotFaint}
                        style={{ backgroundColor: p.colour }}
                        data-testid={`slot-${p.id}`}
                      />
                    );
                  })}
                  {/* Empty trailing slots */}
                  {Array.from({ length: totalSlots - sorted.length }, (_, i) => (
                    <span key={`empty-${i}`} className={styles.slotEmpty} />
                  ))}
                </div>
              ) : null}
            </button>
          );
        })}
      </div>
    </div>
  );
}
