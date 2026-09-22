---
name: using-thunderstorm
last_updated: 2026-09-19
id: 01M2XBVYTZDXTW42ZSCFEEW0QS
---

> Copied from `stormlightlabs/thunderus` on 2026-09-19 with the rest of the
> workflow. The identifier is unchanged so that open issues citing it still
> resolve. It is written for a Claude Code operator; what changes on another
> harness is stormlightlabs/thunderstorm#11.

# Using thunderstorm

What to type and when. `internal/thunderstorm.md` holds the protocol and the
reasoning behind it; this is the operator's side of the same thing.

## The whole loop

```text
/rubber-duck    talk it through     -> internal/ideas/<name>.md
/decompose      file it             -> an issue with sub-issues
/triage         decide what is next -> a ranked list, in chat
/thunderstorm   run it              -> pull requests, reviewed
you             merge in GitHub     -> edge
```

The middle two are optional. Filing work you already understand skips
`/rubber-duck`, and running the only thing that is queued skips `/triage`.

## The three shapes

| Shape | Looks like | What you do with it |
| --- | --- | --- |
| Milestone | not an issue | Nothing directly. It groups issues. |
| Issue with sub-issues | has children | **`/thunderstorm <n>`** |
| Sub-issue | has no children | `/implement <n>` for one on its own |

`/thunderstorm` takes the middle row. A milestone is not an issue, so there is
no way to dispatch one and no rule to remember about it.

An issue holds at most five open sub-issues, because a run keeps every one of
them in a single context and loses the thread past that. When work outgrows
five, it becomes two issues in one milestone.

## What to type

Starting from nothing, `/rubber-duck <topic>` thinks it through and writes an
idea file. `/decompose <that file>` files issues from it. Then
`/thunderstorm <n>`.

Starting from an issue you already filed, `/thunderstorm <n>` if it has
sub-issues and `/implement <n>` if it does not.

When you are not sure what to work on, `/triage` reads the whole board, ranks
it, and ends by printing the command to type next. It writes nothing.

When a pull request comes back with findings, the run handles them. You step in
where it stops and asks.

### On each harness

Claude Code and Pi load the rendered command files. Codex custom prompts are
deprecated, so its marketplace plugin exposes the skills directly.

| Harness     | Where a command lands      | Installs today |
| ----------- | -------------------------- | -------------- |
| Claude Code | `commands/` in the payload | yes            |
| Pi          | `prompts/` in the package  | yes            |
| Codex       | `$thunderstorm:<skill>`     | plugin         |
| Cursor      | `.cursor/commands/`        | no             |

Pi reads prompt templates from a package's `prompts/` directory, and its
format is the one already written: `description` and `argument-hint` in the
frontmatter, `$ARGUMENTS` in the body, filename as the command name. Nothing
had to be translated.

On Codex, qualify the skill with the plugin name and state the argument in the
request:

```text
$thunderstorm:thunderstorm Run issue 75
$thunderstorm:implement Work issue 81
```

The orchestrator reads the packaged TOML role and passes its instructions to a
built-in Codex agent. Installing the plugin does not add persistent custom
agents to `~/.codex/agents/`.

Cursor's `.cursor/commands/` is documented but unexercised here. Cursor is
retiring commands in favour of skills, so check before relying on it.

Where a harness has no command mechanism, name the skill in the prompt.

## What a run does

`/thunderstorm <n>` claims a sub-issue, gives it a worktree, dispatches an
implementer, and opens a pull request. Then three review passes with a fix pass
after each. Then it reports and moves to the next sub-issue.

It stops when every sub-issue is terminal, one hits `status:blocked`, an edit
pass runs out of rounds, or the work needs a decision the issues do not record.
Add `one` to the argument, as `/thunderstorm 75 one`, to stop after the first
sub-issue. A stop for any other reason is a bug.

## Merging is yours

Nothing under `.claude/` merges a pull request. `settings.json` denies
`gh pr merge`, `gh pr review`, `git merge`, and pushes to `edge` and `main`, so
an agent cannot merge or approve its own work even when told to.

Merge in GitHub, squash. The pull request title and body become the commit
verbatim, so what you see in the merge box is what lands in `git log`. Then move
the issue to `status:verify` yourself; nothing does it for you.

## Skipping most of it

A small fix you already understand is `/implement <n>`: no milestone, no run,
no triage. Something you want to think about is `/rubber-duck`, which writes a
file and files nothing. A board you have lost track of is `/triage`, which
answers and changes nothing.

The full sequence is for work spanning several pull requests that you would
rather not hold in your head.

## Milestones

A milestone groups the issues of one body of work. Its description takes
markdown and holds the order they run in and what crosses them: a dependency
between two, a file both write. A run cannot see any of that from inside one
issue.

```sh
gh issue list --milestone "UI Polish"
gh issue edit 75 --milestone "UI Polish"
gh api repos/stormlightlabs/thunderus/milestones          # create or edit
```

There is no `gh milestone` command, so creating one goes through `gh api`.
Where a plan document under `internal/features/` covers the same work, it names
the milestone in its frontmatter.

## The status labels, briefly

`queued` is ready and unowned. `claimed` means a run has it. `review` means a
pull request is open. `verify` means it is on `edge` and you have not confirmed
it. `done` means it shipped from `main`. `blocked` needs a `blocked:*` reason
beside it, and ends a run.

You set `verify` after merging. A run sets the rest.

## Where things are

| Want | Look at |
| --- | --- |
| Why a rule exists | `internal/thunderstorm.md` |
| What a command does | `.claude/commands/<name>.md` |
| How a stage works | `.claude/skills/<name>/SKILL.md` |
| What models run which role | `internal/models.md` |
| Length targets | `.claude/skills/writing-docs/SKILL.md` |
