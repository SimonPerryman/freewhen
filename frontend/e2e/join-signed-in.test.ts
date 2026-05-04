import { test, expect } from "./fixtures";

test("signed-in joiner gets user_id auto-linked when joining via the share URL", async ({
  page,
  api,
  signIn,
  calendar,
}) => {
  // Different signed-in user from the calendar's owner.
  const { email } = await signIn(page);
  const localPart = email.split("@")[0];

  await page.goto(`/calendars/${calendar.id}`);
  await expect(page.getByRole("heading", { level: 1, name: calendar.name })).toBeVisible();

  const joinerName = `Signed ${Date.now()}`;
  await page.getByLabel("Your name").fill(joinerName);
  await page.getByRole("button", { name: "Join" }).click();
  await expect(page.getByText(joinerName).first()).toBeVisible();

  // Verify the API: user_id should be set on the joiner's participant row.
  const res = await api.get(`/calendars/${calendar.id}`);
  const body = (await res.json()) as {
    participants: Array<{ id: string; name: string; user_id: string | null }>;
  };
  const me = body.participants.find((p) => p.name === joinerName);
  expect(me, `participant ${joinerName} should exist (signed in as ${localPart})`).toBeDefined();
  expect(me!.user_id, "user_id should be set for signed-in joiner").toBeTruthy();
});
