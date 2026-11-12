import { test, expect } from "./fixtures";

test("creator signs in, creates a calendar, lands on the calendar page", async ({
  page,
  signIn,
}) => {
  const { email } = await signIn(page);

  await page.goto("/");

  const nameField = page.getByLabel("Event name");
  await expect(nameField).toBeVisible();

  const calendarName = `Trip ${Date.now()}`;
  await nameField.fill(calendarName);
  await page.getByRole("button", { name: /^Create/ }).click();

  await expect(page).toHaveURL(/\/calendars\/[\w-]+$/);
  await expect(page.getByRole("heading", { level: 1, name: calendarName })).toBeVisible();

  // Owner is auto-joined as a participant; the local-part of the email is the default name.
  const localPart = email.split("@")[0];
  await expect(page.getByText(localPart, { exact: false }).first()).toBeVisible();

  // Owner settings sheet (shared Sheet primitive), opened from the keyboard:
  // focus lands inside, Escape closes it and focus returns to the opener.
  // This is the first open on the page, so the overlay's lazy chunk loads
  // now — it must not suspend the whole page and blur the opener.
  const settings = page.getByRole("dialog", { name: "Settings" });
  const settingsButton = page.getByRole("button", { name: "Calendar settings" });
  await settingsButton.focus();
  await page.keyboard.press("Enter");
  await expect(settings).toBeVisible();
  await expect(settings.locator(":focus")).toHaveCount(1);
  await page.keyboard.press("Escape");
  await expect(settings).toBeHidden();
  await expect(settingsButton).toBeFocused();

  // A confirm dialog stacked on the settings sheet: Escape closes only the
  // dialog, leaving the sheet open with focus back on the button that opened
  // the dialog.
  await settingsButton.click();
  await settings.getByRole("button", { name: /Require a passphrase/ }).click();
  const regenerate = settings.getByRole("button", { name: "Regenerate" });
  await regenerate.click();
  const confirm = page.getByRole("alertdialog", { name: "Regenerate passphrase?" });
  await expect(confirm).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(confirm).toBeHidden();
  await expect(settings).toBeVisible();
  await expect(regenerate).toBeFocused();
  // Close with the ✕ button: focus still returns to the opener.
  await settings.getByRole("button", { name: "Close settings" }).click();
  await expect(settings).toBeHidden();
  await expect(settingsButton).toBeFocused();

  // The duplicate sheet has the same ✕ close affordance as the others.
  await settingsButton.click();
  await settings.getByRole("button", { name: /Duplicate this calendar/ }).click();
  const duplicate = page.getByRole("dialog", { name: "Duplicate this calendar" });
  await expect(duplicate).toBeVisible();
  await duplicate.getByRole("button", { name: "Close duplicate calendar" }).click();
  await expect(duplicate).toBeHidden();
  await expect(settingsButton).toBeFocused();

  // Rename from the sheet, and it survives a reload.
  await page.getByRole("button", { name: "Calendar settings" }).click();
  await settings.getByRole("button", { name: /Rename calendar/ }).click();
  const renamed = `${calendarName} renamed`;
  await page.getByLabel("Calendar name").fill(renamed);
  const saved = page.waitForResponse(
    (r) => /\/calendars\/[\w-]+$/.test(r.url()) && r.request().method() !== "GET",
  );
  await page.getByLabel("Calendar name").press("Enter");
  await expect(page.getByRole("heading", { level: 1, name: renamed })).toBeVisible();
  expect((await saved).ok()).toBeTruthy();
  await page.reload();
  await expect(page.getByRole("heading", { level: 1, name: renamed })).toBeVisible();
});
