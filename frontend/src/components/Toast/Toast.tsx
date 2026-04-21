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

export default function Toast({
  message,
  action,
  onDismiss,
  onActionClick,
  durationMs,
  toastKey,
}: ToastProps) {
  const [visible, setVisible] = useState(false);
  const [paused, setPaused] = useState(false);

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
    <div className={styles.root} role="status" aria-live="polite">
      <div
        className={`${styles.toast} ${visible ? styles.visible : ""}`}
        data-testid="toast"
        onMouseEnter={() => setPaused(true)}
        onMouseLeave={() => setPaused(false)}
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
    </div>
  );
}
