"use client";

import { useEffect, useState } from "react";
import styles from "./Toast.module.css";

export interface ToastAction {
  label: string;
  onClick: () => void;
}

export interface ToastProps {
  message: string;
  action?: ToastAction;
  onDismiss: () => void;
  onActionClick?: () => void;
  durationMs: number;
  // Unique id changes per showToast() call so the enter animation re-fires
  // when a toast replaces another.
  toastKey: string | number;
}

// Polite live region the toast renders into. The provider keeps this mounted
// for its whole lifetime: screen readers only announce changes *inside* an
// existing live region, so mounting the region together with its content
// (as we used to) meant toasts were frequently never announced.
export function ToastRegion({ children }: { children?: React.ReactNode }) {
  return (
    <div className={styles.root} role="status" aria-live="polite" aria-atomic="true">
      {children}
    </div>
  );
}

export default function Toast({
  message,
  action,
  onDismiss,
  onActionClick,
  durationMs,
  toastKey,
}: ToastProps) {
  const [visible, setVisible] = useState(false);
  const [hovered, setHovered] = useState(false);
  const [focused, setFocused] = useState(false);
  // Pause auto-dismiss while the pointer is over the toast or focus is inside
  // it (keyboard users tabbing to "Undo" shouldn't have it vanish).
  const paused = hovered || focused;

  useEffect(() => {
    // Trigger enter transition on next frame so CSS sees the state change.
    const id = requestAnimationFrame(() => setVisible(true));
    return () => cancelAnimationFrame(id);
  }, [toastKey]);

  useEffect(() => {
    if (paused || durationMs <= 0) return;
    const t = setTimeout(onDismiss, durationMs);
    return () => clearTimeout(t);
  }, [onDismiss, durationMs, paused, toastKey]);

  return (
    <div
      className={`${styles.toast} ${visible ? styles.visible : ""}`}
      data-testid="toast"
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
      onFocus={() => setFocused(true)}
      onBlur={(e) => {
        if (!e.currentTarget.contains(e.relatedTarget as Node | null)) setFocused(false);
      }}
    >
      <span className={styles.message}>{message}</span>
      {action ? (
        <button
          type="button"
          className={styles.action}
          onClick={() => {
            onActionClick?.();
            action.onClick();
            onDismiss();
          }}
        >
          {action.label}
        </button>
      ) : null}
    </div>
  );
}
