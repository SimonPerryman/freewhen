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

  // Owner settings sheet (shared Sheet primitive): focus lands inside, Escape
  // closes it.
  const settings = page.getByRole("dialog", { name: "Settings" });
  await page.getByRole("button", { name: "Calendar settings" }).click();
  await expect(settings).toBeVisible();
  await expect(settings.locator(":focus")).toHaveCount(1);
  await page.keyboard.press("Escape");
  await expect(settings).toBeHidden();

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
