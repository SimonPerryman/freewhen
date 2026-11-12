import { test, expect } from "./fixtures";

// Being signed out is a normal state, not an error. /auth/me answers a
// signed-out visitor with 200 + `null` so Chrome doesn't log a failed-request
// console error on every page load (Lighthouse Best Practices).
test("signed-out page load: /auth/me is 200 null and logs no 401", async ({ page }) => {
  const consoleErrors: string[] = [];
  page.on("console", (msg) => {
    if (msg.type() === "error") consoleErrors.push(msg.text());
  });

  const meResponse = page.waitForResponse(
    (res) => new URL(res.url()).pathname === "/auth/me" && res.request().method() === "GET",
  );
  await page.goto("/");
  const me = await meResponse;

  expect(me.status()).toBe(200);
  expect(await me.json()).toBeNull();

  // The home page renders its signed-out state.
  await expect(page.getByRole("button", { name: /^create/i })).toBeVisible();

  expect(consoleErrors.filter((e) => /401|auth\/me/i.test(e))).toEqual([]);
});

test("invalid session cookie: /auth/me is 200 null", async ({ api }) => {
  const me = await api.get("/auth/me", { headers: { Cookie: "session=not-a-real-session" } });
  expect(me.status()).toBe(200);
  expect(await me.json()).toBeNull();
});
