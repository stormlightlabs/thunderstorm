import { expect, test } from "@playwright/test";

test.describe("home page", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto("/");
  });

  test("leads with the title and tagline", async ({ page }) => {
    await expect(page.locator("h1#_top")).toHaveText("thunderstorm");
    await expect(page.getByText("An Agent Agnostic Loop Harness")).toBeVisible();
  });

  test("offers an install command for each harness", async ({ page }) => {
    for (const label of ["Claude Code", "Codex", "Pi"]) {
      await expect(page.getByRole("tab", { name: label })).toBeVisible();
    }
    await expect(page.getByText("/plugin marketplace add stormlightlabs/thunderstorm")).toBeVisible();
  });

  test("switching a tab shows that harness's command", async ({ page }) => {
    await page.getByRole("tab", { name: "Pi" }).click();
    await expect(page.getByText("pi install git:github.com/stormlightlabs/thunderstorm")).toBeVisible();
    await expect(page.getByText("/plugin marketplace add stormlightlabs/thunderstorm")).toBeHidden();
  });

  test("lists every stage", async ({ page }) => {
    const stages = page.locator(".stages dt");
    await expect(stages).toHaveCount(9);
    await expect(stages.filter({ hasText: "/forecast" })).toHaveCount(1);
  });
});
