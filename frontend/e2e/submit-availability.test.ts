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

  // A second date: mark it free, then flip "Prefer" rapidly. Each toggle is
  // its own request; the last one must win after a reload (vote-toggle race).
  const secondCell = dayCells.nth(1);
  await secondCell.click();
  await secondCell.click();
  await expect(sheet).toBeVisible();
  // Focus starts inside the sheet and Tab can't escape it.
  await expect(sheet.locator(":focus")).toHaveCount(1);
  for (let i = 0; i < 8; i++) await page.keyboard.press("Tab");
  await expect(sheet.locator(":focus")).toHaveCount(1);
  const prefer = sheet.getByRole("checkbox", { name: /Prefer this date/ });
  for (let i = 0; i < 5; i++) await prefer.click(); // odd count → ends checked
  await expect(prefer).toBeChecked();
  // Escape dismisses the sheet.
  await page.keyboard.press("Escape");
  await expect(sheet).toBeHidden();

  // Verify via API that availability + note + preferred all landed. Saves are
  // serialized client-side, so the toggles drain one request at a time —
  // poll until they settle; the last toggle must win.
  const myAvailability = async () => {
    const res = await api.get(`/calendars/${calendar.id}`);
    expect(res.ok()).toBeTruthy();
    const body = (await res.json()) as {
      participants: Array<{ id: string; name: string }>;
      availability: Array<{
        participant_id: string;
        date: string;
        note: string | null;
        preferred?: boolean;
      }>;
    };
    const me = body.participants.find((p) => p.name === joinerName);
    expect(me).toBeDefined();
    return body.availability
      .filter((a) => a.participant_id === me!.id)
      .sort((a, b) => a.date.localeCompare(b.date));
  };
  await expect
    .poll(async () => (await myAvailability()).map((a) => [a.note ?? null, a.preferred ?? false]))
    .toEqual([
      [note, true],
      [null, true],
    ]);

  // Reload the page and assert the picks survived a round-trip.
  await page.reload();
  await expect(page.getByText(joinerName).first()).toBeVisible();

  // And the UI agrees after the reload.
  await secondCell.click();
  await expect(sheet.getByRole("checkbox", { name: /Prefer this date/ })).toBeChecked();
});
