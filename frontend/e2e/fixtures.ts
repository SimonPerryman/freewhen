import { test as base, expect, type APIRequestContext, type Page } from "@playwright/test";

const BACKEND_URL = process.env.E2E_BACKEND_URL ?? "http://localhost:8090";

export type CalendarFixture = {
  id: string;
  name: string;
  ownerEmail: string;
  ownerParticipantId: string;
};

type Fixtures = {
  cspGuard: void;
  api: APIRequestContext;
  signIn: (page: Page, email?: string) => Promise<{ email: string }>;
  calendar: CalendarFixture;
};

let calendarSeq = 0;
function uniqueCalendarName() {
  calendarSeq += 1;
  return `E2E ${Date.now()}-${calendarSeq}`;
}

function uniqueEmail(prefix = "e2e") {
  return `${prefix}+${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.test`;
}

export const test = base.extend<Fixtures>({
  // Runs for every test: fails it if the page tripped the Content-Security-
  // Policy anywhere (blocked script, fetch, font, form post...). The init
  // script surfaces `securitypolicyviolation` events as console errors; the
  // browser's own "Refused to ... Content Security Policy" errors are caught
  // by the same listener.
  cspGuard: [
    async ({ context }, use) => {
      const violations: string[] = [];
      await context.addInitScript(() => {
        document.addEventListener("securitypolicyviolation", (e) => {
          console.error(
            `[csp-violation] ${e.effectiveDirective} blocked ${e.blockedURI || "inline"} on ${e.documentURI}`,
          );
        });
      });
      context.on("console", (msg) => {
        const text = msg.text();
        if (/\[csp-violation\]|Content Security Policy/.test(text)) violations.push(text);
      });
      await use();
      expect(violations, "Content-Security-Policy violations").toEqual([]);
    },
    { auto: true },
  ],

  api: async ({ playwright }, use) => {
    const ctx = await playwright.request.newContext({ baseURL: BACKEND_URL });
    await use(ctx);
    await ctx.dispose();
  },

  signIn: async ({ context }, use) => {
    // Calls the E2E_TEST_MODE-gated /test/login endpoint. The Set-Cookie that
    // comes back is captured by the browser context, so subsequent page nav
    // is authenticated.
    const helper = async (page: Page, email?: string) => {
      const e = email ?? uniqueEmail();
      const res = await context.request.post(`${BACKEND_URL}/test/login`, {
        data: { email: e, name: e.split("@")[0] },
      });
      expect(res.ok(), `test-login failed: ${res.status()} ${await res.text()}`).toBeTruthy();
      return { email: e };
    };
    await use(helper);
  },

  calendar: async ({ api }, use) => {
    // Sign-in via API (separate cookie jar from the browser context) and
    // create the calendar. Tests that need the browser to be signed in as
    // the same user should call signIn with the same email.
    const ownerEmail = uniqueEmail("owner");
    const loginRes = await api.post(`/test/login`, {
      data: { email: ownerEmail, name: ownerEmail.split("@")[0] },
    });
    expect(loginRes.ok()).toBeTruthy();

    const name = uniqueCalendarName();
    const createRes = await api.post(`/calendars`, { data: { name } });
    expect(createRes.ok(), `create calendar: ${createRes.status()}`).toBeTruthy();
    const body = (await createRes.json()) as {
      id: string;
      participant: { id: string };
    };

    await use({
      id: body.id,
      name,
      ownerEmail,
      ownerParticipantId: body.participant.id,
    });
  },
});

export { expect };
