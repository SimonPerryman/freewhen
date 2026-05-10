"use client";

import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import type { MyCalendarListItem } from "@/lib/types";
import { formatFinalisedDate, formatRelative } from "@/lib/relativeTime";
import { deleteCalendar, leaveCalendar } from "@/lib/api";
import { useToast } from "@/lib/toast";
import log from "@/lib/log";
import DuplicateCalendarSheet from "@/components/DuplicateCalendarSheet/DuplicateCalendarSheet";
import RenameCalendarSheet from "@/components/RenameCalendarSheet/RenameCalendarSheet";
import RowMenu, { type RowMenuItem } from "@/components/RowMenu/RowMenu";
import styles from "./MyCalendarsList.module.css";

// Window during which Undo cancels the destructive action by simply not
// firing the API call. Matches the toast duration so they expire together.
const DESTRUCTIVE_UNDO_MS = 6000;

type Status = "loading" | "error" | "ready";

interface Props {
  status: Status;
  calendars: MyCalendarListItem[];
  hasMore: boolean;
  onRetry?: () => void;
}

export default function MyCalendarsList({ status, calendars, hasMore, onRetry }: Props) {
  const router = useRouter();
  const { showToast } = useToast();
  const [duplicating, setDuplicating] = useState<MyCalendarListItem | null>(null);
  const [renaming, setRenaming] = useState<MyCalendarListItem | null>(null);
  const [localNames, setLocalNames] = useState<Record<string, string>>({});
  // Calendars hidden locally pending the Undo window. Both Leave and Delete
  // use the same deferred-fire pattern: the row vanishes immediately, and
  // the API call only goes out if Undo isn't pressed within DESTRUCTIVE_UNDO_MS.
  const [hiddenIds, setHiddenIds] = useState<Set<string>>(new Set());
  const pendingTimers = useRef<Map<string, ReturnType<typeof setTimeout>>>(new Map());

  useEffect(() => {
    const timers = pendingTimers.current;
    return () => {
      // On unmount, fire any still-pending destructive actions immediately
      // so a user navigating away doesn't lose the action they triggered.
      timers.forEach((t) => {
        clearTimeout(t);
      });
    };
  }, []);

  const scheduleDestructive = (cal: MyCalendarListItem, kind: "leave" | "delete") => {
    setHiddenIds((prev) => {
      const next = new Set(prev);
      next.add(cal.id);
      return next;
    });
    const fire = async () => {
      pendingTimers.current.delete(cal.id);
      try {
        if (kind === "delete") await deleteCalendar(cal.id);
        else await leaveCalendar(cal.id);
      } catch (err) {
        log.warn("destructive_calendar_action_failed", { id: cal.id, kind, err });
        // Restore the row on failure so the user can retry.
        setHiddenIds((prev) => {
          const next = new Set(prev);
          next.delete(cal.id);
          return next;
        });
        showToast({
          message:
            kind === "delete"
              ? "Couldn't delete that calendar. Please try again."
              : "Couldn't leave that calendar. Please try again.",
        });
      }
    };
    const timer = setTimeout(fire, DESTRUCTIVE_UNDO_MS);
    pendingTimers.current.set(cal.id, timer);
    showToast({
      message:
        kind === "delete"
          ? `Deleted "${cal.name || "calendar"}"`
          : `Left "${cal.name || "calendar"}"`,
      durationMs: DESTRUCTIVE_UNDO_MS,
      action: {
        label: "Undo",
        onClick: () => {
          const t = pendingTimers.current.get(cal.id);
          if (t) {
            clearTimeout(t);
            pendingTimers.current.delete(cal.id);
          }
          setHiddenIds((prev) => {
            const next = new Set(prev);
            next.delete(cal.id);
            return next;
          });
        },
      },
    });
  };
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

  const visibleCalendars = calendars.filter((c) => !hiddenIds.has(c.id));

  if (visibleCalendars.length === 0) {
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
        {visibleCalendars.map((cal) => {
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
            items.push({
              id: "delete",
              label: "Delete",
              destructive: true,
              onSelect: () => scheduleDestructive(cal, "delete"),
            });
          } else {
            items.push({
              id: "leave",
              label: "Leave",
              destructive: true,
              onSelect: () => scheduleDestructive(cal, "leave"),
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
