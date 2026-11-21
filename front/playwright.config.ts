import { defineConfig } from "@playwright/test";

// Critical flows run against a real stack — `make e2e` builds the prod
// images and serves them on :8088. No webServer here: a host-side `vite
// dev` would skip the nginx /api proxy and the production build, which is
// exactly what F12 is meant to validate.
const baseURL = process.env.E2E_BASE_URL ?? "http://localhost:8088";

// NixOS and other distros ship a usable Chromium but not the system
// libraries Playwright's bundled build expects; when the runner points at
// one, launch that instead. Unset on a normal machine, where
// `bunx playwright install --with-deps chromium` provides the browser.
const systemChromium = process.env.E2E_CHROMIUM_PATH;

export default defineConfig({
  testDir: "./e2e",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  // One worker: the two flows share the login rate limit (5/min/IP), and
  // the session lives in the storage state the setup project writes.
  workers: 1,
  reporter: [["list"], ["html", { open: "never" }]],
  use: {
    baseURL,
    // The stack runs TZ=UTC; pinning the browser removes the day-boundary
    // drift between the dashboard's ?date= and the event's local hour.
    locale: "en-US",
    timezoneId: "UTC",
    trace: "on-first-retry",
    // Top-level, not per-project: the setup project needs it too.
    ...(systemChromium !== undefined && systemChromium !== ""
      ? { launchOptions: { executablePath: systemChromium } }
      : {}),
  },
  projects: [
    { name: "setup", testMatch: /auth\.setup\.ts/ },
    {
      name: "chromium",
      // Not devices["Desktop Chrome"]: that preset pins Playwright's
      // branded chromium channel, which ignores an executablePath.
      use: {
        browserName: "chromium",
        viewport: { width: 1280, height: 720 },
        storageState: "e2e/.auth/state.json",
      },
      dependencies: ["setup"],
    },
  ],
});
