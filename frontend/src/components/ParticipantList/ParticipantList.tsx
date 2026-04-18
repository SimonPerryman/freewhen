"use client";

import { useState } from "react";
import type { Participant } from "@/lib/types";
import styles from "./ParticipantList.module.css";

interface ParticipantListProps {
  participants: Participant[];
  currentParticipantId: string | null;
}

export default function ParticipantList({
  participants,
  currentParticipantId,
}: ParticipantListProps) {
  const [expanded, setExpanded] = useState(false);

  if (participants.length === 0) return null;

  return (
    <div className={styles.list}>
      <button
        type="button"
        className={styles.compactToggle}
        onClick={() => setExpanded((e) => !e)}
        aria-expanded={expanded}
      >
        <span className={styles.dots}>
          {participants.map((p) => (
            <span key={p.id} className={styles.dot} style={{ backgroundColor: p.colour }} />
          ))}
        </span>
        <span className={styles.compactLabel}>{participants.length} responded</span>
        <span className={styles.count}>{participants.length}</span>
      </button>

      <div className={`${styles.fullList} ${expanded ? styles.fullListExpanded : ""}`}>
        <h3 className={styles.heading}>Participants</h3>
        <ul className={styles.items}>
          {participants.map((p) => (
            <li
              key={p.id}
              className={
                p.id === currentParticipantId ? `${styles.item} ${styles.current}` : styles.item
              }
            >
              <span className={styles.dot} style={{ backgroundColor: p.colour }} />
              <span className={styles.name}>
                {p.name}
                {p.id === currentParticipantId ? " (you)" : ""}
              </span>
            </li>
          ))}
        </ul>
      </div>
    </div>
  );
}
