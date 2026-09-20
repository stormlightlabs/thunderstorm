import { expect, test } from "@playwright/test";

/** The palette is defined three times: bare :root, the dark override, and the
 * light override. A token defined in only one of them reads wrong in the other
 * theme, so check an actual computed color in both. */
test.describe("theme", () => {
  test("dark and light use different backgrounds and both set one", async ({ page }) => {
    await page.goto("/");

    const bodyBackground = () =>
      page.evaluate(() => getComputedStyle(document.body).backgroundColor);

    await page.evaluate(() => document.documentElement.setAttribute("data-theme", "dark"));
    const dark = await bodyBackground();

    await page.evaluate(() => document.documentElement.setAttribute("data-theme", "light"));
    const light = await bodyBackground();

    expect(dark).not.toBe(light);
    for (const color of [dark, light]) {
      expect(color).not.toBe("rgba(0, 0, 0, 0)");
      expect(color).not.toBe("transparent");
    }
  });

  test("the accent is a blue in both themes", async ({ page }) => {
    await page.goto("/");
    for (const theme of ["dark", "light"]) {
      await page.evaluate((t) => document.documentElement.setAttribute("data-theme", t), theme);
      const accent = await page.evaluate(() =>
        getComputedStyle(document.documentElement).getPropertyValue("--sl-color-accent").trim()
      );
      const [, r, g, b] = /#(\w{2})(\w{2})(\w{2})/.exec(accent) ?? [];
      expect(accent, `accent in ${theme}`).toMatch(/^#/);
      expect(parseInt(b, 16), `blue channel dominates in ${theme}`).toBeGreaterThan(
        parseInt(r, 16)
      );
      expect(parseInt(b, 16)).toBeGreaterThan(parseInt(g, 16));
    }
  });
});

test.describe("typography", () => {
  test("uses Google Sans for headings, Inter for text, Google Sans Code for code", async ({
    page,
  }) => {
    await page.goto("/start/introduction/");
    const family = (selector: string) =>
      page.locator(selector).first().evaluate((el) => getComputedStyle(el).fontFamily);

    expect(await family("h1")).toContain("Google Sans Variable");
    expect(await family("p")).toContain("Inter Variable");

    await page.goto("/start/install/");
    expect(await family("code")).toContain("Google Sans Code Variable");
  });
});
