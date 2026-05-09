"use client";

import { useFocusTrap } from "@/lib/useFocusTrap";
import styles from "./CalendarSettingsSheet.module.css";

interface Props {
  hasPassphrase: boolean;
  passphrase?: string | null;
  onRename: () => void;
  onEnablePassphrase: () => void;
  onRegeneratePassphrase: () => void;
  onDisablePassphrase: () => void;
  onDuplicate: () => void;
  onClose: () => void;
}

export default function CalendarSettingsSheet({
  hasPassphrase,
  passphrase,
  onRename,
  onEnablePassphrase,
  onRegeneratePassphrase,
  onDisablePassphrase,
  onDuplicate,
  onClose,
}: Props) {
  const sheetRef = useFocusTrap<HTMLDivElement>({ onEscape: onClose });

  return (
    <div
      className={styles.backdrop}
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div
        ref={sheetRef}
        className={styles.sheet}
        role="dialog"
        aria-modal="true"
        aria-labelledby="settings-sheet-title"
      >
        <div className={styles.header}>
          <h2 id="settings-sheet-title" className={styles.heading}>
            Settings
          </h2>
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
          <button
            type="button"
            className={styles.primaryAction}
            onClick={onRename}
          >
            Rename calendar →
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
              Require a passphrase →
            </button>
          )}
        </section>

        <section className={styles.section}>
          <button
            type="button"
            className={styles.primaryAction}
            onClick={onDuplicate}
          >
            Duplicate this calendar →
          </button>
        </section>
      </div>
    </div>
  );
}
