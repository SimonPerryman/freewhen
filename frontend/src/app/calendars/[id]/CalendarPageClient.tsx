"use client";

import { useState, useCallback } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import PassphraseGate from "@/components/PassphraseGate/PassphraseGate";
import analytics from "@/lib/analytics";
import { useCalendarEvents } from "@/lib/useCalendarEvents";
import { MAX_PARTICIPANTS_PER_CALENDAR } from "@/lib/limits";
import type { CalendarWithDetails } from "@/lib/types";
import { useAuth } from "@/components/AuthProvider/AuthProvider";
import { useCalendarData, type LoadError } from "@/lib/useCalendarData";
import { useParticipantIdentity } from "@/lib/useParticipantIdentity";
import { useAvailabilitySave } from "@/lib/useAvailabilitySave";
import { useCalendarRename } from "@/lib/useCalendarRename";
import { useCalendarOwnerActions } from "@/lib/useCalendarOwnerActions";
import { calendarMetaLabels, useAvailabilityMaps } from "@/lib/availabilityMaps";
import { localToday } from "@/lib/localDate";
import CalendarGrid from "@/components/CalendarGrid/CalendarGrid";
import CalendarHeader from "@/components/CalendarHeader/CalendarHeader";
import CalendarOwnerDialogs from "@/components/CalendarOwnerDialogs/CalendarOwnerDialogs";
import CalendarParticipantsSection from "@/components/CalendarParticipantsSection/CalendarParticipantsSection";
import DayDetailSheet from "@/components/DayDetailSheet/DayDetailSheet";
import DuplicateCalendarSheet from "@/components/DuplicateCalendarSheet/DuplicateCalendarSheet";
import FinalisedBanner from "@/components/FinalisedBanner/FinalisedBanner";
import { useImportFromGoogle } from "@/components/ImportFromGoogle/ImportFromGoogle";
import JoinForm from "@/components/JoinForm/JoinForm";
import BestDay from "@/components/BestDay/BestDay";
import WhosMissingPanel from "@/components/WhosMissingPanel/WhosMissingPanel";
import styles from "./page.module.css";

