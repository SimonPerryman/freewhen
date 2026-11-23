"use client";

import Button from "@/components/Button/Button";
import Sheet from "@/components/Sheet/Sheet";
import styles from "./CalendarSettingsSheet.module.css";

interface Props {
  hasPassphrase: boolean;
  passphrase?: string | null;
  onRename: () => void;
  onEnablePassphrase: () => void;
  onRegeneratePassphrase: () => void;
  onDisablePassphrase: () => void;
  onDuplicate: () => void;
  onDelete: () => void;
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
  onDelete,
  onClose,
}: Props) {
  return (
    <Sheet title="Settings" onClose={onClose} closeLabel="Close settings">
      <section className={styles.section}>
        <Button variant="secondary" onClick={onRename}>
          Rename calendar
        </Button>
      </section>

      <section className={styles.section}>
        <p className={styles.sectionLabel}>Passphrase</p>
        {hasPassphrase ? (
          <div className={styles.passphrase}>
            <code className={styles.passphraseValue}>{passphrase ?? "••••••••"}</code>
            <p className={styles.sectionHint}>
              Anyone with the share link opens the calendar without typing this. Give it out if you
              want someone to unlock by hand.
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
          <Button variant="secondary" onClick={onEnablePassphrase}>
            Require a passphrase
          </Button>
        )}
      </section>

      <section className={styles.section}>
        <Button variant="secondary" onClick={onDuplicate}>
          Duplicate this calendar
        </Button>
      </section>

      <section className={styles.section}>
        <button type="button" className={styles.deleteLink} onClick={onDelete}>
          Delete calendar
        </button>
      </section>
    </Sheet>
  );
}
