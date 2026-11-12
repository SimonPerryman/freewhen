"use client";

import InvitePrompt from "@/components/InvitePrompt/InvitePrompt";
import ParticipantList from "@/components/ParticipantList/ParticipantList";
import ShareButton from "@/components/ShareButton/ShareButton";
import SummaryView from "@/components/SummaryView/SummaryView";
import type { Participant } from "@/lib/types";
import styles from "./CalendarParticipantsSection.module.css";

interface Props {
  calendarId: string;
  calendarName: string;
  calendarEmoji?: string;
  finalisedDate: string | null;
  passphrase: string | null;
  participants: Participant[];
  currentParticipantId: string | null;
  isOwner: boolean;
  freeByDate: Map<string, Set<string>>;
  preferredByDate: Map<string, Set<string>>;
  notesByDateAndAuthor: Map<string, Map<string, string>>;
  onFinalise: (date: string) => void;
  onOpenDay: (date: string) => void;
  onRemoveParticipant: (target: Participant) => void;
  // Undefined when the viewer hasn't joined (renaming needs an identity).
  onRenameParticipant?: (target: Participant, newName: string) => Promise<void>;
}

// Everything below the grid that depends on how far along the calendar is:
// finalised → share + list; 2+ people → summary card + list; otherwise the
// invite prompt (plus the list once one person has joined).
export default function CalendarParticipantsSection({
  calendarId,
  calendarName,
  calendarEmoji,
  finalisedDate,
  passphrase,
  participants,
  currentParticipantId,
  isOwner,
  freeByDate,
  preferredByDate,
  notesByDateAndAuthor,
  onFinalise,
  onOpenDay,
  onRemoveParticipant,
  onRenameParticipant,
}: Props) {
  const list = (
    <ParticipantList
      participants={participants}
      currentParticipantId={currentParticipantId}
      canRemove={isOwner}
      canRenameOthers={isOwner}
      onRemove={onRemoveParticipant}
      onRename={onRenameParticipant}
    />
  );

  if (finalisedDate) {
    if (participants.length < 1) return null;
    return (
      <>
        <ShareButton
          calendarName={calendarName}
          calendarEmoji={calendarEmoji}
          finalisedDate={finalisedDate}
          passphrase={passphrase}
          state="finalised"
        />
        {list}
      </>
    );
  }

  if (participants.length >= 2) {
    return (
      <>
        <div className={styles.summaryCard}>
          <SummaryView
            participants={participants}
            freeByDate={freeByDate}
            preferredByDate={preferredByDate}
            notesByDateAndAuthor={notesByDateAndAuthor}
            calendarId={calendarId}
            canFinalise={isOwner}
            onFinalise={onFinalise}
            onOpenDay={onOpenDay}
          />
          <ShareButton
            calendarName={calendarName}
            calendarEmoji={calendarEmoji}
            finalisedDate={finalisedDate}
            passphrase={passphrase}
            variant="hero"
          />
        </div>
        {list}
      </>
    );
  }

  return (
    <>
      <InvitePrompt
        calendarName={calendarName}
        calendarEmoji={calendarEmoji}
        finalisedDate={finalisedDate}
        passphrase={passphrase}
      />
      {participants.length === 1 ? list : null}
    </>
  );
}
