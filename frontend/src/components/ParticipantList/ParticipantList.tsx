"use client";

import { useEffect, useRef, useState } from "react";
import type { Participant } from "@/lib/types";
import { MAX_PARTICIPANT_NAME_LENGTH } from "@/lib/limits";
import styles from "./ParticipantList.module.css";

interface ParticipantListProps {
  participants: Participant[];
  currentParticipantId: string | null;
  // Participants who have marked at least one date. Joining alone isn't
  // responding — an owner who created the calendar but hasn't marked any
  // dates shouldn't count. Omitted means nobody has responded yet.
  respondedIds?: ReadonlySet<string>;
  canRemove?: boolean;
  canRenameOthers?: boolean;
  onRemove?: (participant: Participant) => void;
  onRename?: (participant: Participant, newName: string) => Promise<void>;
}

interface RowProps {
  participant: Participant;
  isCurrent: boolean;
  canRemove: boolean;
  canRename: boolean;
  onRemove?: (p: Participant) => void;
  onRename?: (p: Participant, newName: string) => Promise<void>;
}

function ParticipantRow({
  participant: p,
  isCurrent,
  canRemove,
  canRename,
  onRemove,
  onRename,
}: RowProps) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(p.name);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const inputRef = useRef<HTMLInputElement | null>(null);
  const pencilRef = useRef<HTMLButtonElement | null>(null);
  const cancelledRef = useRef(false);

  useEffect(() => {
    if (editing) {
      inputRef.current?.focus();
      inputRef.current?.select();
    }
  }, [editing]);

  useEffect(() => {
    if (!editing) setDraft(p.name);
  }, [p.name, editing]);

  const startEdit = () => {
    setDraft(p.name);
    setError(null);
    cancelledRef.current = false;
    setEditing(true);
  };

  const exitEdit = () => {
    setEditing(false);
    setError(null);
    setTimeout(() => pencilRef.current?.focus(), 0);
  };

  const commit = async () => {
    if (!onRename || saving) return;
    const next = draft.trim();
    if (next === "" || next === p.name) {
      exitEdit();
      return;
    }
    setSaving(true);
    try {
      await onRename(p, next);
      setSaving(false);
      exitEdit();
    } catch (err) {
      setSaving(false);
      const msg =
        err && typeof err === "object" && "code" in err && err.code === "duplicate_name"
          ? "That name is already taken"
          : "Couldn't save. Try again.";
      setError(msg);
      // Keep edit mode open and refocus the input so the user can try again.
      setTimeout(() => {
        inputRef.current?.focus();
        inputRef.current?.select();
      }, 0);
    }
  };

  const handleBlur = () => {
    if (cancelledRef.current) {
      cancelledRef.current = false;
      return;
    }
    void commit();
  };

  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter") {
      e.preventDefault();
      void commit();
    } else if (e.key === "Escape") {
      e.preventDefault();
      cancelledRef.current = true;
      exitEdit();
    }
  };

  return (
    <li className={isCurrent ? `${styles.item} ${styles.current}` : styles.item}>
      <span className={styles.dot} style={{ backgroundColor: p.colour }} />
      {editing ? (
        <span className={styles.editWrap}>
          <input
            ref={inputRef}
            type="text"
            className={styles.nameInput}
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            onBlur={handleBlur}
            onKeyDown={handleKeyDown}
            maxLength={MAX_PARTICIPANT_NAME_LENGTH}
            aria-label={`Rename ${p.name}`}
            disabled={saving}
          />
          {error ? (
            <span className={styles.editError} role="alert">
              {error}
            </span>
          ) : null}
        </span>
      ) : (
        <span className={styles.name}>
          {p.name}
          {isCurrent ? " (you)" : ""}
        </span>
      )}
      {canRename && !editing ? (
        <button
          ref={pencilRef}
          type="button"
          className={styles.renameButton}
          onClick={startEdit}
          aria-label={`Rename ${p.name}`}
        >
          ✎
        </button>
      ) : null}
      {canRemove && onRemove && !editing ? (
        <button
          type="button"
          className={styles.removeButton}
          onClick={() => onRemove(p)}
          aria-label={`Remove ${p.name}`}
        >
          ×
        </button>
      ) : null}
    </li>
  );
}

export default function ParticipantList({
  participants,
  currentParticipantId,
  respondedIds,
  canRemove = false,
  canRenameOthers = false,
  onRemove,
  onRename,
}: ParticipantListProps) {
  const [expanded, setExpanded] = useState(false);

  if (participants.length === 0) return null;

  const respondedCount = respondedIds
    ? participants.filter((p) => respondedIds.has(p.id)).length
    : 0;

  return (
    <div className={styles.list}>
      <button
        type="button"
        className={styles.compactToggle}
        onClick={() => setExpanded((e) => !e)}
        aria-expanded={expanded}
      >
        <span className={styles.dots}>
          {participants.map((p) => (
            <span key={p.id} className={styles.dot} style={{ backgroundColor: p.colour }} />
          ))}
        </span>
        <span className={styles.compactLabel}>
          {respondedCount} of {participants.length} responded
        </span>
        <span className={styles.count}>{participants.length}</span>
      </button>

      <div className={`${styles.fullList} ${expanded ? styles.fullListExpanded : ""}`}>
        <h3 className={styles.heading}>Participants</h3>
        <ul className={styles.items}>
          {participants.map((p) => {
            const isCurrent = p.id === currentParticipantId;
            const canRename = !!onRename && (isCurrent || canRenameOthers);
            return (
              <ParticipantRow
                key={p.id}
                participant={p}
                isCurrent={isCurrent}
                canRemove={canRemove}
                canRename={canRename}
                onRemove={onRemove}
                onRename={onRename}
              />
            );
          })}
        </ul>
      </div>
    </div>
  );
}
