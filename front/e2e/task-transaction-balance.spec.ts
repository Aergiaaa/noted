import { test, expect } from "@playwright/test";

// F12 flow 2: task → expense linked to it → the derived balance reflects
// it. The stack starts on a throwaway volume, so the total is $0 before.
test("records a task-linked expense and the balance reflects it", async ({
  page,
}) => {
  const title = `E2E buy keyboard ${Date.now().toString(36)}`;

  await page.goto("/tasks");
  await page.getByRole("button", { name: "New task" }).click();
  await page.getByLabel("Title").fill(title);
  await page.getByRole("button", { name: "Save" }).click();
  await expect(page.getByText(title)).toBeVisible();

  await page.goto("/finance");
  await expect(page.getByText("Total balance")).toBeVisible();
  // The balance header is the only .text-2xl value; wallet cards show
  // compact amounts, so this cannot collide with the strip.
  await expect(page.locator(".text-2xl")).toHaveText("$0");

  await page.getByRole("button", { name: "Transaction", exact: true }).click();
  await page.getByLabel("Amount", { exact: true }).fill("500");
  await page.getByLabel("Amount group").fill("000");
  await page.getByLabel("Memo").fill("E2E keyboard");
  await page.getByLabel("Task", { exact: true }).selectOption({
    label: title,
  });
  await page.getByRole("button", { name: "Record", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "Record", exact: true }),
  ).toBeHidden();

  // The task link resolved to a chip and the balance is derived on read.
  await expect(page.getByText("E2E keyboard", { exact: true })).toBeVisible();
  await expect(page.getByRole("link", { name: title })).toBeVisible();
  await expect(page.locator(".text-2xl")).toHaveText("-$500,000");

  // Survives a full reload: the money lives in SQLite, not page state.
  await page.reload();
  await expect(page.locator(".text-2xl")).toHaveText("-$500,000");
});
