"use client";

import { useCallback, useEffect, useId, useRef, useState } from "react";
import analytics from "@/lib/analytics";
import styles from "./PreferencePill.module.css";

export interface PreferencePillVoter {
  id: string;
  name: string;
  colour: string;
}

interface Props {
  voters: PreferencePillVoter[];
  calendarId?: string;
  date?: string;
}

const MAX_VISIBLE_SWATCHES = 4;
const LONG_NAME_THRESHOLD = 16;

export default function PreferencePill({ voters, calendarId, date }: Props) {
  const [open, setOpen] = useState(false);
  const triggerRef = useRef<HTMLButtonElement | null>(null);
  const popoverRef = useRef<HTMLDivElement | null>(null);
  const popoverId = useId();

  const close = useCallback(() => setOpen(false), []);

  useEffect(() => {
    if (!open) return;

    const handleKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.preventDefault();
        close();
        triggerRef.current?.focus();
      }
    };

    const handlePointer = (e: PointerEvent) => {
      const t = e.target as Node | null;
      if (!t) return;
      if (popoverRef.current?.contains(t)) return;
      if (triggerRef.current?.contains(t)) return;
      close();
    };

    document.addEventListener("keydown", handleKey);
    document.addEventListener("pointerdown", handlePointer);
    return () => {
      document.removeEventListener("keydown", handleKey);
      document.removeEventListener("pointerdown", handlePointer);
    };
  }, [open, close]);

  if (voters.length === 0) return null;

  // n=1 with a short name renders as a static pill. Long names fall through to
  // the popover path so the row stays bounded and the full name can wrap.
  const inlineSingle = voters.length === 1 && voters[0].name.length <= LONG_NAME_THRESHOLD;

  if (inlineSingle) {
    const v = voters[0];
    return (
      <span className={styles.pillStatic} title={v.name}>
        <span
          aria-hidden="true"
          className={styles.swatch}
          style={{ backgroundColor: v.colour }}
        />
        <span className={styles.label}>
          <span className={styles.name}>{v.name}</span> prefers
        </span>
      </span>
    );
  }

  const visible = voters.slice(0, MAX_VISIBLE_SWATCHES);

  function toggle(e: React.MouseEvent) {
    e.preventDefault();
    e.stopPropagation();
    setOpen((prev) => {
      const next = !prev;
      if (next && calendarId) {
        analytics.capture("preference_popover_opened", {
          calendar_id: calendarId,
          voter_count: voters.length,
          ...(date ? { date } : {}),
        });
      }
      return next;
    });
  }

  return (
    <span className={styles.wrap}>
      <button
        ref={triggerRef}
        type="button"
        className={styles.pillButton}
        aria-haspopup="true"
        aria-expanded={open}
        aria-controls={open ? popoverId : undefined}
        aria-label={
          voters.length === 1
            ? `1 person prefers this day. Show who.`
            : `${voters.length} people prefer this day. Show who.`
        }
        onClick={toggle}
      >
        <span aria-hidden="true" className={styles.swatchStack}>
          {visible.map((v) => (
            <span
              key={v.id}
              className={styles.swatch}
              style={{ backgroundColor: v.colour }}
            />
          ))}
        </span>
        <span className={styles.label}>
          {voters.length === 1 ? `1 prefers` : `${voters.length} prefer`}
        </span>
      </button>
      {open ? (
        <div
          ref={popoverRef}
          id={popoverId}
          className={styles.popover}
          role="group"
          aria-label="People who prefer this day"
        >
          <ul className={styles.list}>
            {voters.map((v) => (
              <li key={v.id} className={styles.row}>
                <span
                  aria-hidden="true"
                  className={styles.swatch}
                  style={{ backgroundColor: v.colour }}
                />
                <span className={styles.rowName}>{v.name}</span>
              </li>
            ))}
          </ul>
        </div>
      ) : null}
    </span>
  );
}
