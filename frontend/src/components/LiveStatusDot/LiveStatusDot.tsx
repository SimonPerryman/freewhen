"use client";

import type { StreamStatus } from "@/lib/useCalendarEvents";
import styles from "./LiveStatusDot.module.css";

interface Props {
  status: StreamStatus;
}

// LiveStatusDot surfaces only the degraded live-update states. On "open" or
// "connecting" it renders nothing — the visible page content is the feedback.
export default function LiveStatusDot({ status }: Props) {
  if (status === "polling") {
    return (
      <span className={styles.root} role="status" aria-live="polite" title="Updates every 15s">
        <span className={`${styles.dot} ${styles.polling}`} aria-hidden="true" />
        Polling
      </span>
    );
  }
  if (status === "closed") {
    return (
      <span
        className={styles.root}
        role="status"
        aria-live="polite"
        title="Updates paused — refresh to retry"
      >
        <span className={`${styles.dot} ${styles.closed}`} aria-hidden="true" />
        Offline
      </span>
    );
  }
  return null;
}
