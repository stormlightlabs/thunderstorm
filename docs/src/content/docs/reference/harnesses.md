---
title: Harness support
description: Where each coding agent loads skills from, how it dispatches work, and what the loop does about the differences.
---

Each agent loads skills from a different directory and dispatches work its own
way. What follows was verified on 2026-09-19 against Claude Code, `pi` 0.85.1,
and `codex-cli` 0.146.0.

|          | Claude Code         | Codex                         | Pi                               |
| -------- | ------------------- | ----------------------------- | -------------------------------- |
| Payload  | plugin              | marketplace plugin            | Pi package                       |
| Skills   | `.claude/skills/`   | plugin skills                 | `.agents/skills/`, `.pi/skills/` |
| Commands | `.claude/commands/` | `$thunderstorm:<skill>`       | `.pi/prompts/`                   |
| Dispatch | subagents           | built-in agents with roles    | a session per multiplexer tab    |
| Checks   | hooks, both events  | a hook on commands only       | an extension, refusals only      |
| Merging  | deny rules          | hook blocks the prefixes      | extension blocks the prefixes    |

The renderer builds all three payloads from the same skills, commands, role
definitions, and scripts. Codex packages the role definitions as TOML, and the
orchestrator passes their instructions to built-in agents. Pi carries the
Markdown definitions for reference while `tstorm dispatch` uses the same
definitions embedded in the binary.

`.agents/skills/` is read by both Codex and Pi, so one directory serves them
together. Claude Code reads only `.claude/skills/`, which is why each agent gets
its own rendered payload.

A `SKILL.md` file is the same everywhere. All three agents expect a directory
holding one, with `name` and `description` in the frontmatter.

Cursor and OpenCode Go are planned too, and neither has been assessed: no
Cursor agent was installed when these contracts were verified.

## Dispatch

Codex has a multi-agent system with four concurrency slots, counting the
orchestrator. Every agent it spawns shares one working directory, so the loop
creates a git worktree for each worker and tells it where to go before any work
starts.

Pi has no subagent mechanism. A worker there is a separate `pi` session running
in a tmux window or Zellij tab, given its provider, model, thinking level, and
tool list as command-line arguments. `--multiplexer auto` uses the session that
contains Pi; an explicit flag chooses one in a nested setup.

The Codex orchestrator reads each packaged role and starts a built-in agent with
those instructions. Reviewer roles run read-only; implementer and reviser roles
may write in their worktree.

## Only a human merges

The loop denies four commands: `gh pr merge`, `gh pr review`, `git push` and
`git merge`. Claude Code takes them as written, one deny rule each. Codex
loads a `PreToolUse` hook from the enabled plugin and denies matching shell
commands. An optional execpolicy file applies the same policy when the plugin
is disabled. Pi's package extension intercepts its `bash` tool and blocks the
same prefixes. Pi still leaves process isolation to the operating system or a
container.

[Install](/start/install/#deny-rules) explains what each harness loads.

## Choosing a model

The loop names a model on every dispatch so that an implementer and the reviewer
reading its work are never the same model. Codex makes this easy to get wrong.
A full-history fork inherits the parent's model and reasoning effort and will
reject an override, so a dispatch has to set `fork_turns` to `"none"` or to a
number before it can choose anything.
