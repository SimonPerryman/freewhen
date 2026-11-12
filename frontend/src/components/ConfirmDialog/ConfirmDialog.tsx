"use client";

import { useId, useRef } from "react";
import Button from "@/components/Button/Button";
import Sheet from "@/components/Sheet/Sheet";
import styles from "./ConfirmDialog.module.css";

interface Props {
  title: string;
  message: string;
  confirmLabel?: string;
  cancelLabel?: string;
  destructive?: boolean;
  onConfirm: () => void;
  onCancel: () => void;
}

export default function ConfirmDialog({
  title,
  message,
  confirmLabel = "Confirm",
  cancelLabel = "Cancel",
  destructive = false,
  onConfirm,
  onCancel,
}: Props) {
  const confirmRef = useRef<HTMLButtonElement>(null);
  const bodyId = useId();

  return (
    <Sheet
      layout="dialog"
      role="alertdialog"
      title={title}
      describedBy={bodyId}
      onClose={onCancel}
      initialFocus={confirmRef}
    >
      <p id={bodyId} className={styles.body}>
        {message}
      </p>
      <div className={styles.actions}>
        <Button variant="secondary" onClick={onCancel}>
          {cancelLabel}
        </Button>
        <Button ref={confirmRef} variant={destructive ? "danger" : "secondary"} onClick={onConfirm}>
          {confirmLabel}
        </Button>
      </div>
    </Sheet>
  );
}
