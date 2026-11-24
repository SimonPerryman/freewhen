import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import CalendarSettingsSheet from "./CalendarSettingsSheet";
import { PICKABLE_EMOJIS } from "@/lib/emoji";

function renderSheet(emoji: string | undefined, onEmojiChange = vi.fn()) {
  render(
    <CalendarSettingsSheet
      emoji={emoji}
      onEmojiChange={onEmojiChange}
      hasPassphrase={false}
      onRename={() => {}}
      onEnablePassphrase={() => {}}
      onRegeneratePassphrase={() => {}}
      onDisablePassphrase={() => {}}
      onDuplicate={() => {}}
      onDelete={() => {}}
      onClose={() => {}}
    />,
  );
  return onEmojiChange;
}

describe("CalendarSettingsSheet emoji picker", () => {
  it("offers every pickable emoji as a radio with the current one checked", () => {
    renderSheet("🦊");
    const group = screen.getByRole("group", { name: "Emoji" });
    expect(group).toBeInTheDocument();
    const radios = screen.getAllByRole("radio");
    expect(radios).toHaveLength(PICKABLE_EMOJIS.length);
    expect(screen.getByRole("radio", { name: "🦊" })).toBeChecked();
    expect(screen.getByRole("radio", { name: "🎉" })).not.toBeChecked();
  });

  it("calls onEmojiChange with the chosen emoji", async () => {
    const user = userEvent.setup();
    const onEmojiChange = renderSheet("🦊");
    await user.click(screen.getByRole("radio", { name: "📚" }));
    expect(onEmojiChange).toHaveBeenCalledWith("📚");
  });

  it("checks nothing when the calendar has no emoji from the set", () => {
    renderSheet(undefined);
    for (const radio of screen.getAllByRole("radio")) expect(radio).not.toBeChecked();
  });
});
