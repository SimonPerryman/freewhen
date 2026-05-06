"use client";

import {
  useCallback,
  useEffect,
  useId,
  useRef,
  useState,
  type FormEvent,
  type KeyboardEvent,
} from "react";
import { useAuth } from "@/components/AuthProvider/AuthProvider";
import { updateMe, logoutEverywhere, ApiError } from "@/lib/api";
import { MAX_USER_NAME_LENGTH } from "@/lib/limits";
import log from "@/lib/log";
import { useToast } from "@/lib/toast";
import styles from "./UserMenu.module.css";

const COUNTER_THRESHOLD = 10;

function initialFor(name: string | undefined, email: string): string {
  const source = (name ?? "").trim() || email;
  const ch = source.trim().charAt(0);
  return ch ? ch.toUpperCase() : "?";
}

export default function UserMenu() {
  const { user, setUser, signOut } = useAuth();
  const { showToast } = useToast();
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState(false);
  const [nameDraft, setNameDraft] = useState("");
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [signingOut, setSigningOut] = useState(false);
  const [revokingOthers, setRevokingOthers] = useState(false);

  const triggerRef = useRef<HTMLButtonElement>(null);
  const panelRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const panelId = useId();

  const close = useCallback(() => {
    setOpen(false);
    setEditing(false);
    setSaveError(null);
  }, []);

  useEffect(() => {
    if (!open) return;
    function onKey(event: globalThis.KeyboardEvent) {
      if (event.key === "Escape") {
        close();
        triggerRef.current?.focus();
      }
    }
    function onPointer(event: MouseEvent) {
      const target = event.target as Node | null;
      if (!target) return;
      if (panelRef.current?.contains(target)) return;
      if (triggerRef.current?.contains(target)) return;
      close();
    }
    document.addEventListener("keydown", onKey);
    document.addEventListener("mousedown", onPointer);
    return () => {
      document.removeEventListener("keydown", onKey);
      document.removeEventListener("mousedown", onPointer);
    };
  }, [open, close]);

  useEffect(() => {
    if (editing) {
      inputRef.current?.focus();
      inputRef.current?.select();
    }
  }, [editing]);

  if (!user) return null;

  const displayName = user.name?.trim() || "";
  const hasName = displayName.length > 0;
  const initial = initialFor(displayName, user.email);

  function beginEdit() {
    if (!user) return;
    setNameDraft(user.name ?? "");
    setSaveError(null);
    setEditing(true);
  }

  function cancelEdit() {
    setEditing(false);
    setSaveError(null);
  }

  async function handleSave(event?: FormEvent<HTMLFormElement>) {
    event?.preventDefault();
    if (!user) return;
    const trimmed = nameDraft.trim();
    if (!trimmed) {
      setSaveError("Name cannot be empty.");
      return;
    }
    if (Array.from(trimmed).length > MAX_USER_NAME_LENGTH) {
      setSaveError(`Keep it under ${MAX_USER_NAME_LENGTH} characters.`);
      return;
    }
    if (trimmed === (user.name ?? "").trim()) {
      setEditing(false);
      return;
    }
    setSaving(true);
    setSaveError(null);
    try {
      const updated = await updateMe({ name: trimmed });
      setUser(updated);
      setEditing(false);
    } catch (err) {
      if (err instanceof ApiError) {
        setSaveError(err.message || "Could not save name.");
      } else {
        log.error("update name failed", err instanceof Error ? err.message : String(err));
        setSaveError("Could not save name.");
      }
    } finally {
      setSaving(false);
    }
  }

  function onInputKeyDown(event: KeyboardEvent<HTMLInputElement>) {
    if (event.key === "Escape") {
      event.preventDefault();
      cancelEdit();
    }
  }

  async function handleSignOut() {
    setSigningOut(true);
    try {
      await signOut();
    } finally {
      setSigningOut(false);
    }
  }

  async function handleSignOutEverywhere() {
    setRevokingOthers(true);
    try {
      await logoutEverywhere();
      showToast({ message: "Signed out everywhere else." });
      close();
    } catch (err) {
      const msg =
        err instanceof ApiError
          ? err.message || "Could not sign out other devices."
          : "Could not sign out other devices.";
      log.error(
        "logout everywhere failed",
        err instanceof Error ? err.message : String(err),
      );
      showToast({ message: msg });
    } finally {
      setRevokingOthers(false);
    }
  }

  const showCounter = nameDraft.length >= MAX_USER_NAME_LENGTH - COUNTER_THRESHOLD;

  return (
    <div className={styles.root}>
      <button
        ref={triggerRef}
        type="button"
        className={styles.trigger}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-controls={panelId}
        aria-label={hasName ? `Account menu for ${displayName}` : "Account menu"}
        onClick={() => setOpen((v) => !v)}
      >
        <span className={styles.avatar} aria-hidden="true">
          {initial}
          {!hasName ? <span className={styles.pulse} /> : null}
        </span>
        <span className={styles.triggerName}>{hasName ? displayName : "You"}</span>
        <svg
          className={`${styles.chevron} ${open ? styles.chevronOpen : ""}`}
          width="12"
          height="12"
          viewBox="0 0 12 12"
          fill="none"
          aria-hidden="true"
        >
          <path
            d="M3 4.5L6 7.5L9 4.5"
            stroke="currentColor"
            strokeWidth="1.5"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
        </svg>
      </button>

      {open ? (
        <>
          <div className={styles.backdrop} aria-hidden="true" onClick={close} />
          <div
            id={panelId}
            ref={panelRef}
            className={styles.panel}
            role="menu"
            aria-label="Account"
          >
            <div className={styles.handle} aria-hidden="true" />
            <div className={styles.identity}>
              <span className={styles.avatarLarge} aria-hidden="true">
                {initial}
              </span>
              <div className={styles.identityText}>
                {editing ? (
                  <form className={styles.editForm} onSubmit={handleSave}>
                    <input
                      ref={inputRef}
                      type="text"
                      className={styles.nameInput}
                      value={nameDraft}
                      onChange={(e) => setNameDraft(e.target.value)}
                      onKeyDown={onInputKeyDown}
                      placeholder="Your name"
                      autoComplete="name"
                      maxLength={MAX_USER_NAME_LENGTH}
                      disabled={saving}
                      aria-label="Display name"
                    />
                    <div className={styles.editActions}>
                      <button
                        type="button"
                        className={styles.textButton}
                        onClick={cancelEdit}
                        disabled={saving}
                      >
                        Cancel
                      </button>
                      <button
                        type="submit"
                        className={styles.primaryButton}
                        disabled={saving || !nameDraft.trim()}
                      >
                        {saving ? "Saving…" : "Save"}
                      </button>
                    </div>
                    {showCounter ? (
                      <p className={styles.counter} aria-live="polite">
                        {nameDraft.length} / {MAX_USER_NAME_LENGTH}
                      </p>
                    ) : null}
                    {saveError ? (
                      <p role="alert" className={styles.error}>
                        {saveError}
                      </p>
                    ) : null}
                  </form>
                ) : (
                  <>
                    <button
                      type="button"
                      className={`${styles.nameRow} ${hasName ? "" : styles.nameRowEmpty}`}
                      onClick={beginEdit}
                    >
                      <span className={styles.name}>{hasName ? displayName : "Add your name"}</span>
                      <svg
                        width="14"
                        height="14"
                        viewBox="0 0 14 14"
                        fill="none"
                        aria-hidden="true"
                        className={styles.editIcon}
                      >
                        <path
                          d="M9.5 2L12 4.5L5 11.5H2.5V9L9.5 2Z"
                          stroke="currentColor"
                          strokeWidth="1.3"
                          strokeLinejoin="round"
                        />
                      </svg>
                    </button>
                    <span className={styles.email} title={user.email}>
                      {user.email}
                    </span>
                  </>
                )}
              </div>
            </div>

            <div className={styles.divider} aria-hidden="true" />

            <a href="/calendars" role="menuitem" className={styles.menuItem} onClick={close}>
              <svg width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true">
                <rect
                  x="2"
                  y="3.5"
                  width="12"
                  height="10.5"
                  rx="1.5"
                  stroke="currentColor"
                  strokeWidth="1.3"
                />
                <path
                  d="M2 6H14M5.5 2V4.5M10.5 2V4.5"
                  stroke="currentColor"
                  strokeWidth="1.3"
                  strokeLinecap="round"
                />
              </svg>
              My calendars
            </a>

            <button
              type="button"
              role="menuitem"
              className={styles.menuItem}
              onClick={handleSignOutEverywhere}
              disabled={revokingOthers}
            >
              <svg width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true">
                <path
                  d="M2 8C2 4.7 4.7 2 8 2C11.3 2 14 4.7 14 8C14 11.3 11.3 14 8 14"
                  stroke="currentColor"
                  strokeWidth="1.3"
                  strokeLinecap="round"
                />
                <path
                  d="M5 5L8 8L11 5"
                  stroke="currentColor"
                  strokeWidth="1.3"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                />
              </svg>
              {revokingOthers ? "Signing out other devices…" : "Sign out other devices"}
            </button>

            <button
              type="button"
              role="menuitem"
              className={styles.menuItem}
              onClick={handleSignOut}
              disabled={signingOut}
            >
              <svg width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true">
                <path
                  d="M9.5 2H12.5C13.05 2 13.5 2.45 13.5 3V13C13.5 13.55 13.05 14 12.5 14H9.5"
                  stroke="currentColor"
                  strokeWidth="1.3"
                  strokeLinecap="round"
                />
                <path
                  d="M6 5L2.5 8L6 11M2.5 8H10.5"
                  stroke="currentColor"
                  strokeWidth="1.3"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                />
              </svg>
              {signingOut ? "Signing out…" : "Sign out"}
            </button>
          </div>
        </>
      ) : null}
    </div>
  );
}
