import { test, expect } from "./fixtures";

test("owner changes the calendar emoji in settings, with Undo", async ({
  page,
  api,
  signIn,
  calendar,
}) => {
  const before = await api.get(`/calendars/${calendar.id}`);
  const original = ((await before.json()) as { calendar: { emoji: string } }).calendar.emoji;
  const target = original === "📚" ? "🎲" : "📚";

  await signIn(page, calendar.ownerEmail);
  await page.goto(`/calendars/${calendar.id}`);
  await page.getByRole("button", { name: "Calendar settings" }).click();
  const settings = page.getByRole("dialog", { name: "Settings" });
  await expect(settings.getByRole("radio", { name: original })).toBeChecked();

  await settings.getByRole("radio", { name: target }).click();
  await expect(settings.getByRole("radio", { name: target })).toBeChecked();
  await expect(page.getByText(`Emoji changed to ${target}`)).toBeVisible();

  const stored = async () =>
    ((await (await api.get(`/calendars/${calendar.id}`)).json()) as { calendar: { emoji: string } })
      .calendar.emoji;
  await expect.poll(stored).toBe(target);

  // Undo puts the previous emoji back.
  await page.getByRole("button", { name: "Undo" }).click();
  await expect.poll(stored).toBe(original);
  await expect(settings.getByRole("radio", { name: original })).toBeChecked();
});
