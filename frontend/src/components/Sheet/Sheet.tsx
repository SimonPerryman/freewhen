"use client";

import { useId, type ReactNode, type RefObject } from "react";
import { useFocusTrap } from "@/lib/useFocusTrap";
import styles from "./Sheet.module.css";

interface SheetProps {
  // Rendered inside the panel's <h2> and used as its accessible name. Pass
  // a styled <span> for a one-off title treatment.
  title: ReactNode;
  onClose: () => void;
  children: ReactNode;
  // "sheet": bottom sheet on phones, centred card from 640px.
  // "dialog": compact centred card at every width, stacked above sheets.
  layout?: "sheet" | "dialog";
  role?: "dialog" | "alertdialog";
  // Id of an element inside the panel that describes it.
  describedBy?: string;
  // When false, Escape / backdrop click / the close button do nothing (e.g.
  // while a form is submitting). Focus stays trapped either way.
  dismissible?: boolean;
  // Element to focus on open. Defaults to the first focusable element,
  // which is the close button when one is shown.
  initialFocus?: RefObject<HTMLElement | null>;
  // Shows a ✕ close button in the header with this accessible label.
  closeLabel?: string;
  className?: string;
}

export default function Sheet({
  title,
  onClose,
  children,
  layout = "sheet",
  role = "dialog",
  describedBy,
  dismissible = true,
  initialFocus,
  closeLabel,
  className,
}: SheetProps) {
  const titleId = useId();
  const panelRef = useFocusTrap<HTMLDivElement>({
    onEscape: dismissible ? onClose : undefined,
    initialFocus,
  });

  return (
    <div
      className={`${styles.backdrop} ${styles[layout]}`}
      role="presentation"
      onClick={(e) => {
        if (e.target === e.currentTarget && dismissible) onClose();
      }}
    >
      <div
        ref={panelRef}
        className={[styles.panel, className].filter(Boolean).join(" ")}
        role={role}
        aria-modal="true"
        aria-labelledby={titleId}
        aria-describedby={describedBy}
      >
        <div className={styles.header}>
          <h2 id={titleId} className={styles.title}>
            {title}
          </h2>
          {closeLabel ? (
            <button
              type="button"
              className={styles.close}
              onClick={onClose}
              disabled={!dismissible}
              aria-label={closeLabel}
            >
              ✕
            </button>
          ) : null}
        </div>
        {children}
      </div>
    </div>
  );
}
