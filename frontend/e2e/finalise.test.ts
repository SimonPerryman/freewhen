import { test, expect } from "./fixtures";

const BACKEND_URL = process.env.E2E_BACKEND_URL ?? "http://localhost:8090";

function tomorrowISO(): string {
  const d = new Date();
  d.setDate(d.getDate() + 1);
  return d.toISOString().slice(0, 10);
}

test("owner can finalise an everyone-free date; non-owners cannot", async ({
  page,
  api,
  signIn,
  calendar,
  playwright,
}) => {
  await signIn(page, calendar.ownerEmail);

  const targetDate = tomorrowISO();

  // Add the second participant from a fresh, *unauthenticated* request
  // context — `api` is signed in as the owner, so a join from there would
  // 409 "already_joined".
  const anonCtx = await playwright.request.newContext({ baseURL: BACKEND_URL });
  const addRes = await anonCtx.post(`/calendars/${calendar.id}/participants`, {
    data: { name: "Bob" },
  });
  expect(addRes.ok(), `add participant: ${addRes.status()}`).toBeTruthy();
  const bob = (await addRes.json()) as { id: string };
  await anonCtx.dispose();

  for (const participantId of [calendar.ownerParticipantId, bob.id]) {
    const r = await api.put(`/calendars/${calendar.id}/availability`, {
      data: { participant_id: participantId, dates: [targetDate] },
    });
    expect(r.ok(), `set availability for ${participantId}`).toBeTruthy();
  }

  // A non-owner can't finalise — confirm via the API before any UI action so
  // the assertion isn't masked by the calendar already being locked.
  const otherCtx = await playwright.request.newContext({ baseURL: BACKEND_URL });
  const otherLogin = await otherCtx.post("/test/login", {
    data: { email: `intruder-${Date.now()}@example.test` },
  });
  expect(otherLogin.ok()).toBeTruthy();
  const intruder = await otherCtx.post(`/calendars/${calendar.id}/finalise`, {
    data: { date: targetDate },
  });
  expect(intruder.status()).toBe(403);
  await otherCtx.dispose();

  await page.goto(`/calendars/${calendar.id}`);
  await expect(page.getByRole("heading", { level: 1, name: calendar.name })).toBeVisible();

  await page
    .getByRole("button", { name: /^Finalise / })
    .first()
    .click();
  await page.getByRole("button", { name: /^Confirm / }).click();

  // Finalised banner replaces the summary; "Confirmed" is its top-line label.
  await expect(page.getByText("Confirmed")).toBeVisible();

  const after = await api.get(`/calendars/${calendar.id}`);
  const body = (await after.json()) as { calendar: { finalised_date: string | null } };
  expect(body.calendar.finalised_date).toBe(targetDate);
});
