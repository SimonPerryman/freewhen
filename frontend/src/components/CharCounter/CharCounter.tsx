"use client";

import { useEffect, useState } from "react";
import styles from "./CharCounter.module.css";

// Remaining-character count at which the counter first appears.
export const COUNTER_THRESHOLD = 20;
// Remaining counts that are announced to screen readers.
const ANNOUNCE_AT = new Set([20, 10, 0]);

export function isAtLimit(count: number, max: number): boolean {
  return count >= max;
}

interface Props {
  count: number;
  max: number;
  id?: string;
  className?: string;
}

// Shared limit counter for text fields. Silent until COUNTER_THRESHOLD
// characters remain; at the limit it adds an icon and bold text so the state
// never relies on colour alone. Screen readers hear the remaining count at 20,
// 10 and 0, not on every keystroke.
export default function CharCounter({ count, max, id, className }: Props) {
  const remaining = Math.max(0, max - count);
  const visible = remaining <= COUNTER_THRESHOLD;
  const atLimit = isAtLimit(count, max);
  const [announcement, setAnnouncement] = useState("");

  useEffect(() => {
    if (!ANNOUNCE_AT.has(remaining)) return;
    setAnnouncement(remaining === 0 ? "Character limit reached" : `${remaining} characters left`);
  }, [remaining]);

  return (
    <>
      {visible ? (
        <span
          id={id}
          className={[atLimit ? styles.atLimit : styles.counter, className]
            .filter(Boolean)
            .join(" ")}
          aria-hidden="true"
        >
          {atLimit ? (
            <svg
              className={styles.icon}
              viewBox="0 0 16 16"
              width="14"
              height="14"
              focusable="false"
            >
              <path
                d="M8 1.5 15 14H1L8 1.5Z M8 6v4 M8 11.5v.5"
                fill="none"
                stroke="currentColor"
                strokeWidth="1.6"
                strokeLinecap="round"
                strokeLinejoin="round"
              />
            </svg>
          ) : null}
          {atLimit ? "Limit reached" : `${remaining} left`}
        </span>
      ) : null}
      <span className={styles.srOnly} role="status" aria-live="polite">
        {announcement}
      </span>
    </>
  );
}
