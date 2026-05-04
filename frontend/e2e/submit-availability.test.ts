import { test, expect } from "./fixtures";

test("anonymous joiner picks a date, adds a note, marks it preferred, and reload persists", async ({
  page,
  api,
  calendar,
}) => {
  await page.goto(`/calendars/${calendar.id}`);

  const joinerName = `Picker ${Date.now()}`;
  await page.getByLabel("Your name").fill(joinerName);
  await page.getByRole("button", { name: "Join" }).click();

  await expect(page.getByText(joinerName).first()).toBeVisible();

  // Pick the first enabled, non-placeholder day cell. Real cells are <button>
  // with data-testid="day-cell"; placeholders are aria-hidden <div>s with the
  // same testid.
  const dayCells = page.locator('button[data-testid="day-cell"]:not([disabled])');
  await expect(dayCells.first()).toBeVisible();
  const firstCell = dayCells.first();

  // First click marks the cell free.
  await firstCell.click();

  // Second click on the now-selected cell opens the day-detail sheet. The
  // sheet's aria-labelledby points at its <h2>, which formats the date in a
  // different style than the cell's aria-label — match by role only.
  await firstCell.click();
  const sheet = page.getByRole("dialog");
  await expect(sheet).toBeVisible();

  // Note + preferred. The "Save" button in the sheet only enables once the
  // textarea differs from the empty currentNote.
  const note = "only after 6pm";
  await sheet.getByLabel("Your note").fill(note);
  await sheet.getByRole("checkbox", { name: /Prefer this date/ }).check();
  await sheet.getByRole("button", { name: "Save" }).click();

  // Sheet closes (Save calls handleSave then onClose).
  await expect(sheet).toBeHidden();

  // Reload the page and assert the picks survived a round-trip.
  await page.reload();
  await expect(page.getByText(joinerName).first()).toBeVisible();

  // Verify via API that availability + note + preferred all landed.
  const res = await api.get(`/calendars/${calendar.id}`);
  expect(res.ok()).toBeTruthy();
  const body = (await res.json()) as {
    participants: Array<{ id: string; name: string }>;
    availability: Array<{
      participant_id: string;
      date: string;
      note: string | null;
      preferred: boolean;
    }>;
  };
  const me = body.participants.find((p) => p.name === joinerName);
  expect(me).toBeDefined();
  const myAvail = body.availability.filter((a) => a.participant_id === me!.id);
  expect(myAvail.length, "should have one availability row").toBe(1);
  expect(myAvail[0].note).toBe(note);
  expect(myAvail[0].preferred).toBe(true);
});
