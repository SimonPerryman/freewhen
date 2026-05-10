import { test, expect } from "./fixtures";

test("anonymous joiner opens share link and joins with a name", async ({ page, api, calendar }) => {
  // Browser context is not signed in (no test-login call). Open the share URL.
  await page.goto(`/calendars/${calendar.id}`);

  await expect(page.getByRole("heading", { level: 1, name: calendar.name })).toBeVisible();

  const joinerName = `Anon ${Date.now()}`;
  await page.getByLabel("Your name").fill(joinerName);
  await page.getByRole("button", { name: "Join" }).click();

  // Joiner appears in the participant list.
  await expect(page.getByText(joinerName).first()).toBeVisible();

  // Verify the API: anonymous join => user_id is null on the participant record.
  const res = await api.get(`/calendars/${calendar.id}`);
  expect(res.ok()).toBeTruthy();
  const body = (await res.json()) as {
    participants: Array<{ name: string; user_id: string | null }>;
  };
  const joiner = body.participants.find((p) => p.name === joinerName);
  expect(joiner, "joiner should be in participants").toBeDefined();
  // user_id is omitempty in the JSON response — absent when null.
  expect(joiner!.user_id ?? null).toBeNull();
});
