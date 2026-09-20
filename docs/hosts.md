# Host contracts

What Claude Code, Pi, and Codex actually load, verified on this machine against
Claude Code, `pi` 0.85.1, and `codex-cli` 0.146.0. Cursor is not covered; no
Cursor agent is installed here, so nothing below is claimed about it.

Every row was checked by building a fixture with a probe skill in each candidate
directory and asking each agent to name the skills it could see. Paths taken
from documentation alone are marked as such.

## Skill discovery

A skill is a directory holding `SKILL.md` with `name` and `description`
frontmatter. All three hosts agree on that shape, so the file itself needs no
translation. They disagree only on where they look.

| Directory              | Claude Code | Pi  | Codex |
| ---------------------- | ----------- | --- | ----- |
| `.claude/skills/`      | yes         | no  | no    |
| `.pi/skills/`          | no          | yes | no    |
| `.codex/skills/`       | no          | no  | yes   |
| `.agents/skills/`      | no          | yes | yes   |
| `~/.claude/skills/`    | yes         | no  | no    |
| `~/.agents/skills/`    | no          | yes | yes   |

`.agents/skills/` covers Pi and Codex together. Claude Code reads neither the
project nor the user copy of it, so two locations cover all three hosts rather
than four. That is what the `.agents/skills -> ../.claude/skills` symlink in
thunderus was working around.

Pi walks from the working directory up to the git root collecting
`.agents/skills` at every level, and requires the project to be trusted before
it reads any of them. It also distinguishes the two roots: under `.pi/skills` a
bare `*.md` file is a skill, while under `.agents/skills` a skill must be a
subdirectory containing `SKILL.md`. Write the subdirectory form and both roots
accept it.

## Other resources

Skills are the portable part. Nothing else is.

| Resource    | Claude Code            | Pi                  | Codex                  |
| ----------- | ---------------------- | ------------------- | ---------------------- |
| Commands    | `.claude/commands/*.md`| `.pi/prompts/*.md`  | `~/.codex/prompts/*.md`|
| Subagents   | `.claude/agents/*.md`  | none                | none                   |
| Hooks       | `.claude/settings.json`| extensions (TS/JS)  | none usable            |
| Themes      | none                   | `.pi/themes/*.json` | `config.toml`          |

Pi's project resource roots are `.pi/{extensions,skills,prompts,themes}` and its
user roots are `~/.pi/agent/{extensions,skills,prompts,themes}`. Codex prompts
live at `~/.codex/prompts`; that path is from Codex's documentation and was not
exercised here, because the directory does not exist on this machine.

Two gaps matter for the workflow. Neither Pi nor Codex has subagents, so the
review fan-out cannot be dispatched the way Claude Code dispatches it. And
neither has a hook that fires on a tool call: Pi's equivalent is an extension
written in TypeScript, and Codex's plugin validator rejects a `hooks` field in
the manifest even though the manifest spec lists one.

## Plugin manifests

Three manifest formats, one shape.

| Host        | Manifest path                    | Marketplace                            |
| ----------- | -------------------------------- | -------------------------------------- |
| Claude Code | `.claude-plugin/plugin.json`     | `.claude-plugin/marketplace.json`      |
| Codex       | `.codex-plugin/plugin.json`      | `.agents/plugins/marketplace.json`     |
| Pi          | `package.json`, under a `pi` key | npm, or a git URL                      |

Codex's manifest carries `name`, `version`, `description`, `author`, and an
`interface` block of presentation metadata; `version` must be strict semver, and
the validator rejects unknown fields. Pi's is an npm package whose `pi` key
names its extensions and skill directories:

```json
{ "pi": { "extensions": ["./index.ts"], "skills": ["./skills"] } }
```

There is also a fourth, vendor-neutral format. Pi validates a `plugin.json`
carrying `$schema` of `https://agent-plugins.org/schemas/1.0.0/plugin.schema.json`
and Codex puts its marketplace under `.agents/plugins/`, the same namespace as
`.agents/skills`. The standard is young and neither host requires it, so it is
worth tracking rather than building on.

## Installation

All three install from a git repository, so one repository can serve all three:

```sh
# Claude Code
/plugin marketplace add stormlightlabs/thunderstorm

# Codex
codex plugin marketplace add stormlightlabs/thunderstorm
codex plugin add thunderstorm@stormlightlabs

# Pi
pi install git:github.com/stormlightlabs/thunderstorm
```

Codex's `marketplace add` takes `owner/repo[@ref]`, an HTTPS URL, an SSH URL, or
a local path, and `--sparse` restricts the checkout to named paths. Pi installs
from `npm:`, `git:`, an HTTPS or SSH URL, or a local path, and `-l` installs
into the project's own `.pi/settings.json` rather than the user's.
