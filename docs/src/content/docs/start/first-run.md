---
title: First run
description: The nine commands, what each one does, and how to run your first issue through the loop.
sidebar:
  order: 3
---

Install puts the loop on disk. This page says what to type once it is there.

## The nine commands

| Command | What it does |
| --- | --- |
| `/rubber-duck` | Rubber-duck a design before any code exists. |
| `/specify` | Turn an idea into a spec when issues need a decision first. |
| `/decompose` | Cut an idea or spec into issues and their sub-issues. |
| `/triage` | Rank the board into a dispatch plan for one or more threads. |
| `/thunderstorm` | Run one loop over an issue and its sub-issues. |
| `/implement` | Work a GitHub issue on an agent branch and open a PR. |
| `/rev` | Standard review pass on a PR or branch. |
| `/adv-rev` | Adversarial review pass on a PR or branch. |
| `/revise` | Address review findings on a pull request. |

## What you type, and what a run dispatches for you

A person types every one of these, and a person starts every run. Nothing in
the loop runs on a schedule.

`/thunderstorm` takes an issue that has sub-issues. It gives each one a
worktree, dispatches an implementer into it, and runs the review sequence
below over the pull request that comes back, so you type none of the review
commands yourself. An issue with no sub-issues is one unit of work: run
`/implement` on it rather than wrapping it in a run.

## Skip what you already understand

`/rubber-duck` and `/specify` both exist to settle a decision before work gets
filed. Neither earns its place when you can already write a sub-issue's stop
rule.

An idea becomes work once it has acceptance criteria someone could verify. At
that point, skip straight to `/decompose` and file the issue. A spec is for a
decision that five sub-issues would otherwise each make differently; most bug
fixes and chores never need one.

## Review, then a human merges

Every pull request the loop opens goes through the same sequence: `/rev`,
`/revise`, `/rev`, `/revise`, `/adv-rev`, `/revise`. Then it stops. Nothing in
the loop merges or approves anything, and a bare `git push` is denied to it. A
person does that when the sequence is done.

A `/thunderstorm` run drives this sequence itself. Reviewing a pull request
outside a run means running `/rev` and `/revise` in the same order yourself,
passing `first` or `second` to `/rev` so it knows which pass it is.

## Before you claim anything

`/implement`, and a `/thunderstorm` run on each sub-issue it takes, claim the
issue they work: they move it to `In Progress` on the board and assign it.
`/triage` only reads the board, but reading fails the same way a claim does
when no board is configured. Set up the board, and the rest of what the
repository keeps for itself, before running either command — see
[Configuration](/reference/configuration/).

## Run your first issue

File or find an issue with a checkable stop rule. Then:

- One unit of work, no sub-issues: run `/implement <issue-number>`. It works
  the issue on its own branch and opens a pull request.
- An issue with sub-issues: run `/thunderstorm <issue-number>`. Add `one` to
  stop after the first sub-issue has a pull request open.

What either leaves you is a reviewed pull request waiting for a person.
