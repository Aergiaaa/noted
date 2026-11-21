import { mkdirSync } from "node:fs";
import { dirname } from "node:path";
import { test as setup, expect } from "@playwright/test";
import { currentTotp } from "./totp";

// One login for both critical flows: the stack's rate limit counts every
// attempt (5/min/IP), and a retried login in each spec could exhaust it.
const STATE_PATH = "e2e/.auth/state.json";

setup("logs in with the seeded TOTP secret", async ({ page }) => {
  await page.goto("/");
  await expect(page).toHaveURL(/\/login$/);

  await page.getByLabel("Code").fill(currentTotp());
  await page.getByRole("button", { name: "Log in" }).click();

  await expect(
    page.getByRole("heading", { name: "Today at a glance" }),
  ).toBeVisible();

  mkdirSync(dirname(STATE_PATH), { recursive: true });
  await page.context().storageState({ path: STATE_PATH });
});
