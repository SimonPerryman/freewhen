import { test, expect } from "./fixtures";

// Every spec fails on a Content-Security-Policy violation (the auto `cspGuard`
// fixture in fixtures.ts). This one walks the surfaces the feature specs
// don't: the landing and login pages after hydration/idle work, the owner's
// settings / rename / duplicate / day-detail sheets, and the finalised export.

function tomorrowISO(): string {
  const d = new Date();
  d.setDate(d.getDate() + 1);
  return d.toISOString().slice(0, 10);
}

test("the enforced CSP is sent", async ({ page }) => {
  const res = await page.goto("/");
  const csp = res?.headers()["content-security-policy"] ?? "";
  expect(csp).toContain("default-src 'self'");
  expect(csp).toContain("object-src 'none'");
  expect(csp).toContain("frame-ancestors 'none'");
  expect(csp).not.toContain("'unsafe-eval'");
  expect(res?.headers()["content-security-policy-report-only"]).toBeUndefined();
});

test("landing and login render without CSP violations", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByRole("heading", { level: 1 }).first()).toBeVisible();
  await page.waitForLoadState("networkidle");

  await page.goto("/login");
  await expect(page.getByRole("link", { name: /Continue with Google/ })).toBeVisible();
  await page.waitForLoadState("networkidle");
});

test("owner sheets and finalised export work under the CSP", async ({
  page,
  api,
  signIn,
  calendar,
}) => {
  await signIn(page, calendar.ownerEmail);
  const targetDate = tomorrowISO();

  await page.goto(`/calendars/${calendar.id}`);
  await expect(page.getByRole("heading", { level: 1, name: calendar.name })).toBeVisible();

  // Settings sheet → inline rename.
  await page.getByRole("button", { name: "Calendar settings" }).click();
  await expect(page.getByRole("dialog")).toBeVisible();
  await page.getByRole("button", { name: /Rename calendar/ }).click();
  await expect(page.getByRole("textbox", { name: "Calendar name" })).toBeVisible();

  // Settings sheet → duplicate sheet. (The day-detail sheet is covered by
  // submit-availability.test.ts.)
  await page.reload();
  await page.getByRole("button", { name: "Calendar settings" }).click();
  await page.getByRole("button", { name: /Duplicate this calendar/ }).click();
  await expect(page.getByRole("heading", { name: /Duplicate/ })).toBeVisible();

  // Finalise via the API, then use the export buttons.
  const avail = await api.put(`/calendars/${calendar.id}/availability`, {
    data: { participant_id: calendar.ownerParticipantId, dates: [targetDate] },
  });
  expect(avail.ok()).toBeTruthy();
  const fin = await api.post(`/calendars/${calendar.id}/finalise`, {
    data: { date: targetDate },
  });
  expect(fin.ok(), `finalise: ${fin.status()}`).toBeTruthy();
  await page.reload();
  await expect(page.getByText("Confirmed").first()).toBeVisible();
  await expect(page.getByRole("link", { name: "Add to Google Calendar" })).toBeVisible();

  const downloadPromise = page.waitForEvent("download");
  await page.getByRole("link", { name: "Download .ics" }).click();
  const download = await downloadPromise;
  expect(download.suggestedFilename()).toMatch(/\.ics$/);
});
