"use client";

import CalendarSettingsSheet from "@/components/CalendarSettingsSheet/CalendarSettingsSheet";
import ConfirmDialog from "@/components/ConfirmDialog/ConfirmDialog";
import type { UseCalendarOwnerActionsResult } from "@/lib/useCalendarOwnerActions";

interface Props {
  hasPassphrase: boolean;
  passphrase: string | null;
  actions: UseCalendarOwnerActionsResult;
  settingsOpen: boolean;
  onCloseSettings: () => void;
  onStartRename: () => void;
  onOpenDuplicate: () => void;
}

// The owner's overlays: settings sheet and the confirm dialogs
// for destructive actions. Each renders only while open.
export default function CalendarOwnerDialogs({
  hasPassphrase,
  passphrase,
  actions,
  settingsOpen,
  onCloseSettings,
  onStartRename,
  onOpenDuplicate,
}: Props) {
  return (
    <>
      {actions.removePassphraseOpen ? (
        <ConfirmDialog
          title="Remove passphrase?"
          message="The calendar will be openable without it. You can set a new one later."
          confirmLabel="Remove passphrase"
          cancelLabel="Keep it"
          destructive
          onConfirm={actions.confirmDisablePassphrase}
          onCancel={actions.cancelDisablePassphrase}
        />
      ) : null}

      {actions.regeneratePassphraseOpen ? (
        <ConfirmDialog
          title="Regenerate passphrase?"
          message="The old passphrase stops working immediately. Anyone using the old link will need the new one."
          confirmLabel="Regenerate"
          cancelLabel="Cancel"
          destructive
          onConfirm={actions.confirmRegeneratePassphrase}
          onCancel={actions.cancelRegeneratePassphrase}
        />
      ) : null}

      {actions.participantToRemove ? (
        <ConfirmDialog
          title={`Remove ${actions.participantToRemove.name}?`}
          message="Their availability will be cleared from the calendar."
          confirmLabel="Remove"
          cancelLabel="Cancel"
          destructive
          onConfirm={actions.confirmRemoveParticipant}
          onCancel={actions.cancelRemoveParticipant}
        />
      ) : null}

      {settingsOpen ? (
        <CalendarSettingsSheet
          hasPassphrase={hasPassphrase}
          passphrase={passphrase}
          onRename={() => {
            onCloseSettings();
            onStartRename();
          }}
          onEnablePassphrase={actions.handleEnablePassphrase}
          onRegeneratePassphrase={actions.handleRegeneratePassphrase}
          onDisablePassphrase={actions.handleDisablePassphrase}
          onDuplicate={() => {
            onCloseSettings();
            onOpenDuplicate();
          }}
          onClose={onCloseSettings}
        />
      ) : null}
    </>
  );
}
