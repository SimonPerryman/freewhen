"use client";

import { useEffect } from "react";
import styles from "./CalendarSettingsSheet.module.css";

interface Props {
  visibility: "open" | "submit_to_view";
  hasPassphrase: boolean;
  passphrase?: string | null;
  onVisibilityToggle: () => void;
  onEnablePassphrase: () => void;
  onRegeneratePassphrase: () => void;
  onDisablePassphrase: () => void;
  onDuplicate: () => void;
  onClose: () => void;
}

export default function CalendarSettingsSheet({
  visibility,
  hasPassphrase,
  passphrase,
  onVisibilityToggle,
  onEnablePassphrase,
  onRegeneratePassphrase,
  onDisablePassphrase,
  onDuplicate,
  onClose,
}: Props) {
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  return (
    <div
      className={styles.backdrop}
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div
        className={styles.sheet}
        role="dialog"
        aria-modal="true"
        aria-label="Calendar settings"
      >
        <div className={styles.header}>
          <h2 className={styles.heading}>Settings</h2>
          <button
            type="button"
            className={styles.closeButton}
            onClick={onClose}
            aria-label="Close settings"
          >
            ×
          </button>
        </div>

        <section className={styles.section}>
          <p className={styles.sectionLabel}>Visibility</p>
          <button
            type="button"
            className={styles.toggleRow}
            onClick={onVisibilityToggle}
            aria-pressed={visibility === "submit_to_view"}
          >
            <span>
              <span className={styles.toggleTitle}>
                Hide others until they respond
              </span>
              <span className={styles.toggleHint}>
                {visibility === "submit_to_view"
                  ? "On — people must mark a day before they see the grid."
                  : "Off — anyone with the link sees everyone's availability."}
              </span>
            </span>
            <span
              className={`${styles.switch} ${
                visibility === "submit_to_view" ? styles.switchOn : ""
              }`}
              aria-hidden="true"
            />
          </button>
        </section>

        <section className={styles.section}>
          <p className={styles.sectionLabel}>Passphrase</p>
          {hasPassphrase ? (
            <div className={styles.passphrase}>
              <code className={styles.passphraseValue}>
                {passphrase ?? "••••••••"}
              </code>
              <p className={styles.sectionHint}>
                Anyone with the share link opens the calendar without typing
                this. Give it out if you want someone to unlock by hand.
              </p>
              <div className={styles.destructiveRow}>
                <button
                  type="button"
                  className={styles.destructiveLink}
                  onClick={onRegeneratePassphrase}
                >
                  Regenerate
                </button>
                <span aria-hidden="true" className={styles.dot}>
                  ·
                </span>
                <button
                  type="button"
                  className={styles.destructiveLink}
                  onClick={onDisablePassphrase}
                >
                  Remove passphrase
                </button>
              </div>
            </div>
          ) : (
            <button
              type="button"
              className={styles.primaryAction}
              onClick={onEnablePassphrase}
            >
              Require a passphrase
            </button>
          )}
        </section>

        <section className={styles.section}>
          <button
            type="button"
            className={styles.primaryAction}
            onClick={onDuplicate}
          >
            Duplicate this calendar
          </button>
        </section>
      </div>
    </div>
  );
}
