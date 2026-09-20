import { execFileSync } from "node:child_process";
import { defineConfig, devices } from "@playwright/test";

/**
 * Playwright downloads its own Chromium, which is linked against FHS paths.
 * On NixOS it fails at startup with a missing libglib, so fall back to a
 * browser the system already provides. CI runs on an image where the
 * downloaded build works, so it keeps the default.
 */
function systemBrowser(): string | undefined {
  if (process.env.CI) return undefined;
  if (process.env.PLAYWRIGHT_CHROME) return process.env.PLAYWRIGHT_CHROME;
  for (const name of ["google-chrome", "chromium"]) {
    try {
      return execFileSync("sh", ["-c", `command -v ${name}`], { encoding: "utf8" }).trim();
    } catch {
      continue;
    }
  }
  return undefined;
}

const executablePath = systemBrowser();

/**
 * The suite runs against a build served by `astro preview`, so it checks the
 * output that gets deployed. Building is the slow step, so a local run reuses
 * a server that is already up while CI starts its own.
 */
export default defineConfig({
  testDir: "./tests",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  reporter: process.env.CI ? "github" : "list",
  use: {
    baseURL: "http://localhost:4322",
    trace: "on-first-retry",
    launchOptions: executablePath ? { executablePath } : {},
  },
  projects: [
    {
      name: "desktop",
      use: { ...devices["Desktop Chrome"], viewport: { width: 1440, height: 900 } },
    },
    { name: "mobile", use: { ...devices["Pixel 7"] } },
  ],
  // Its own port, and never a server it did not start. Reusing whatever sat on
  // 4321 meant a local `astro dev` answered the suite, so the assertions ran
  // against unbuilt pages and the build's link validation never ran at all.
  webServer: {
    command: "pnpm build && pnpm preview --port 4322 --ignore-lock",
    url: "http://localhost:4322",
    reuseExistingServer: false,
    timeout: 180_000,
  },
});
