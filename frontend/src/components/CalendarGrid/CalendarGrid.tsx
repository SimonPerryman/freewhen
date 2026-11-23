"use client";

import { useState, useMemo, useCallback, useEffect, useRef } from "react";
import type { Participant } from "@/lib/types";
import {
  layoutSlots,
  sortParticipants,
  classifyDay,
  participantInitial,
  textOnColour,
} from "@/lib/overlap";
import analytics from "@/lib/analytics";
import { parseLocalDate } from "@/lib/localDate";
import { useToday } from "@/lib/useToday";
import CoachmarkPreferNote from "./CoachmarkPreferNote";
import styles from "./CalendarGrid.module.css";

const COACHMARK_STORAGE_PREFIX = "freewhen.coachmark.detailSheet.";

function coachmarkKey(calendarId: string, participantId: string): string {
  return `${COACHMARK_STORAGE_PREFIX}${calendarId}.${participantId}`;
}

function readCoachmarkDismissed(calendarId: string, participantId: string): boolean {
  try {
    return localStorage.getItem(coachmarkKey(calendarId, participantId)) === "1";
  } catch {
    return true;
  }
}

function writeCoachmarkDismissed(calendarId: string, participantId: string): void {
  try {
    localStorage.setItem(coachmarkKey(calendarId, participantId), "1");
  } catch {
    /* private mode etc — silently ignore */
  }
}

// "Tap once to mark…" strip: gone for good (per browser) after the first tap.
const INSTRUCTION_STRIP_KEY = "freewhen.instructionStrip.dismissed";

function readInstructionStripDismissed(): boolean {
  try {
    return localStorage.getItem(INSTRUCTION_STRIP_KEY) === "1";
  } catch {
    return false;
  }
}

function writeInstructionStripDismissed(): void {
  try {
    localStorage.setItem(INSTRUCTION_STRIP_KEY, "1");
  } catch {
    /* private mode etc — silently ignore */
  }
}

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
  calendarId?: string;
  participants: Participant[];
  freeByDate: Map<string, Set<string>>;
  // noteAuthorsByDate[date] = set of participant IDs who have a note on that date.
  // Drives the note dot colour rule: single author → that participant's colour;
  // multiple authors → neutral.
  noteAuthorsByDate?: Map<string, Set<string>>;
  // voteCountsByDate[date] = number of participants who have marked this date
  // as preferred. Folded into the cell's heat tint as half-weight warmth on top
  // of the free-vote ratio (see plans/ui/prefer-indicator-rework.md).
  voteCountsByDate?: Map<string, number>;
  currentParticipantId: string | null;
  // Inline Google Calendar import suggestions. When present, cells for these
  // dates render in the "suggested-free" (dashed accent border + dot) or
  // "suggested-busy" (hatched) states. Tapping either fires onToggle as if
  // the cell were empty; the caller is responsible for removing the date
  // from the suggestion set once the server confirms.
  suggestedFreeDates?: Set<string>;
  suggestedBusyDates?: Set<string>;
  onToggle: (date: string) => void;
  onOpenDay?: (date: string) => void;
  onRangeToggle?: (dates: string[], target: boolean) => void;
  onClear?: () => void;
  canClear?: boolean;
  selectedDates?: Set<string>;
  joined: boolean;
  finalisedDate?: string | null;
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

// Free votes carry full weight; preference votes add half-weight warmth on top,
// capped at the same 14% accent-mix ceiling. See plans/ui/prefer-indicator-rework.md.
export function computeHeatPercent(
  participantCount: number,
  freeCount: number,
  preferCount: number,
): number {
  if (participantCount <= 0) return 0;
  const freeRatio = freeCount / participantCount;
  const preferRatio = preferCount / participantCount;
  const heatRatio = Math.min(1, freeRatio + 0.5 * preferRatio);
  return Math.round(heatRatio * 14);
}

