import { defineConfig, devices } from "@playwright/test";

const FRONTEND_PORT = 3100;
const BACKEND_PORT = 8090;
const FRONTEND_URL = `http://localhost:${FRONTEND_PORT}`;
const BACKEND_URL = `http://localhost:${BACKEND_PORT}`;

export default defineConfig({
  testDir: "./e2e",
  testMatch: "**/*.test.ts",
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: 1,
  reporter: process.env.CI ? [["github"], ["html", { open: "never" }]] : "list",
  use: {
    baseURL: FRONTEND_URL,
    trace: "retain-on-failure",
    video: "retain-on-failure",
  },
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },
  ],
  webServer: [
    {
      // Boot embedded Postgres + the backend in E2E_TEST_MODE.
      command: "go run ./cmd/e2e-server",
      cwd: "../backend",
      env: {
        PORT: String(BACKEND_PORT),
        CORS_ORIGIN: FRONTEND_URL,
      },
      url: `${BACKEND_URL}/health`,
      reuseExistingServer: !process.env.CI,
      timeout: 120_000,
      stdout: "pipe",
      stderr: "pipe",
    },
    {
      // Loopback only, like the e2e backend — avoids Windows Firewall prompts.
      // `localhost`, not 127.0.0.1: tests load http://localhost, and a
      // mismatched bind slowed page startup enough to drop typed input.
      command: `pnpm exec next dev --hostname localhost --port ${FRONTEND_PORT}`,
      env: {
        NEXT_PUBLIC_API_URL: BACKEND_URL,
      },
      url: FRONTEND_URL,
      reuseExistingServer: !process.env.CI,
      timeout: 120_000,
      stdout: "pipe",
      stderr: "pipe",
    },
  ],
});
