"use client";

import { useEffect, useState } from "react";
import styles from "./CoachmarkPreferNote.module.css";

interface CoachmarkPreferNoteProps {
  anchorEl: HTMLElement | null;
  containerEl: HTMLElement | null;
  onDismiss: () => void;
}

interface Position {
  top: number;
  left: number;
  arrowX: number;
}

const TOOLTIP_WIDTH = 220;

export default function CoachmarkPreferNote({
  anchorEl,
  containerEl,
  onDismiss,
}: CoachmarkPreferNoteProps) {
  const [pos, setPos] = useState<Position | null>(null);

  useEffect(() => {
    if (!anchorEl || !containerEl) {
      setPos(null);
      return;
    }
    const compute = () => {
      const cellRect = anchorEl.getBoundingClientRect();
      const cardRect = containerEl.getBoundingClientRect();
      const top = cellRect.bottom - cardRect.top + 10;
      let left = cellRect.left - cardRect.left + cellRect.width / 2 - TOOLTIP_WIDTH / 2;
      const maxLeft = containerEl.clientWidth - TOOLTIP_WIDTH - 8;
      left = Math.max(8, Math.min(left, maxLeft));
      const arrowX = cellRect.left - cardRect.left + cellRect.width / 2 - left - 6;
      setPos({ top, left, arrowX });
    };
    compute();
    window.addEventListener("resize", compute);
    window.addEventListener("scroll", compute, true);
    return () => {
      window.removeEventListener("resize", compute);
      window.removeEventListener("scroll", compute, true);
    };
  }, [anchorEl, containerEl]);

  useEffect(() => {
    if (!anchorEl) return;
    const handlePointer = (e: MouseEvent | TouchEvent) => {
      const target = e.target as Node | null;
      if (target && (anchorEl.contains(target) || isInsideCoachmark(target))) return;
      onDismiss();
    };
    document.addEventListener("mousedown", handlePointer);
    document.addEventListener("touchstart", handlePointer);
    return () => {
      document.removeEventListener("mousedown", handlePointer);
      document.removeEventListener("touchstart", handlePointer);
    };
  }, [anchorEl, onDismiss]);

  if (!pos) return null;

  return (
    <div
      className={styles.coachmark}
      role="status"
      aria-live="polite"
      data-testid="coachmark-prefer-note"
      style={{
        top: `${pos.top}px`,
        left: `${pos.left}px`,
        ["--arrow-x" as string]: `${pos.arrowX}px`,
      }}
    >
      <div className={styles.title}>
        Tap again to <em>prefer</em>
      </div>
      <div className={styles.body}>
        Mark top picks or leave a note (e.g. "after 6pm"). Helps the group land on a date everyone
        wants.
      </div>
      <button type="button" className={styles.dismiss} onClick={onDismiss}>
        Got it
      </button>
    </div>
  );
}

function isInsideCoachmark(node: Node): boolean {
  let n: Node | null = node;
  while (n && n instanceof Element) {
    if (n.getAttribute("data-testid") === "coachmark-prefer-note") return true;
    n = n.parentNode;
  }
  return false;
}
