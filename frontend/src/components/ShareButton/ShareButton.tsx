"use client";

import { useState, useEffect, useCallback } from "react";
import analytics from "@/lib/analytics";
import styles from "./ShareButton.module.css";

interface ShareButtonProps {
  calendarName: string;
  variant?: "default" | "hero";
}

export default function ShareButton({ calendarName, variant = "default" }: ShareButtonProps) {
  const [canShare, setCanShare] = useState(false);
  const [canCopy, setCanCopy] = useState(false);
  const [feedback, setFeedback] = useState<string | null>(null);

  useEffect(() => {
    setCanShare(typeof navigator !== "undefined" && !!navigator.share);
    setCanCopy(typeof navigator !== "undefined" && !!navigator.clipboard);
  }, []);

  const handleClick = useCallback(async () => {
    const url = window.location.href;

    if (canShare) {
      try {
        await navigator.share({
          title: calendarName,
          text: "Mark your availability on FreeWhen",
          url,
        });
        analytics.capture("invite_shared", { method: "native" });
        return; // share sheet handles its own feedback
      } catch (err) {
        // User cancelled or share failed — fall through to clipboard
        if (err instanceof Error && err.name === "AbortError") return;
      }
    }

    if (canCopy) {
      try {
        await navigator.clipboard.writeText(url);
        analytics.capture("invite_shared", { method: "clipboard" });
        setFeedback("Copied!");
        setTimeout(() => setFeedback(null), 2000);
      } catch {
        // clipboard write failed — ignore silently
      }
    }
  }, [canShare, canCopy, calendarName]);

  if (!canShare && !canCopy) return null;

  return (
    <button
      type="button"
      className={`${styles.button} ${variant === "hero" ? styles.hero : styles.default}`}
      onClick={handleClick}
    >
      <svg
        className={styles.icon}
        width="16"
        height="16"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
        aria-hidden="true"
      >
        {canShare ? (
          <>
            <path d="M4 12v8a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-8" />
            <polyline points="16 6 12 2 8 6" />
            <line x1="12" y1="2" x2="12" y2="15" />
          </>
        ) : (
          <>
            <path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71" />
            <path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71" />
          </>
        )}
      </svg>
      {feedback ?? (canShare ? "Share" : "Copy link")}
    </button>
  );
}
