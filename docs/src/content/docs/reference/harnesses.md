---
title: Harness support
description: Where each coding agent loads skills from, how it dispatches work, and what the loop does about the differences.
---

Each agent loads skills from a different directory and dispatches work its own
way. What follows was verified on 2026-09-19 against Claude Code, `pi` 0.85.1,
and `codex-cli` 0.146.0.

|          | Claude Code         | Codex                               | Pi                               |
| -------- | ------------------- | ----------------------------------- | -------------------------------- |
| Payload  | **installs today**  | no                                  | no                               |
| Skills   | `.claude/skills/`   | `.agents/skills/`, `.codex/skills/` | `.agents/skills/`, `.pi/skills/` |
| Commands | `.claude/commands/` | `~/.codex/prompts/`                 | `.pi/prompts/`                   |
| Dispatch | subagents           | `spawn_agent`                       | a session per tmux pane          |
| Checks   | hooks               | hooks                               | an extension                     |

The table is where each agent *would* read a payload, not a claim that one
exists. Only Claude Code has one: a command has nowhere to go on Codex or Pi,
and Pi has no subagents to dispatch the review passes with, so the renderer
refuses to build for either rather than installing half a loop.

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
in a tmux pane, given its model, thinking level, and tool list as command-line
arguments.

## Choosing a model

The loop names a model on every dispatch so that an implementer and the reviewer
reading its work are never the same model. Codex makes this easy to get wrong.
A full-history fork inherits the parent's model and reasoning effort and will
reject an override, so a dispatch has to set `fork_turns` to `"none"` or to a
number before it can choose anything.
