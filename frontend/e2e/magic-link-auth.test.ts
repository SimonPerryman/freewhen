import { test, expect } from "./fixtures";

const BACKEND_URL = process.env.E2E_BACKEND_URL ?? "http://localhost:8090";

test("magic-link: request a link, fetch it from the captured-email endpoint, click it, end up signed in", async ({
  page,
  api,
}) => {
  const email = `magic-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.test`;

  await page.goto("/login");

  await page.getByLabel("Email").fill(email);
  await page.getByRole("button", { name: "Send magic link" }).click();

  // Frontend redirects to /login/check-email after a successful send.
  await expect(page).toHaveURL(/\/login\/check-email/);

  // The capturing email sender stored the link instead of sending it. The
  // link itself goes through the frontend bridge page, which redirects to
  // the backend callback, which redirects back to the frontend root.
  const linkRes = await page.request.get(
    `${BACKEND_URL}/test/magic-link?email=${encodeURIComponent(email)}`,
  );
  expect(linkRes.ok(), `magic link should be captured for ${email}`).toBeTruthy();
  const { link } = (await linkRes.json()) as { link: string };
  expect(link).toContain("/auth/magic-link/callback?token=");

  await page.goto(link);

  // The callback page is an interstitial (audit F4) — token consumption
  // only fires from the user-initiated POST behind this button, not on
  // page load, so link prefetchers can't burn the token. Click through to
  // finish the sign-in.
  // The interstitial once shipped Tailwind classes in a project without
  // Tailwind and rendered as a browser-default page. Guard the styling: the
  // copy is centred and Continue is the shared primary Button, not a grey
  // UA-default button.
  await expect(page.getByRole("heading", { name: "Sign in to FreeWhen" })).toHaveCSS(
    "text-align",
    "center",
  );
  const continueButton = page.getByRole("button", { name: "Continue" });
  await expect(continueButton).toBeVisible();
  await expect(continueButton).toHaveClass(/primary/);
  await expect(continueButton).not.toHaveCSS("background-color", "rgb(239, 239, 239)");
  await continueButton.click();

  // After verify the page navigates to "/" with a session cookie set.
  await expect(page).toHaveURL(/\/$/);

  const me = await api.get(`/auth/me`, {
    headers: {
      Cookie: (await page.context().cookies()).map((c) => `${c.name}=${c.value}`).join("; "),
    },
  });
  expect(me.ok(), `auth/me should return 200 after magic-link auth`).toBeTruthy();
  const body = (await me.json()) as { email: string };
  expect(body.email).toBe(email);
});
