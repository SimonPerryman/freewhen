import { test, expect } from "./fixtures";

const BACKEND_URL = process.env.E2E_BACKEND_URL ?? "http://localhost:8090";

function isoDaysAhead(n: number): string {
  const d = new Date();
  d.setDate(d.getDate() + n);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}

// At 375px a "most people" row carries Finalise plus a "<name> prefers" pill.
// Before the grid layout that row was ~36px wider than the card.
test("summary rows never overflow the card at 375px", async ({
  page,
  api,
  signIn,
  calendar,
  playwright,
}) => {
  await page.setViewportSize({ width: 375, height: 800 });
  const top = isoDaysAhead(3);
  const partial = isoDaysAhead(4);

  const owner = await api.put(`/calendars/${calendar.id}/availability`, {
    data: { participant_id: calendar.ownerParticipantId, dates: [top, partial] },
  });
  expect(owner.ok()).toBeTruthy();
  for (const [name, dates, preferred] of [
    ["Samantha", [top, partial], [partial]],
    ["Jordan", [top], []],
  ] as const) {
    const anon = await playwright.request.newContext({ baseURL: BACKEND_URL });
    const add = await anon.post(`/calendars/${calendar.id}/participants`, { data: { name } });
    expect(add.ok()).toBeTruthy();
    const p = (await add.json()) as { id: string };
    const put = await anon.put(`/calendars/${calendar.id}/availability`, {
      headers: { "X-Participant-Token": add.headers()["x-participant-token"] },
      data: { participant_id: p.id, dates: [...dates], preferred: [...preferred] },
    });
    expect(put.ok()).toBeTruthy();
    await anon.dispose();
  }

  await signIn(page, calendar.ownerEmail);
  await page.goto(`/calendars/${calendar.id}`);
  const pill = page.getByTestId("summary-row-meta").getByText("Samantha", { exact: true });
  await expect(pill).toBeVisible();

  const row = pill.locator("xpath=ancestor::li[1]");
  const list = row.locator("xpath=..");
  const [rowBox, listBox] = [await row.boundingBox(), await list.boundingBox()];
  expect(rowBox && listBox).toBeTruthy();
  expect(rowBox!.x + rowBox!.width).toBeLessThanOrEqual(listBox!.x + listBox!.width + 0.5);
  const widths = await list.evaluate((el) => ({ scroll: el.scrollWidth, client: el.clientWidth }));
  expect(widths.scroll).toBeLessThanOrEqual(widths.client);
  const pillBox = await pill.boundingBox();
  expect(pillBox!.x + pillBox!.width).toBeLessThanOrEqual(listBox!.x + listBox!.width + 0.5);
});
