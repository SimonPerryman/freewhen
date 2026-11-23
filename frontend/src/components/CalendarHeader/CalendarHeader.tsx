"use client";

import type { CSSProperties, MutableRefObject } from "react";
import LiveStatusDot from "@/components/LiveStatusDot/LiveStatusDot";
import { MAX_CALENDAR_NAME_LENGTH } from "@/lib/limits";
import type { Participant } from "@/lib/types";
import { renderTitle, titlePlain } from "@/lib/title";
import type { StreamStatus } from "@/lib/useCalendarEvents";
import styles from "./CalendarHeader.module.css";

interface Props {
  calendarName: string;
  // Show the owner's settings cog (hidden while renaming or once finalised).
  showSettings: boolean;
  onOpenSettings: () => void;
  streamStatus: StreamStatus;
  // null when not renaming.
  renameDraft: string | null;
  renameInputRef: MutableRefObject<HTMLInputElement | null>;
  onRenameDraftChange: (value: string) => void;
  onRenameCommit: () => void;
  onRenameCancel: () => void;
  participants: Participant[];
  // Participants who have marked at least one date.
  respondedIds: ReadonlySet<string>;
}

// Calendar page header: title (or the inline rename input), settings cog,
// live-updates dot, and the "N of M responded" readout.
export default function CalendarHeader({
  calendarName,
  showSettings,
  onOpenSettings,
  streamStatus,
  renameDraft,
  renameInputRef,
  onRenameDraftChange,
  onRenameCommit,
  onRenameCancel,
  participants,
  respondedIds,
}: Props) {
  return (
    <div className={styles.header}>
      <div className={styles.titleRow}>
        {renameDraft !== null ? (
          <input
            ref={renameInputRef}
            type="text"
            className={styles.headingInput}
            value={renameDraft}
            onChange={(e) => onRenameDraftChange(e.target.value)}
            onBlur={onRenameCommit}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                e.preventDefault();
                onRenameCommit();
              } else if (e.key === "Escape") {
                e.preventDefault();
                onRenameCancel();
              }
            }}
            aria-label="Calendar name"
            aria-describedby="calendar-name-help"
            maxLength={MAX_CALENDAR_NAME_LENGTH}
          />
        ) : (
          <h1 className={styles.heading} aria-label={titlePlain(calendarName)}>
            {renderTitle(calendarName)}
          </h1>
        )}
        {showSettings && renameDraft === null ? (
          <button
            type="button"
            className={styles.settingsButton}
            onClick={onOpenSettings}
            aria-label="Calendar settings"
          >
            <svg
              width="18"
              height="18"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
              aria-hidden="true"
            >
              <circle cx="12" cy="12" r="3" />
              <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" />
            </svg>
          </button>
        ) : null}
        <span className={styles.liveStatus}>
          <LiveStatusDot status={streamStatus} />
        </span>
      </div>
      {renameDraft !== null ? (
        <p id="calendar-name-help" className={styles.renameHint}>
          Wrap a word in *asterisks* to italicise it.
        </p>
      ) : null}
      {participants.length > 0 ? (
        <div className={styles.metaRow}>
          <span className={styles.dots} aria-hidden="true">
            {participants.map((p) => (
              <span
                key={p.id}
                className={`${styles.dot} ${respondedIds.has(p.id) ? "" : styles.dotPending}`}
                style={{ "--dot-colour": p.colour } as CSSProperties}
              />
            ))}
          </span>
          <span className={styles.responded}>
            {participants.filter((p) => respondedIds.has(p.id)).length} of {participants.length}{" "}
            responded
          </span>
        </div>
      ) : null}
    </div>
  );
}
