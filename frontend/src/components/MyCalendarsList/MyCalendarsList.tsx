"use client";

import Link from "next/link";
import type { MyCalendarListItem } from "@/lib/types";
import { formatFinalisedDate, formatRelative } from "@/lib/relativeTime";
import styles from "./MyCalendarsList.module.css";

type Status = "loading" | "error" | "ready";

interface Props {
  status: Status;
  calendars: MyCalendarListItem[];
  hasMore: boolean;
  onRetry?: () => void;
}

export default function MyCalendarsList({ status, calendars, hasMore, onRetry }: Props) {
  if (status === "loading") {
    return (
      <div className={styles.skeleton} aria-busy="true" aria-label="Loading calendars">
        <div className={styles.skeletonCard} />
        <div className={styles.skeletonCard} />
        <div className={styles.skeletonCard} />
      </div>
    );
  }

  if (status === "error") {
    return (
      <div className={styles.error} role="alert">
        <div>Couldn&apos;t load your calendars.</div>
        {onRetry ? (
          <button type="button" className={styles.retryButton} onClick={onRetry}>
            Try again
          </button>
        ) : null}
      </div>
    );
  }

  if (calendars.length === 0) {
    return (
      <div className={styles.empty}>
        <div className={styles.emptyTitle}>
          You haven&apos;t created or joined any calendars yet.
        </div>
        <Link href="/" className={styles.emptyCta}>
          Create a calendar
        </Link>
      </div>
    );
  }

  return (
    <>
      <ul className={styles.list}>
        {calendars.map((cal) => (
          <li key={cal.id}>
            <Link href={`/calendars/${cal.id}`} className={styles.card}>
              <div className={styles.titleRow}>
                <span className={styles.emoji} aria-hidden="true">
                  {cal.emoji || "📅"}
                </span>
                <span className={styles.name}>{cal.name || "Untitled calendar"}</span>
                {cal.myRole === "owner" ? <span className={styles.rolePill}>Owner</span> : null}
              </div>
              <div className={styles.metaRow}>
                <div className={styles.metaLeft}>
                  <span>
                    {cal.participantCount} {cal.participantCount === 1 ? "person" : "people"}
                  </span>
                  {cal.finalisedDate ? (
                    <span className={styles.finalisedBadge}>
                      📅 {formatFinalisedDate(cal.finalisedDate)}
                    </span>
                  ) : null}
                </div>
                <span>Updated {formatRelative(cal.updatedAt)}</span>
              </div>
            </Link>
          </li>
        ))}
      </ul>
      {hasMore ? (
        <div className={styles.footer}>Showing your 100 most recent calendars.</div>
      ) : null}
    </>
  );
}
