"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import {
  addInvites as apiAddInvites,
  deleteInvite as apiDeleteInvite,
  getMissing,
  matchInvite as apiMatchInvite,
  nudgeCalendar,
  renameInvite as apiRenameInvite,
} from "@/lib/api";
import analytics from "@/lib/analytics";
import { useToast } from "@/lib/toast";
import type { Invite, MissingView, Participant } from "@/lib/types";
import styles from "./WhosMissingPanel.module.css";

interface Props {
  calendarId: string;
  participants: Participant[];
  refreshKey?: number;
}

const STORAGE_KEY_PREFIX = "freewhen.whos-missing.open:";

function storageKey(calendarId: string) {
  return STORAGE_KEY_PREFIX + calendarId;
}

function readOpenState(calendarId: string): boolean {
  if (typeof window === "undefined") return false;
  try {
    return window.localStorage.getItem(storageKey(calendarId)) === "1";
  } catch {
    return false;
  }
}

function writeOpenState(calendarId: string, open: boolean) {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.setItem(storageKey(calendarId), open ? "1" : "0");
  } catch {
    // Quota or privacy-mode — ignore.
  }
}

export default function WhosMissingPanel({ calendarId, participants, refreshKey }: Props) {
  const { showToast } = useToast();
  const [open, setOpen] = useState(false);
  const [view, setView] = useState<MissingView | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [inputValue, setInputValue] = useState("");
  const [bulkMode, setBulkMode] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [submitNote, setSubmitNote] = useState<string | null>(null);
  const [linkMenuFor, setLinkMenuFor] = useState<string | null>(null);
  const [nudging, setNudging] = useState(false);

  const handleNudge = useCallback(async () => {
    if (nudging) return;
    setNudging(true);
    try {
      const result = await nudgeCalendar(calendarId);
      analytics.capture("calendar_nudge_sent", {
        calendar_id: calendarId,
        emailed: result.emailed,
        skipped: result.skipped,
      });
      const parts = [`Emailed ${result.emailed} ${result.emailed === 1 ? "person" : "people"}`];
      if (result.skipped > 0) {
        parts.push(`${result.skipped} have no email — copy the calendar link to share with them.`);
      }
      showToast({ message: parts.join(" · ") });
    } catch (err) {
      showToast({ message: err instanceof Error ? err.message : "Couldn't send reminder" });
    } finally {
      setNudging(false);
    }
  }, [calendarId, nudging, showToast]);

  useEffect(() => {
    setOpen(readOpenState(calendarId));
  }, [calendarId]);

  const reload = useCallback(async () => {
    try {
      const data = await getMissing(calendarId);
      setView(data);
      setLoadError(null);
    } catch (err) {
      setLoadError(err instanceof Error ? err.message : "Couldn't load who's missing");
    }
  }, [calendarId]);

  useEffect(() => {
    if (!open) return;
    void reload();
  }, [open, reload, refreshKey]);

  const toggleOpen = useCallback(() => {
    setOpen((prev) => {
      const next = !prev;
      writeOpenState(calendarId, next);
      if (next) {
        analytics.capture("invites_panel_opened", { calendar_id: calendarId });
      }
      return next;
    });
  }, [calendarId]);

  const missingCount = useMemo(() => {
    if (!view) return 0;
    return view.notJoined.length + view.joinedNotResponded.length;
  }, [view]);

  const handleCopyName = useCallback(
    async (name: string) => {
      try {
        await navigator.clipboard.writeText(name);
        showToast({ message: `Copied "${name}" — paste into your group chat.` });
        analytics.capture("invite_copied_to_clipboard", { calendar_id: calendarId });
      } catch {
        showToast({ message: `Couldn't copy — name is "${name}"` });
      }
    },
    [calendarId, showToast],
  );

  const handleSubmitAdd = useCallback(async () => {
    const raw = inputValue.trim();
    if (!raw || submitting) return;
    setSubmitting(true);
    setSubmitNote(null);
    try {
      // The server splits on commas and newlines; send the raw paste so the
      // server can report back skipped/invalid line-by-line.
      const names = [raw];
      const res = await apiAddInvites(calendarId, names);
      const count = res.added.length;
      analytics.capture("invite_added", {
        calendar_id: calendarId,
        bulk: count > 1,
        count,
      });
      const notes: string[] = [];
      if (res.skipped.length > 0) {
        notes.push(`${res.skipped.length} already on the list`);
      }
      if (res.invalid.length > 0) {
        notes.push(`${res.invalid.length} invalid (${res.invalid[0].reason})`);
      }
      setSubmitNote(notes.length > 0 ? notes.join(" · ") : null);
      setInputValue("");
      setBulkMode(false);
      await reload();
    } catch (err) {
      const msg = err instanceof Error ? err.message : "Couldn't add invites";
      setSubmitNote(msg);
    } finally {
      setSubmitting(false);
    }
  }, [calendarId, inputValue, reload, submitting]);

  const handleDelete = useCallback(
    async (invite: Invite) => {
      try {
        await apiDeleteInvite(calendarId, invite.id);
        analytics.capture("invite_deleted", { calendar_id: calendarId });
        await reload();
      } catch (err) {
        showToast({ message: err instanceof Error ? err.message : "Couldn't delete" });
      }
    },
    [calendarId, reload, showToast],
  );

  const handleRename = useCallback(
    async (invite: Invite) => {
      const next = window.prompt("Rename invite", invite.name);
      if (!next || next.trim() === invite.name) return;
      try {
        await apiRenameInvite(calendarId, invite.id, next);
        await reload();
      } catch (err) {
        showToast({ message: err instanceof Error ? err.message : "Couldn't rename" });
      }
    },
    [calendarId, reload, showToast],
  );

  const handleLink = useCallback(
    async (invite: Invite, participantId: string | null) => {
      try {
        await apiMatchInvite(calendarId, invite.id, participantId);
        if (participantId !== null) {
          analytics.capture("invite_matched_manual", { calendar_id: calendarId });
        }
        setLinkMenuFor(null);
        await reload();
      } catch (err) {
        showToast({ message: err instanceof Error ? err.message : "Couldn't link" });
      }
    },
    [calendarId, reload, showToast],
  );

  const matchedInvites = useMemo(() => {
    if (!view) return [];
    return view.invites.filter((inv) => inv.matchedParticipantId !== null);
  }, [view]);

  return (
    <section className={styles.panel} aria-label="Who's missing?">
      <button type="button" className={styles.toggle} onClick={toggleOpen} aria-expanded={open}>
        <span className={styles.toggleLabel}>Who&apos;s missing?</span>
        {missingCount > 0 ? (
          <span className={styles.badge} aria-label={`${missingCount} missing`}>
            {missingCount}
          </span>
        ) : null}
        <span className={styles.chevron} aria-hidden="true">
          {open ? "▾" : "▸"}
        </span>
      </button>

      {open ? (
        <div className={styles.body}>
          {loadError ? (
            <p className={styles.error} role="alert">
              {loadError}
            </p>
          ) : null}

          {view && view.invites.length === 0 && view.joinedNotResponded.length === 0 ? (
            <p className={styles.hint}>Add names to track who still needs to respond.</p>
          ) : null}

          {view &&
          view.notJoined.length === 0 &&
          view.joinedNotResponded.length === 0 &&
          view.invites.length > 0 ? (
            <p className={styles.hint}>No-one&apos;s missing 🎉</p>
          ) : null}

          {view && view.notJoined.length > 0 ? (
            <div className={styles.group}>
              <h4 className={styles.groupTitle}>Not joined</h4>
              <ul className={styles.chipList}>
                {view.notJoined.map((entry) => {
                  const invite = view.invites.find((i) => i.id === entry.inviteId);
                  if (!invite) return null;
                  return (
                    <li key={entry.inviteId} className={styles.chipWrap}>
                      <button
                        type="button"
                        className={styles.chip}
                        onClick={() => handleCopyName(entry.name)}
                        aria-label={`Copy ${entry.name}`}
                      >
                        {entry.name}
                      </button>
                      <div className={styles.chipMenu}>
                        <button
                          type="button"
                          className={styles.chipAction}
                          onClick={() => handleRename(invite)}
                        >
                          Rename
                        </button>
                        <button
                          type="button"
                          className={styles.chipAction}
                          onClick={() =>
                            setLinkMenuFor((cur) => (cur === invite.id ? null : invite.id))
                          }
                        >
                          Link…
                        </button>
                        <button
                          type="button"
                          className={styles.chipAction}
                          onClick={() => handleDelete(invite)}
                        >
                          Delete
                        </button>
                      </div>
                      {linkMenuFor === invite.id ? (
                        <div className={styles.linkMenu} role="menu">
                          {participants.length === 0 ? (
                            <p className={styles.menuHint}>No participants yet.</p>
                          ) : (
                            participants.map((p) => (
                              <button
                                key={p.id}
                                type="button"
                                className={styles.menuItem}
                                onClick={() => handleLink(invite, p.id)}
                              >
                                {p.name}
                              </button>
                            ))
                          )}
                        </div>
                      ) : null}
                    </li>
                  );
                })}
              </ul>
            </div>
          ) : null}

          {view && view.joinedNotResponded.length > 0 ? (
            <div className={styles.group}>
              <h4 className={styles.groupTitle}>Joined but not responded</h4>
              <ul className={styles.chipList}>
                {view.joinedNotResponded.map((entry) => (
                  <li key={entry.participantId} className={styles.chipWrap}>
                    <button
                      type="button"
                      className={`${styles.chip} ${styles.chipQuiet}`}
                      onClick={() => handleCopyName(entry.name)}
                      aria-label={`Copy ${entry.name}`}
                    >
                      {entry.name}
                    </button>
                  </li>
                ))}
              </ul>
            </div>
          ) : null}

          <div className={styles.addBlock}>
            {bulkMode ? (
              <textarea
                className={styles.textarea}
                value={inputValue}
                onChange={(e) => setInputValue(e.target.value)}
                placeholder={"Paste names — comma or newline separated"}
                rows={4}
              />
            ) : (
              <input
                type="text"
                className={styles.input}
                value={inputValue}
                onChange={(e) => setInputValue(e.target.value)}
                placeholder="Add an invite"
              />
            )}
            <div className={styles.addActions}>
              <button
                type="button"
                className={styles.addButton}
                onClick={handleSubmitAdd}
                disabled={submitting || inputValue.trim() === ""}
              >
                {submitting ? "Adding…" : "Add"}
              </button>
              <button
                type="button"
                className={styles.linkLike}
                onClick={() => setBulkMode((v) => !v)}
              >
                {bulkMode ? "Single name" : "Paste a list"}
              </button>
            </div>
            {submitNote ? <p className={styles.note}>{submitNote}</p> : null}
          </div>

          {missingCount > 0 ? (
            <div className={styles.addBlock}>
              <button
                type="button"
                className={styles.addButton}
                onClick={handleNudge}
                disabled={nudging}
              >
                {nudging ? "Sending…" : "Send reminder"}
              </button>
            </div>
          ) : null}

          {matchedInvites.length > 0 ? (
            <details className={styles.allInvited}>
              <summary>All invited ({matchedInvites.length})</summary>
              <ul className={styles.chipList}>
                {matchedInvites.map((inv) => (
                  <li key={inv.id} className={styles.chipWrap}>
                    <span className={`${styles.chip} ${styles.chipMatched}`}>{inv.name}</span>
                    <button
                      type="button"
                      className={styles.chipAction}
                      onClick={() => handleLink(inv, null)}
                    >
                      Unlink
                    </button>
                  </li>
                ))}
              </ul>
            </details>
          ) : null}
        </div>
      ) : null}
    </section>
  );
}