export default function CalendarPageClient({ calendarId }: { calendarId: string }) {
  const { user } = useAuth();
  const router = useRouter();
  const [duplicateOpen, setDuplicateOpen] = useState(false);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [openDayDate, setOpenDayDate] = useState<string | null>(null);

  // The three state machines are wired through callbacks: a full load
  // resolves who "you" are and seeds your availability; a live refresh syncs
  // your participant record and re-hydrates your notes / votes.
  const data = useCalendarData(calendarId, {
    user,
    onLoaded: async (fresh: CalendarWithDetails) => {
      const resolved = await identity.resolveFromLoad(fresh);
      if (resolved) availability.seed(fresh, resolved.id);
    },
    onRefreshed: (fresh: CalendarWithDetails) => {
      const currentId = identity.syncFromRefresh(fresh);
      if (currentId) availability.rehydrate(fresh, currentId);
    },
  });
  const { calendar, setCalendar, markLocalMutation } = data;

  const identity = useParticipantIdentity({
    calendarId,
    user,
    calendar,
    setCalendar,
    markLocalMutation,
  });
  const { currentParticipant } = identity;

  const finalisedDate = calendar?.calendar.finalisedDate ?? null;
  const availability = useAvailabilitySave({
    calendarId,
    participantIdRef: identity.participantIdRef,
    finalisedDate,
    markLocalMutation,
  });
  const { selectedDates, notesByDate, votesByDate, handleToggle } = availability;

  const rename = useCalendarRename({ calendarId, calendar, setCalendar, markLocalMutation });
  const owner = useCalendarOwnerActions({ calendarId, calendar, setCalendar, markLocalMutation });

  const { status: streamStatus } = useCalendarEvents(
    calendar ? calendarId : "",
    currentParticipant?.id ?? null,
    data.refresh,
  );

  const importView = useImportFromGoogle({
    calendarId,
    user,
    participantId: currentParticipant?.id ?? null,
  });
  const { clearSuggestion } = importView;

  const handleToggleWithSuggestions = useCallback(
    (date: string) => {
      clearSuggestion(date);
      handleToggle(date);
    },
    [clearSuggestion, handleToggle],
  );

  const handleOpenDay = useCallback(
    (date: string) => {
      setOpenDayDate(date);
      analytics.capture("day_sheet_opened", {
        calendar_id: calendarId,
        has_notes: (calendar?.availability ?? []).some(
          (a) => a.date === date && a.note && a.note.length > 0,
        ),
      });
    },
    [calendar, calendarId],
  );

  const handleCloseDay = useCallback(() => {
    setOpenDayDate(null);
  }, []);

  const handleRemoveAvailabilityForDay = useCallback(
    (date: string) => {
      handleToggle(date);
      setOpenDayDate(null);
    },
    [handleToggle],
  );

  const { freeByDate, voteCountsByDate, preferredByDate, noteAuthorsByDate, notesByDateAndAuthor } =
    useAvailabilityMaps(calendar, {
      participant: currentParticipant,
      selectedDates,
      votesByDate,
      notesByDate,
    });

  if (data.loading) {
    return (
      <div className={styles.page}>
        <div className={styles.container}>
          <p className={styles.loading}>Loading…</p>
        </div>
      </div>
    );
  }

  if (data.requiresPassphrase) {
    return (
      <div className={styles.page}>
        <div className={styles.container}>
          <PassphraseGate
            calendarId={calendarId}
            initialError={data.magicLinkError}
            onUnlocked={data.handleUnlocked}
          />
        </div>
      </div>
    );
  }

  if (data.error || !calendar) {
    const kind: LoadError = data.error ?? "not-found";
    const heading = kind === "not-found" ? "Calendar not found" : "Couldn't load this calendar";
    const sub =
      kind === "not-found"
        ? "The link may be wrong or the calendar may have been removed."
        : "Something went wrong on our end. Check your connection and try again.";

    return (
      <div className={styles.page}>
        <div className={styles.errorPanel}>
          <h1 className={styles.errorHeading}>{heading}</h1>
          <p className={styles.errorSub}>{sub}</p>
          {kind === "not-found" ? (
            <Link href="/" className={styles.errorCta}>
              Back to home
            </Link>
          ) : (
            <button type="button" className={styles.errorCta} onClick={data.retryLoad}>
              Try again
            </button>
          )}
        </div>
      </div>
    );
  }

  const calendarName = calendar.calendar.name || "Untitled Calendar";
  const calendarEmoji = calendar.calendar.emoji;
  const passphrase = calendar.calendar.passphrase ?? null;
  const isOwner = !!user && calendar.calendar.ownerId === user.id;
  const isFull = calendar.participants.length >= MAX_PARTICIPANTS_PER_CALENDAR;
  const { peopleLabel, dateLabel } = calendarMetaLabels(calendar, selectedDates, localToday());

  return (
    <div className={styles.page}>
      <div className={styles.container}>
        <CalendarHeader
          calendarName={calendarName}
          showSettings={isOwner && !finalisedDate}
          onOpenSettings={() => setSettingsOpen(true)}
          streamStatus={streamStatus}
          renameDraft={rename.renameDraft}
          renameInputRef={rename.renameInputRef}
          onRenameDraftChange={rename.setRenameDraft}
          onRenameCommit={rename.handleRenameCommit}
          onRenameCancel={rename.handleRenameCancel}
          peopleLabel={peopleLabel}
          dateLabel={dateLabel}
        />

        {finalisedDate ? (
          <FinalisedBanner
            date={finalisedDate}
            emoji={calendarEmoji || "📅"}
            calendarId={calendar.calendar.id}
            canUnfinalise={isOwner}
            onUnfinalise={owner.handleUnfinalise}
            participants={calendar.participants}
            availability={calendar.availability}
            voteCount={voteCountsByDate.get(finalisedDate) ?? 0}
          />
        ) : null}

        {!currentParticipant && !finalisedDate ? (
          isFull ? (
            <p className={styles.emptyHint}>
              This calendar is full ({MAX_PARTICIPANTS_PER_CALENDAR} people).
            </p>
          ) : (
            <JoinForm
              onJoin={identity.handleJoin}
              error={identity.joinError}
              defaultName={identity.storedName}
              hideEmail={!!user}
            />
          )
        ) : null}

        {!currentParticipant && finalisedDate ? (
          <p className={styles.emptyHint}>
            This calendar is finalised — no further changes needed.
          </p>
        ) : null}

        {availability.saveError ? (
          <div className={styles.banner} role="alert">
            <span>Save failed</span>
            <button type="button" className={styles.retryButton} onClick={availability.handleRetry}>
              Retry
            </button>
          </div>
        ) : null}

        {calendar.participants.length === 0 && !currentParticipant ? (
          <p className={styles.emptyHint}>
            Share the link — when people join, you&apos;ll see their availability here.
          </p>
        ) : null}

        {!finalisedDate ? (
          <BestDay
            participants={calendar.participants}
            freeByDate={freeByDate}
            voteCountsByDate={voteCountsByDate}
          />
        ) : null}

        {!finalisedDate && currentParticipant ? <div>{importView.entry}</div> : null}
        {importView.chrome}
        {importView.interstitial}
        <CalendarGrid
          calendarId={calendarId}
          participants={calendar.participants}
          freeByDate={freeByDate}
          noteAuthorsByDate={noteAuthorsByDate}
          voteCountsByDate={voteCountsByDate}
          currentParticipantId={currentParticipant?.id ?? null}
          suggestedFreeDates={importView.suggestedFree}
          suggestedBusyDates={importView.suggestedBusy}
          onToggle={handleToggleWithSuggestions}
          onOpenDay={currentParticipant ? handleOpenDay : undefined}
          onRangeToggle={availability.handleRangeToggle}
          onClear={currentParticipant ? availability.handleClearAvailability : undefined}
          canClear={availability.canClear}
          selectedDates={selectedDates}
          joined={!!currentParticipant}
          finalisedDate={finalisedDate}
        />

        {duplicateOpen ? (
          <DuplicateCalendarSheet
            calendarId={calendarId}
            sourceName={calendarName}
            onClose={() => setDuplicateOpen(false)}
            onDuplicated={(newId) => {
              setDuplicateOpen(false);
              router.push(`/calendars/${newId}`);
            }}
          />
        ) : null}

        {openDayDate && currentParticipant ? (
          <DayDetailSheet
            date={openDayDate}
            participants={calendar.participants}
            availability={calendar.availability}
            currentParticipantId={currentParticipant.id}
            currentNote={notesByDate.get(openDayDate) ?? ""}
            isSelected={selectedDates.has(openDayDate)}
            finalised={finalisedDate === openDayDate}
            isPreferred={votesByDate.has(openDayDate)}
            preferredParticipantIds={preferredByDate.get(openDayDate) ?? new Set()}
            onSaveNote={(text) => availability.handleSaveNote(openDayDate, text)}
            onToggleVote={(preferred) => availability.handleToggleVote(openDayDate, preferred)}
            onRemoveAvailability={() => handleRemoveAvailabilityForDay(openDayDate)}
            onClose={handleCloseDay}
          />
        ) : null}

        <CalendarParticipantsSection
          calendarId={calendarId}
          calendarName={calendarName}
          calendarEmoji={calendarEmoji}
          finalisedDate={finalisedDate}
          passphrase={passphrase}
          participants={calendar.participants}
          currentParticipantId={currentParticipant?.id ?? null}
          isOwner={isOwner}
          freeByDate={freeByDate}
          preferredByDate={preferredByDate}
          notesByDateAndAuthor={notesByDateAndAuthor}
          onFinalise={owner.handleFinalise}
          onOpenDay={handleOpenDay}
          onRemoveParticipant={owner.handleRemoveParticipant}
          onRenameParticipant={currentParticipant ? identity.handleRenameParticipant : undefined}
        />

        {isOwner && !finalisedDate ? (
          <WhosMissingPanel
            calendarId={calendar.calendar.id}
            participants={calendar.participants}
          />
        ) : null}

        <CalendarOwnerDialogs
          hasPassphrase={calendar.calendar.hasPassphrase ?? false}
          passphrase={passphrase}
          actions={owner}
          settingsOpen={settingsOpen}
          onCloseSettings={() => setSettingsOpen(false)}
          onStartRename={rename.handleRenameStart}
          onOpenDuplicate={() => setDuplicateOpen(true)}
        />
      </div>
    </div>
  );
}
