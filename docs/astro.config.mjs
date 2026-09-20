// @ts-check
import { defineConfig } from "astro/config";
import starlight from "@astrojs/starlight";
import linksValidator from "starlight-links-validator";
import llmsTxt from "starlight-llms-txt";

export default defineConfig({
  site: "https://thunderstorm.stormlightlabs.org",
  integrations: [
    starlight({
      plugins: [linksValidator(), llmsTxt()],
      title: "thunderstorm",
      description:
        "An installable development loop for Claude Code, Pi, Codex, and Cursor: one set of skills, one board, and the checks that hold them together.",
      social: [
        { icon: "github", label: "GitHub", href: "https://github.com/stormlightlabs/thunderstorm" },
      ],
      customCss: [
        "@fontsource-variable/geist",
        "@fontsource-variable/geist-mono",
        "./src/styles/theme.css",
      ],
      sidebar: [
        { label: "Start here", items: [{ autogenerate: { directory: "start" } }] },
        { label: "Reference", items: [{ autogenerate: { directory: "reference" } }] },
      ],
    }),
  ],
});
