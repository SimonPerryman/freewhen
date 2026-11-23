import { test, expect } from "./fixtures";

const BACKEND_URL = process.env.E2E_BACKEND_URL ?? "http://localhost:8090";

test("a calendar name typed while signed out is created after the magic-link sign-in", async ({
  page,
}) => {
  const email = `keep-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.test`;
  const calendarName = `Kept ${Date.now()}`;

  await page.goto("/");
  await page.getByLabel("Event name").fill(calendarName);
  await page.getByRole("button", { name: "Create" }).click();

  await expect(page).toHaveURL(/\/login/);
  await page.getByLabel("Email").fill(email);
  await page.getByRole("button", { name: "Send magic link" }).click();
  await expect(page).toHaveURL(/\/login\/check-email/);

  const linkRes = await page.request.get(
    `${BACKEND_URL}/test/magic-link?email=${encodeURIComponent(email)}`,
  );
  const { link } = (await linkRes.json()) as { link: string };
  await page.goto(link);
  await page.getByRole("button", { name: "Continue" }).click();

  // Back on the landing page signed in, the calendar is created and opened.
  await expect(page).toHaveURL(/\/calendars\/[^/]+$/);
  await expect(page.getByRole("heading", { level: 1, name: calendarName })).toBeVisible();
});
