import ShareButton from "@/components/ShareButton/ShareButton";
import styles from "./InvitePrompt.module.css";

interface InvitePromptProps {
  calendarName: string;
}

export default function InvitePrompt({ calendarName }: InvitePromptProps) {
  return (
    <div className={styles.card}>
      <h3 className={styles.title}>Invite your friends</h3>
      <p className={styles.body}>
        Once they mark their days, we&apos;ll spotlight when everyone is free.
      </p>
      <div className={styles.action}>
        <ShareButton calendarName={calendarName} variant="hero" />
      </div>
    </div>
  );
}
