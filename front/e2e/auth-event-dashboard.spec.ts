import { test, expect } from "@playwright/test";

// F12 flow 1: login (auth.setup.ts) → create an event through the calendar
// → the dashboard's "Events today" card shows it.
test("creates an event from the calendar and finds it on the dashboard", async ({
  page,
}) => {
  // Unique per run: a repeated local run against the same disposable volume
  // must not trip over a previous run's rows.
  const title = `E2E keyboard pickup ${Date.now().toString(36)}`;

  await page.goto("/calendar");
  await expect(page.getByRole("heading", { name: "Calendar" })).toBeVisible();

  await page.getByRole("button", { name: "New event" }).click();
  await page.getByLabel("Title").fill(title);
  await page.getByRole("button", { name: "Save" }).click();
  await expect(page.getByRole("button", { name: "Save" })).toBeHidden();

  await page.getByRole("link", { name: "Dashboard" }).click();
  await expect(
    page.getByRole("heading", { name: "Today at a glance" }),
  ).toBeVisible();
  await expect(page.getByRole("link", { name: title })).toBeVisible();
});
