"use client";

import { useEffect, useState } from "react";
import { getCalendarAccessToken } from "@/lib/calendarAccess";
import { API_BASE_URL } from "@/lib/config";
import styles from "./ExportButtons.module.css";

interface ExportButtonsProps {
  calendarId: string;
}

export default function ExportButtons({ calendarId }: ExportButtonsProps) {
  // Export links are plain navigations, which can't send the
  // X-Calendar-Access header — so for passphrase-protected calendars the
  // access token rides along as `?access=` (same as the SSE stream). Read
  // after mount: the token lives in localStorage, unavailable during SSR.
  const [accessToken, setAccessToken] = useState<string | null>(null);
  useEffect(() => {
    setAccessToken(getCalendarAccessToken(calendarId));
  }, [calendarId]);

  const query = accessToken ? `?access=${encodeURIComponent(accessToken)}` : "";
  const googleHref = `${API_BASE_URL}/calendars/${calendarId}/export/google${query}`;
  const icsHref = `${API_BASE_URL}/calendars/${calendarId}/export.ics${query}`;

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
