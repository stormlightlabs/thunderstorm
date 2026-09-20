---
title: Harness support
description: What each coding agent provides, and what the loop does where it provides nothing.
---

The loop needs four things from a harness: somewhere to read skills, a way to
run a check, a way to dispatch a worker, and a way to keep an implementer and a
reviewer on different models. No two agents provide all four the same way.

| | Claude Code | Codex | Pi |
| --- | --- | --- | --- |
| Skills | `.claude/skills/` | `.agents/skills/`, `.codex/skills/` | `.agents/skills/`, `.pi/skills/` |
| Commands | `.claude/commands/` | `~/.codex/prompts/` | `.pi/prompts/` |
| Dispatch | subagents | `spawn_agent` | a session per tmux pane |
| Checks | hooks | hooks | an extension |

`.agents/skills/` is read by both Codex and Pi, so one directory serves them
both. Claude Code reads neither it nor anything else shared, which is why the
payloads are rendered per harness rather than symlinked together.

## Dispatch

Codex has a real multi-agent system: four concurrency slots including the
orchestrator, and every agent sharing one working directory. That last part is
why each worker gets a git worktree created for it before it starts, rather
than being trusted to stay out of the others' way.

Pi has no subagents and does not want them. A worker there is a separate
session in a tmux pane, given its model, its thinking level, and its tool list
on the command line.

## Keeping models apart

A review is worth less when the reviewer is the same model as the author, so
the loop names a model on every dispatch. On Codex that takes care: a
full-history fork inherits the parent's model and refuses to override it, so a
dispatch has to opt out of inheriting context before it can choose.
