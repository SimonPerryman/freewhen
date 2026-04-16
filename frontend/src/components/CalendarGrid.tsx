"use client";

import { useState, useMemo } from "react";
import type { Participant } from "@/lib/types";
import styles from "./CalendarGrid.module.css";

const DAY_NAMES = ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"];
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
  selectedDates: Set<string>;
  otherAvailability: Map<string, Participant[]>;
  onToggle: (date: string) => void;
  userColour: string | null;
  joined: boolean;
}

function formatDate(d: Date): string {
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${y}-${m}-${day}`;
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

export default function CalendarGrid({
  selectedDates,
  otherAvailability,
  onToggle,
  userColour,
  joined,
}: CalendarGridProps) {
  const [weekOffset, setWeekOffset] = useState(0);

  const today = getToday();
  const baseMonday = getMonday(new Date());

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

  return (
    <div className={styles.container}>
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

      <div className={styles.grid}>
        {DAY_NAMES.map((name) => (
          <div key={name} className={styles.dayHeader}>
            {name}
          </div>
        ))}

        {days.map((date) => {
          const dateStr = formatDate(date);
          const isPast = dateStr < today;
          const isSelected = selectedDates.has(dateStr);
          const isToday = dateStr === today;
          const others = otherAvailability.get(dateStr) ?? [];
          const showMonth = date.getDate() === 1;

          const classNames = [styles.day];
          if (isPast) classNames.push(styles.past);
          if (isSelected) classNames.push(styles.selected);
          if (isToday) classNames.push(styles.today);
          if (joined && !isPast) classNames.push(styles.clickable);

          return (
            <button
              key={dateStr}
              type="button"
              className={classNames.join(" ")}
              onClick={() => {
                if (joined && !isPast) onToggle(dateStr);
              }}
              disabled={!joined || isPast}
              style={
                isSelected && userColour
                  ? { backgroundColor: userColour, borderColor: userColour }
                  : undefined
              }
              aria-label={`${dateStr}${isSelected ? " (selected)" : ""}`}
              aria-pressed={isSelected}
            >
              {showMonth ? (
                <span className={styles.monthLabel}>{MONTH_SHORT[date.getMonth()]}</span>
              ) : null}
              <span className={isSelected ? styles.dayNumberSelected : styles.dayNumber}>
                {date.getDate()}
              </span>
              {others.length > 0 ? (
                <div className={styles.dots}>
                  {others.slice(0, 4).map((p) => (
                    <span
                      key={p.id}
                      className={styles.dot}
                      style={{ backgroundColor: p.colour }}
                      title={p.name}
                    />
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
