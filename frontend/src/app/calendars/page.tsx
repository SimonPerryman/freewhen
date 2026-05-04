"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { ApiError, listMyCalendars } from "@/lib/api";
import analytics from "@/lib/analytics";
import { useAuth } from "@/components/AuthProvider/AuthProvider";
import MyCalendarsList from "@/components/MyCalendarsList/MyCalendarsList";
import type { MyCalendarListItem } from "@/lib/types";
import styles from "./page.module.css";

type Status = "loading" | "error" | "ready";

export default function MyCalendarsPage() {
  const router = useRouter();
  const { isAuthenticated, isLoading: authLoading, checkAuth } = useAuth();
  const [status, setStatus] = useState<Status>("loading");
  const [calendars, setCalendars] = useState<MyCalendarListItem[]>([]);
  const [hasMore, setHasMore] = useState(false);
  const [analyticsSent, setAnalyticsSent] = useState(false);

  useEffect(() => {
    checkAuth().then((u) => {
      if (!u) {
        router.replace("/login?return_url=/calendars");
      }
    });
  }, [checkAuth, router]);

  const load = useCallback(async () => {
    setStatus("loading");
    try {
      const res = await listMyCalendars();
      setCalendars(res.calendars);
      setHasMore(res.hasMore);
      setStatus("ready");
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        router.replace("/login?return_url=/calendars");
        return;
      }
      setStatus("error");
    }
  }, [router]);

  useEffect(() => {
    if (!isAuthenticated) return;
    void load();
  }, [isAuthenticated, load]);

  useEffect(() => {
    if (status !== "ready" || analyticsSent) return;
    analytics.capture("my_calendars_viewed", { count: calendars.length });
    if (calendars.length === 0) {
      analytics.capture("my_calendars_empty");
    }
    setAnalyticsSent(true);
  }, [status, analyticsSent, calendars.length]);

  if (authLoading && !isAuthenticated) {
    return (
      <div className={styles.page}>
        <main id="main" className={styles.container}>
          <h1 className={styles.heading}>My calendars</h1>
          <MyCalendarsList status="loading" calendars={[]} hasMore={false} />
        </main>
      </div>
    );
  }

  if (!isAuthenticated) {
    return null;
  }

  return (
    <div className={styles.page}>
      <main id="main" className={styles.container}>
        <h1 className={styles.heading}>My calendars</h1>
        <MyCalendarsList
          status={status}
          calendars={calendars}
          hasMore={hasMore}
          onRetry={() => void load()}
        />
      </main>
    </div>
  );
}
