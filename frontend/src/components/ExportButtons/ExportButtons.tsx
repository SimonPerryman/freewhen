"use client";

import { API_BASE_URL } from "@/lib/config";
import styles from "./ExportButtons.module.css";

interface ExportButtonsProps {
  calendarId: string;
}

export default function ExportButtons({ calendarId }: ExportButtonsProps) {
  const googleHref = `${API_BASE_URL}/calendars/${calendarId}/export/google`;
  const icsHref = `${API_BASE_URL}/calendars/${calendarId}/export.ics`;

  return (
    <div className={styles.buttons}>
      <a className={styles.button} href={googleHref} target="_blank" rel="noopener noreferrer">
        Add to Google Calendar
      </a>
      <a className={styles.button} href={icsHref} download>
        Download .ics
      </a>
    </div>
  );
}
