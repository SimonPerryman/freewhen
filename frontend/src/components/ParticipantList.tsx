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
  if (participants.length === 0) return null;

  return (
    <div className={styles.list}>
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
  );
}
