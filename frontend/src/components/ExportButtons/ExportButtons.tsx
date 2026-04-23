"use client";

import styles from "./ExportButtons.module.css";

const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

interface ExportButtonsProps {
  calendarId: string;
}

export default function ExportButtons({ calendarId }: ExportButtonsProps) {
  const googleHref = `${API_BASE}/calendars/${calendarId}/export/google`;
  const icsHref = `${API_BASE}/calendars/${calendarId}/export.ics`;

  return (
    <div className={styles.buttons}>
      <a
        className={styles.button}
        href={googleHref}
        target="_blank"
        rel="noopener noreferrer"
      >
        Add to Google Calendar
      </a>
      <a className={styles.button} href={icsHref} download>
        Download .ics
      </a>
    </div>
  );
}
