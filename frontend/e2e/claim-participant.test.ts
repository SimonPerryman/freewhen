import { test, expect } from "./fixtures";

const BACKEND_URL = process.env.E2E_BACKEND_URL ?? "http://localhost:8090";

// user_id is omitempty on the wire — absent (not null) until claimed.
type CalendarBody = {
  participants: Array<{ id: string; name: string; user_id?: string }>;
};

test("anonymous joiner becomes a logged-in user; their participant row is claimed", async ({
  page,
  api,
  signIn,
  calendar,
  playwright,
}) => {
  // 1. Anonymous join through the UI. The browser keeps the participant
  //    pointer and the participant token (X-Participant-Token, issued only
  //    on join) in localStorage — the token is what proves "this was me".
  await page.goto(`/calendars/${calendar.id}`);
  const joinerName = `Claimer ${Date.now()}`;
  await page.getByLabel("Your name").fill(joinerName);
  await page.getByRole("button", { name: "Join" }).click();
  await expect(page.getByText(`${joinerName} (you)`)).toBeVisible();

  // Verify pre-claim: user_id is null.
  let res = await api.get(`/calendars/${calendar.id}`);
  let body = (await res.json()) as CalendarBody;
  const before = body.participants.find((p) => p.name === joinerName);
  expect(before).toBeDefined();
  // user_id is omitempty — absent when null.
  expect(before!.user_id ?? null).toBeNull();

  // A signed-in user without the participant token can't claim the row:
  // the participant id alone is public.
  const intruderCtx = await playwright.request.newContext({ baseURL: BACKEND_URL });
  const intruderLogin = await intruderCtx.post("/test/login", {
    data: { email: `intruder-${Date.now()}@example.test` },
  });
  expect(intruderLogin.ok()).toBeTruthy();
  const stolen = await intruderCtx.post(
    `/calendars/${calendar.id}/participants/${before!.id}/claim`,
    { headers: { "Content-Type": "application/json" } },
  );
  expect(stolen.status()).toBe(403);
  await intruderCtx.dispose();

  // 2. Sign in from the same browser (brand-new email, so no prior
  //    participant), then come back to the calendar — the full page load a
  //    magic-link return does. The page silently claims the anonymous row,
  //    sending the stored participant token.
  await signIn(page);
  const claimResponse = page.waitForResponse(
    (r) => r.url().endsWith(`/participants/${before!.id}/claim`) && r.request().method() === "POST",
  );
  await page.goto(`/calendars/${calendar.id}`);
  const claim = await claimResponse;
  expect(claim.ok(), `claim: ${claim.status()} ${await claim.text()}`).toBeTruthy();
  await expect(page.getByText(`${joinerName} (you)`)).toBeVisible();

  // 3. Verify post-claim: same participant id, user_id now populated.
  res = await api.get(`/calendars/${calendar.id}`);
  body = (await res.json()) as CalendarBody;
  const after = body.participants.find((p) => p.id === before!.id);
  expect(after, "participant should still exist post-claim").toBeDefined();
  expect(after!.name).toBe(joinerName);
  expect(after!.user_id, "user_id should be set after claim").toBeTruthy();
});
