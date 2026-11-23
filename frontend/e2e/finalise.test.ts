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
  // 409 "already_joined". The join hands Bob his participant token.
  const anonCtx = await playwright.request.newContext({ baseURL: BACKEND_URL });
  const addRes = await anonCtx.post(`/calendars/${calendar.id}/participants`, {
    data: { name: "Bob" },
  });
  expect(addRes.ok(), `add participant: ${addRes.status()}`).toBeTruthy();
  const bob = (await addRes.json()) as { id: string };
  const bobToken = addRes.headers()["x-participant-token"];
  expect(bobToken, "anonymous join should issue a participant token").toBeTruthy();

  // Each participant marks their own availability with their own
  // credentials: the owner by session, Bob by his participant token.
  const ownerSet = await api.put(`/calendars/${calendar.id}/availability`, {
    data: { participant_id: calendar.ownerParticipantId, dates: [targetDate] },
  });
  expect(ownerSet.ok(), `owner set availability: ${ownerSet.status()}`).toBeTruthy();

  const bobSet = await anonCtx.put(`/calendars/${calendar.id}/availability`, {
    headers: { "X-Participant-Token": bobToken },
    data: { participant_id: bob.id, dates: [targetDate] },
  });
  expect(bobSet.ok(), `Bob set availability: ${bobSet.status()}`).toBeTruthy();
  await anonCtx.dispose();

  // The owner has no override on someone else's availability.
  const ownerForBob = await api.put(`/calendars/${calendar.id}/availability`, {
    data: { participant_id: bob.id, dates: [] },
  });
  expect(ownerForBob.status()).toBe(403);

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

  // "Change the date" reopens the calendar; Undo finalises the same date again.
  await page.getByRole("button", { name: "Change the date" }).click();
  await expect(page.getByText("Calendar reopened")).toBeVisible();
  await expect(page.getByText("Confirmed")).toBeHidden();
  await page.getByRole("button", { name: "Undo" }).click();
  await expect(page.getByText("Confirmed")).toBeVisible();
});

test("passphrase gate: wrong passphrase is rejected, right one unlocks, and .ics export works with access", async ({
  page,
  api,
  calendar,
}) => {
  const targetDate = tomorrowISO();
  const setRes = await api.put(`/calendars/${calendar.id}/availability`, {
    data: { participant_id: calendar.ownerParticipantId, dates: [targetDate] },
  });
  expect(setRes.ok()).toBeTruthy();
  const finRes = await api.post(`/calendars/${calendar.id}/finalise`, {
    data: { date: targetDate },
  });
  expect(finRes.ok(), `finalise: ${finRes.status()}`).toBeTruthy();
  const passRes = await api.post(`/calendars/${calendar.id}/passphrase`, { data: {} });
  expect(passRes.ok(), `set passphrase: ${passRes.status()}`).toBeTruthy();
  const { passphrase } = (await passRes.json()) as { passphrase: string };

  // The browser context is signed out (only `api` is the owner), so it has
  // no access and hits the gate.
  const ctx = page.context();
  await page.goto(`/calendars/${calendar.id}`);
  const field = page.getByLabel("Passphrase");
  await expect(field).toBeVisible();

  // Export is refused without access.
  const noAccess = await ctx.request.get(`${BACKEND_URL}/calendars/${calendar.id}/export.ics`);
  expect(noAccess.status()).toBe(401);

  await field.fill("definitely-not-it");
  await page.getByRole("button", { name: /Unlock/ }).click();
  await expect(page.getByText(/doesn't match/)).toBeVisible();

  await field.fill(passphrase);
  await page.getByRole("button", { name: /Unlock/ }).click();
  await expect(page.getByRole("heading", { level: 1, name: calendar.name })).toBeVisible();

  // The finalised banner's .ics link carries the access token and downloads.
  const ics = page.getByRole("link", { name: "Download .ics" });
  await expect(ics).toHaveAttribute("href", /[?&]access=/);
  const icsRes = await ctx.request.get((await ics.getAttribute("href"))!);
  expect(icsRes.ok(), `ics: ${icsRes.status()}`).toBeTruthy();
  expect(await icsRes.text()).toContain("BEGIN:VCALENDAR");
});
