import { expect, test } from "@playwright/test";

const pages = [
  "/",
  "/start/introduction/",
  "/start/install/",
  "/start/first-run/",
  "/reference/harnesses/",
  "/reference/configuration/",
];

// Sideways scrolling makes a page hard to read on a phone. The tables in the
// harness reference are the widest thing on the site, so they are what this
// catches first.
for (const path of pages) {
  test(`${path} never scrolls horizontally`, async ({ page }) => {
    await page.goto(path);
    const overflow = await page.evaluate(
      () => document.documentElement.scrollWidth - document.documentElement.clientWidth
    );
    expect(overflow).toBeLessThanOrEqual(0);
  });
}

test.describe("hero layout", () => {
  test("sits in two columns on a desktop", async ({ page }, testInfo) => {
    test.skip(testInfo.project.name !== "desktop", "desktop only");
    await page.goto("/");
    const copy = await page.locator(".hero > .copy").boundingBox();
    const panel = await page.locator(".hero > .panel").boundingBox();
    expect(copy).not.toBeNull();
    expect(panel).not.toBeNull();
    // Side by side: the panel starts after the copy ends, and they overlap vertically.
    expect(panel!.x).toBeGreaterThan(copy!.x + copy!.width - 1);
    expect(panel!.y).toBeLessThan(copy!.y + copy!.height);
  });

  // 720 is the shortest laptop worth designing for, and the stage list is what
  // will push past it first if rows are added.
  for (const height of [720, 800, 900]) {
    test(`fits a ${height}px screen without scrolling`, async ({ page }, testInfo) => {
      test.skip(testInfo.project.name !== "desktop", "desktop only");
      await page.setViewportSize({ width: 1440, height });
      await page.goto("/");
      const scrollable = await page.evaluate(
        () => document.documentElement.scrollHeight - window.innerHeight
      );
      expect(scrollable).toBeLessThanOrEqual(0);
    });
  }

  test("stacks into one column on a phone", async ({ page }, testInfo) => {
    test.skip(testInfo.project.name !== "mobile", "mobile only");
    await page.goto("/");
    const copy = await page.locator(".hero > .copy").boundingBox();
    const panel = await page.locator(".hero > .panel").boundingBox();
    expect(panel!.y).toBeGreaterThan(copy!.y + copy!.height - 1);
  });
});
