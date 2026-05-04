import { test, expect } from "./fixtures";

test("anonymous joiner becomes a logged-in user; their participant row is claimed", async ({
  page,
  api,
  signIn,
  calendar,
}) => {
  // 1. Anonymous join — sets the localStorage participant pointer that the
  //    claim flow later uses to identify "who I was".
  await page.goto(`/calendars/${calendar.id}`);
  const joinerName = `Claimer ${Date.now()}`;
  await page.getByLabel("Your name").fill(joinerName);
  await page.getByRole("button", { name: "Join" }).click();
  await expect(page.getByText(joinerName).first()).toBeVisible();

  // Verify pre-claim: user_id is null.
  let res = await api.get(`/calendars/${calendar.id}`);
  let body = (await res.json()) as {
    participants: Array<{ id: string; name: string; user_id: string | null }>;
  };
  const before = body.participants.find((p) => p.name === joinerName);
  expect(before).toBeDefined();
  // user_id is omitempty — absent when null.
  expect(before!.user_id ?? null).toBeNull();

  // 2. Sign in (using a brand-new email so we know they had no prior
  //    participant). The browser context now holds both: the localStorage
  //    pointer to the anon participant AND a session cookie.
  await signIn(page);

  // 3. Hit the claim endpoint with the fresh session cookie. The page also
  //    runs this silently when a localStorage participant + signed-in user
  //    coincide; here we drive it explicitly because the page's silent path
  //    depends on `user` being hydrated *before* the calendar page's load
  //    effect fires, which races with a fresh full-page navigation. The
  //    endpoint is the same; this asserts the contract the silent path
  //    leans on.
  const claim = await page.request.post(
    `${process.env.E2E_BACKEND_URL ?? "http://localhost:8090"}/calendars/${calendar.id}/participants/${before!.id}/claim`,
  );
  expect(claim.ok(), `claim: ${claim.status()} ${await claim.text()}`).toBeTruthy();

  // 4. Verify post-claim: same participant id, user_id now populated.
  res = await api.get(`/calendars/${calendar.id}`);
  body = (await res.json()) as {
    participants: Array<{ id: string; name: string; user_id?: string }>;
  };
  const after = body.participants.find((p) => p.id === before!.id);
  expect(after, "participant should still exist post-claim").toBeDefined();
  expect(after!.name).toBe(joinerName);
  expect(after!.user_id, "user_id should be set after claim").toBeTruthy();

  // And the page reflects the claimed state on next load — owner UI / "you"
  // tag would be the most precise check, but the participant simply
  // remaining visible after claim is the primary thing that breaks if the
  // claim made the row unreachable.
  await page.goto(`/calendars/${calendar.id}`);
  await expect(page.getByText(joinerName).first()).toBeVisible();
});
