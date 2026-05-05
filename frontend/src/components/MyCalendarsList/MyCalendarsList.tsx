"use client";

import { useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import type { MyCalendarListItem } from "@/lib/types";
import { formatFinalisedDate, formatRelative } from "@/lib/relativeTime";
import DuplicateCalendarSheet from "@/components/DuplicateCalendarSheet/DuplicateCalendarSheet";
import RenameCalendarSheet from "@/components/RenameCalendarSheet/RenameCalendarSheet";
import RowMenu, { type RowMenuItem } from "@/components/RowMenu/RowMenu";
import styles from "./MyCalendarsList.module.css";

type Status = "loading" | "error" | "ready";

interface Props {
  status: Status;
  calendars: MyCalendarListItem[];
  hasMore: boolean;
  onRetry?: () => void;
}

export default function MyCalendarsList({ status, calendars, hasMore, onRetry }: Props) {
  const router = useRouter();
  const [duplicating, setDuplicating] = useState<MyCalendarListItem | null>(null);
  const [renaming, setRenaming] = useState<MyCalendarListItem | null>(null);
  const [localNames, setLocalNames] = useState<Record<string, string>>({});
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
        {calendars.map((cal) => {
          const displayName = localNames[cal.id] ?? cal.name;
          const items: RowMenuItem[] = [];
          if (cal.myRole === "owner") {
            items.push({
              id: "duplicate",
              label: "Duplicate",
              onSelect: () => setDuplicating(cal),
            });
            items.push({
              id: "rename",
              label: "Rename",
              onSelect: () => setRenaming(cal),
            });
          }
          return (
            <li key={cal.id} className={styles.row}>
              <Link href={`/calendars/${cal.id}`} className={styles.card}>
                <div className={styles.titleRow}>
                  <span className={styles.emoji} aria-hidden="true">
                    {cal.emoji || "📅"}
                  </span>
                  <span className={styles.name}>{displayName || "Untitled calendar"}</span>
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
              {items.length > 0 ? (
                <RowMenu items={items} triggerLabel={`Actions for ${displayName || "calendar"}`} />
              ) : null}
            </li>
          );
        })}
      </ul>
      {hasMore ? (
        <div className={styles.footer}>Showing your 100 most recent calendars.</div>
      ) : null}
      {duplicating ? (
        <DuplicateCalendarSheet
          calendarId={duplicating.id}
          sourceName={duplicating.name || "Untitled calendar"}
          onClose={() => setDuplicating(null)}
          onDuplicated={(newId) => {
            setDuplicating(null);
            router.push(`/calendars/${newId}`);
          }}
        />
      ) : null}
      {renaming ? (
        <RenameCalendarSheet
          calendarId={renaming.id}
          currentName={renaming.name}
          onClose={() => setRenaming(null)}
          onRenamed={(newName) => {
            setLocalNames((prev) => ({ ...prev, [renaming.id]: newName }));
            setRenaming(null);
          }}
        />
      ) : null}
    </>
  );
}
