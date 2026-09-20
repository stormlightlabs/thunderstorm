import { expect, test } from "@playwright/test";

test.describe("home page", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto("/");
  });

  test("leads with the title and tagline", async ({ page }) => {
    await expect(page.locator("h1#_top")).toHaveText("thunderstorm");
    await expect(page.getByText("An Agent Agnostic Loop Harness")).toBeVisible();
  });

  test("offers the install commands for the one harness that has a payload", async ({ page }) => {
    await expect(page.getByRole("tab", { name: "Claude Code" })).toBeVisible();
    await expect(page.getByText("claude plugin marketplace add stormlightlabs/thunderstorm")).toBeVisible();
    await expect(page.getByText("claude plugin install thunderstorm@stormlightlabs")).toBeVisible();
  });

  // The renderer refuses to build a Codex or Pi payload, so the page must not
  // offer a command that installs nothing.
  test("offers nothing for the harnesses that have no payload", async ({ page }) => {
    for (const label of ["Codex", "Pi", "Cursor"]) {
      await expect(page.getByRole("tab", { name: label })).toHaveCount(0);
    }
    await expect(page.getByText("pi install git:")).toHaveCount(0);
    await expect(page.getByText("codex plugin")).toHaveCount(0);
  });

  test("lists every stage", async ({ page }) => {
    const stages = page.locator(".stages dt");
    await expect(stages).toHaveCount(9);
    await expect(stages.filter({ hasText: "/forecast" })).toHaveCount(1);
  });
});
