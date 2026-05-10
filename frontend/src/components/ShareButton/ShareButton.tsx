"use client";

import { useState, useEffect, useCallback } from "react";
import analytics from "@/lib/analytics";
import { buildShareMessage } from "@/lib/share";
import styles from "./ShareButton.module.css";

interface ShareButtonProps {
  calendarName: string;
  calendarEmoji?: string | null;
  finalisedDate?: string | null;
  passphrase?: string | null;
  variant?: "default" | "hero";
  state?: "pre-finalisation" | "finalised";
}

export default function ShareButton({
  calendarName,
  calendarEmoji = null,
  finalisedDate = null,
  passphrase = null,
  variant = "default",
  state = "pre-finalisation",
}: ShareButtonProps) {
  const [canShare, setCanShare] = useState(false);
  const [canCopy, setCanCopy] = useState(false);
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    setCanShare(typeof navigator !== "undefined" && !!navigator.share);
    setCanCopy(typeof navigator !== "undefined" && !!navigator.clipboard);
  }, []);

  const handleClick = useCallback(async () => {
    let url = window.location.href;
    if (passphrase) {
      const u = new URL(window.location.href);
      u.searchParams.set("unlock", passphrase);
      url = u.toString();
    }

    if (canShare) {
      try {
        await navigator.share({
          title: calendarName,
          text: buildShareMessage({
            name: calendarName,
            emoji: calendarEmoji,
            finalisedDate,
          }),
          url,
        });
        analytics.capture("share_link_clicked", { method: "native", context: state });
        return;
      } catch (err) {
        if (err instanceof Error && err.name === "AbortError") return;
      }
    }

    if (canCopy) {
      try {
        await navigator.clipboard.writeText(url);
        analytics.capture("share_link_clicked", { method: "clipboard", context: state });
        setCopied(true);
        setTimeout(() => setCopied(false), 1800);
      } catch {
        // clipboard write failed — ignore silently
      }
    }
  }, [canShare, canCopy, calendarName, calendarEmoji, finalisedDate, passphrase, state]);

  if (!canShare && !canCopy) return null;

  if (variant === "hero") {
    return (
      <button type="button" className={`${styles.button} ${styles.hero}`} onClick={handleClick}>
        {copied ? "Copied!" : canShare ? "Share" : "Copy link"}
      </button>
    );
  }

  const kicker = state === "finalised" ? "Tell everyone the date" : "Invite the rest of the group";
  const chipLabel = copied ? "Copied!" : "Share";

  return (
    <button
      type="button"
      className={`${styles.card} ${copied ? styles.copied : ""}`}
      onClick={handleClick}
    >
      <span className={styles.text}>
        <span className={styles.kicker}>{kicker}</span>
        <span className={styles.headline}>Send the link to the group</span>
      </span>
      <span className={styles.chip}>{chipLabel}</span>
    </button>
  );
}
