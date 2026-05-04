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
  await page.getByRole("button", { name: "Create calendar" }).click();

  await expect(page).toHaveURL(/\/calendars\/[\w-]+$/);
  await expect(page.getByRole("heading", { level: 1, name: calendarName })).toBeVisible();

  // Owner is auto-joined as a participant; the local-part of the email is the default name.
  const localPart = email.split("@")[0];
  await expect(page.getByText(localPart, { exact: false }).first()).toBeVisible();
});