function buildAriaLabel(
  dateLabel: string,
  sorted: Participant[],
  freeIds: Set<string>,
  preferCount: number,
): string {
  if (sorted.length === 0) return dateLabel;
  const free = sorted.filter((p) => freeIds.has(p.id)).map((p) => p.name);
  const notFree = sorted.filter((p) => !freeIds.has(p.id)).map((p) => p.name);
  const parts = [dateLabel];
  if (free.length > 0) parts.push(`Free: ${free.join(", ")}.`);
  if (notFree.length > 0) parts.push(`Not free: ${notFree.join(", ")}.`);
  if (preferCount > 0) {
    parts.push(
      preferCount === 1 ? "Preferred by 1 person." : `Preferred by ${preferCount} people.`,
    );
  }
  return parts.join(" ");
}

const SWIPE_THRESHOLD = 50;

export default function CalendarGrid({
  calendarId,
  participants,
  freeByDate,
  noteAuthorsByDate,
  voteCountsByDate,
  currentParticipantId,
  suggestedFreeDates,
  suggestedBusyDates,
  onToggle,
  onOpenDay,
  onRangeToggle,
  onClear,
  canClear = false,
  selectedDates,
  joined,
  finalisedDate = null,
}: CalendarGridProps) {
  const locked = finalisedDate !== null;
  const [weekOffset, setWeekOffset] = useState(0);
  const [pulsing, setPulsing] = useState<string | null>(null);
  const pulseTimer = useRef<ReturnType<typeof setTimeout>>(null);
  const swipeStartX = useRef<number | null>(null);
  const swipeStartY = useRef<number | null>(null);
  const containerRef = useRef<HTMLDivElement | null>(null);
  const cellRefs = useRef<Map<string, HTMLButtonElement>>(new Map());
  const [coachmarkAnchor, setCoachmarkAnchor] = useState<HTMLButtonElement | null>(null);
  const coachmarkTimer = useRef<ReturnType<typeof setTimeout>>(null);
  // Read after mount so server and first client render agree.
  const [stripDismissed, setStripDismissed] = useState(false);
  useEffect(() => {
    if (readInstructionStripDismissed()) setStripDismissed(true);
  }, []);

  const dismissCoachmark = useCallback(() => {
    if (coachmarkTimer.current) {
      clearTimeout(coachmarkTimer.current);
      coachmarkTimer.current = null;
    }
    setCoachmarkAnchor(null);
    if (calendarId && currentParticipantId) {
      writeCoachmarkDismissed(calendarId, currentParticipantId);
    }
  }, [calendarId, currentParticipantId]);

  useEffect(() => {
    return () => {
      if (coachmarkTimer.current) clearTimeout(coachmarkTimer.current);
    };
  }, []);

  const toolbarAvailable = joined && !locked && !!onRangeToggle;

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
    (dateStr: string, alreadySelected: boolean) => {
      setStripDismissed(true);
      writeInstructionStripDismissed();
      // Tap-on-own-free-cell opens the day sheet (notes + "Remove availability").
      // Tap on an unselected cell still toggles fast. Range-mode taps (onRangeToggle)
      // come through a different path and are not affected.
      if (alreadySelected && onOpenDay) {
        if (coachmarkTimer.current) {
          clearTimeout(coachmarkTimer.current);
          coachmarkTimer.current = null;
        }
        if (coachmarkAnchor) dismissCoachmark();
        onOpenDay(dateStr);
        return;
      }
      if (pulseTimer.current) clearTimeout(pulseTimer.current);
      setPulsing(dateStr);
      pulseTimer.current = setTimeout(() => setPulsing(null), 300);
      onToggle(dateStr);

      // First-mark coachmark: only when adding (not removing), only once per
      // calendar+participant. The 220ms delay lets the cell entrance animation
      // land first.
      if (
        calendarId &&
        currentParticipantId &&
        !coachmarkAnchor &&
        !readCoachmarkDismissed(calendarId, currentParticipantId)
      ) {
        if (coachmarkTimer.current) clearTimeout(coachmarkTimer.current);
        coachmarkTimer.current = setTimeout(() => {
          const cell = cellRefs.current.get(dateStr);
          if (cell) setCoachmarkAnchor(cell);
        }, 220);
      }
    },
    [onToggle, onOpenDay, calendarId, currentParticipantId, coachmarkAnchor, dismissCoachmark],
  );

  // From useToday() rather than `new Date()` so a server-rendered grid
  // hydrates with the same week the server picked (see useToday).
  const today = useToday();
  const baseMonday = getMonday(parseLocalDate(today));

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

  const firstVisibleIdx = useMemo(() => {
    const idx = days.findIndex((d) => formatDate(d) >= today);
    return idx === -1 ? 0 : idx;
  }, [days, today]);

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

  const isSelectedForCurrent = useCallback(
    (dateStr: string): boolean => {
      if (selectedDates) return selectedDates.has(dateStr);
      if (!currentParticipantId) return false;
      return freeByDate.get(dateStr)?.has(currentParticipantId) ?? false;
    },
    [selectedDates, currentParticipantId, freeByDate],
  );

  const eligibleForRange = useCallback(
    (date: Date): boolean => {
      const dateStr = formatDate(date);
      if (dateStr < today) return false;
      if (finalisedDate === dateStr) return false;
      return true;
    },
    [today, finalisedDate],
  );

  const handleQuickAction = useCallback(
    (action: "weekends" | "weekdays") => {
      if (!onRangeToggle) return;
      const predicate = (date: Date): boolean => {
        const dow = date.getDay();
        const isWeekend = dow === 0 || dow === 6;
        return action === "weekends" ? isWeekend : !isWeekend;
      };
      const matching: string[] = [];
      let anyUnselected = false;
      for (const d of days) {
        if (!predicate(d)) continue;
        if (!eligibleForRange(d)) continue;
        const ds = formatDate(d);
        matching.push(ds);
        if (!isSelectedForCurrent(ds)) anyUnselected = true;
      }
      if (matching.length === 0) return;
      const target = anyUnselected;
      onRangeToggle(matching, target);
      analytics.capture("availability_quick_action_used", {
        action,
        direction: target ? "select" : "deselect",
      });
    },
    [onRangeToggle, days, eligibleForRange, isSelectedForCurrent],
  );

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
      ref={containerRef}
      className={styles.container}
      onTouchStart={handleTouchStart}
      onTouchEnd={handleTouchEnd}
    >
      {toolbarAvailable ? (
        <div className={styles.toolbar} role="toolbar" aria-label="Bulk availability tools">
          <button
            type="button"
            className={styles.toolbarButton}
            onClick={() => handleQuickAction("weekends")}
          >
            Weekends
          </button>
          <button
            type="button"
            className={styles.toolbarButton}
            onClick={() => handleQuickAction("weekdays")}
          >
            Weekdays
          </button>
          {onClear ? (
            <button
              type="button"
              className={`${styles.toolbarButton} ${styles.toolbarButtonSecondary}`}
              onClick={onClear}
              disabled={!canClear}
              aria-disabled={!canClear}
            >
              Clear
            </button>
          ) : null}
        </div>
      ) : null}

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
        <div className={styles.navCenter}>
          <span className={styles.rangeLabel}>{rangeLabel}</span>
          {weekOffset > 0 ? (
            <button
              type="button"
              className={styles.todayButton}
              onClick={() => setWeekOffset(0)}
              aria-label="Jump back to today"
            >
              <span className={styles.todayButtonIcon} aria-hidden="true">
                ↺
              </span>
              Today
            </button>
          ) : null}
        </div>
        <button
          type="button"
          className={styles.navButton}
          onClick={() => setWeekOffset((o) => o + 1)}
          aria-label="Next week"
        >
          ›
        </button>
      </div>

      {currentParticipantId && !stripDismissed && !locked ? (
        <div className={styles.instructionStrip} data-testid="instruction-strip">
          <strong>Tap once</strong> to mark a date free. <strong>Tap again</strong> to prefer it or
          add a note.
        </div>
      ) : null}

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

          if (isPast) {
            return (
              <div
                key={dateStr}
                className={styles.placeholder}
                data-testid="day-cell"
                aria-hidden="true"
              />
            );
          }

          const isToday = dateStr === today;
          const showMonth = date.getDate() === 1;
          const freeIds = freeByDate.get(dateStr) ?? new Set<string>();
          const { everyoneFree } = classifyDay(sorted, freeIds);

          const isFinalised = finalisedDate === dateStr;
          const voteCount = voteCountsByDate?.get(dateStr) ?? 0;

          const heatPercent = computeHeatPercent(sorted.length, freeIds.size, voteCount);
          const classNames = [styles.day, styles.cellEntrance];
          if (isToday) classNames.push(styles.today);
          if (joined && !locked) classNames.push(styles.clickable);
          if (everyoneFree) classNames.push(styles.everyoneFree);
          if (isFinalised) classNames.push(styles.finalised);
          if (pulsing === dateStr) classNames.push(styles.toggle);
          const isSuggestedFree = !!suggestedFreeDates?.has(dateStr);
          const isSuggestedBusy = !isSuggestedFree && !!suggestedBusyDates?.has(dateStr);
          if (isSuggestedFree) classNames.push(styles.suggestedFree);
          if (isSuggestedBusy) classNames.push(styles.suggestedBusy);

          const dateLabel = formatDateLabel(date);
          const ariaLabel = buildAriaLabel(dateLabel, sorted, freeIds, voteCount);

          const visibleIndex = i - firstVisibleIdx;
          const cellStyle: React.CSSProperties = {
            minHeight: `${cellMinHeight}px`,
            ["--entrance-delay" as string]: `${visibleIndex * 15}ms`,
          };
          if (heatPercent > 0 && !everyoneFree) {
            cellStyle.backgroundColor = `color-mix(in srgb, var(--accent) ${heatPercent}%, var(--tile-bg))`;
          }

          const alreadySelected = isSelectedForCurrent(dateStr);
          const canToggle = joined && !locked && !isFinalised;
          // When locked/finalised we still allow opening the day sheet (read-only)
          // so viewers can see notes on the finalised day.
          const canOpenSheet = joined && !!onOpenDay && (locked || isFinalised);
          const canInteract = canToggle || canOpenSheet;
          const onCellClick = () => {
            if (canToggle) {
              handleDayClick(dateStr, alreadySelected);
              return;
            }
            if (canOpenSheet && onOpenDay) {
              onOpenDay(dateStr);
            }
          };

          const noteAuthors = noteAuthorsByDate?.get(dateStr);
          const showNoteDot = !!noteAuthors && noteAuthors.size > 0;
          let noteDotColour: string | undefined;
          if (showNoteDot) {
            if (noteAuthors.size === 1) {
              const [onlyId] = noteAuthors;
              noteDotColour = sorted.find((p) => p.id === onlyId)?.colour;
            }
          }

          return (
            <button
              key={dateStr}
              type="button"
              data-testid="day-cell"
              ref={(el) => {
                if (el) cellRefs.current.set(dateStr, el);
                else cellRefs.current.delete(dateStr);
              }}
              className={classNames.join(" ")}
              onClick={onCellClick}
              disabled={!canInteract}
              style={cellStyle}
              aria-label={isFinalised ? `${ariaLabel} — finalised` : ariaLabel}
              title={ariaLabel}
            >
              {showMonth ? (
                <span className={styles.monthLabel}>{MONTH_SHORT[date.getMonth()]}</span>
              ) : null}
              <span className={styles.dayNumber}>{date.getDate()}</span>
              {showNoteDot ? (
                <span
                  className={styles.noteDot}
                  data-testid="note-dot"
                  aria-hidden="true"
                  style={
                    noteDotColour
                      ? ({ ["--note-dot-colour" as string]: noteDotColour } as React.CSSProperties)
                      : undefined
                  }
                />
              ) : null}
              {sorted.length > 0 ? (
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
                    const initial = participantInitial(p.name);
                    return (
                      <span
                        key={p.id}
                        className={isFree ? styles.slotFree : styles.slotFaint}
                        style={{ backgroundColor: p.colour }}
                        data-testid={`slot-${p.id}`}
                      >
                        {initial ? (
                          <span
                            className={styles.slotInitial}
                            style={{ color: textOnColour(p.colour) }}
                            aria-hidden="true"
                          >
                            {initial}
                          </span>
                        ) : null}
                      </span>
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

      {coachmarkAnchor ? (
        <CoachmarkPreferNote
          anchorEl={coachmarkAnchor}
          containerEl={containerRef.current}
          onDismiss={dismissCoachmark}
        />
      ) : null}
    </div>
  );
}
