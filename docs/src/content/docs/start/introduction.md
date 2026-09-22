---
title: Introduction
description: What a thunderstorm run does, what gets installed, and why it is a package.
sidebar:
  order: 1
---

## Our loop

A run covers one issue and the sub-issues under it. It claims the issue on the
board, creates a git worktree for each worker, takes each change through a
fixed sequence of review passes, and stops once a pull request is open. A
person starts every run, and a person merges it.

## `tstorm`

The skills describe each stage and nine commands start them, from
`/rubber-duck` to `/revise`. [First run](/start/first-run/) lists them and
walks through one. `tstorm` is the small app that renders the payloads, runs
the gates, answers the harness hooks, reads and writes the board, and starts a
session per role. The
[checks reference](/reference/checks/) says what each check reads and what its
exit code means, and the [harness reference](/reference/harnesses/) says what
each agent gets.

### Why does this exist?

The loop began as six directories inside a single repository. A second
repository could copy them, and the two copies drifted within weeks.
Installing gives both of them the same fix.

The checks are separate from the skills for a different reason. A skill is an
instruction, and when an agent ignores one, nothing records that it happened.
A check that exits non-zero leaves a result someone can read afterward.

## Loop engineering

The loop is [ReAct](https://arxiv.org/abs/2210.03629)'s (Yao et al., ICLR
2023): the model acts, reads what happened, and acts again. What separates one
coding agent from another is the middle step. Here it is a test result, an
exit code from `tstorm check`, or a comment from a review pass that ran as its
own session.

Letting the model grade itself was the cheaper option and it does not hold.
[Huang et al.](https://arxiv.org/abs/2310.01798) (ICLR 2024) had models revise
their own reasoning with no outside signal: accuracy did not improve, and on
some tasks it fell.

Each agent gets a payload of its own because of what
[SWE-agent](https://arxiv.org/abs/2405.15793) (Yang et al., NeurIPS 2024)
measured. Giving a model an interface built for it, rather than the one a
person uses, took SWE-bench to 12.5% pass@1 where non-interactive models had
been far below that. Skills, commands and hooks are that interface here, and
no two agents read them from the same place in the same format.

<!-- trps-ignore-start -->

Fixing the stages and their order makes a run a workflow in the sense
[Anthropic](https://www.anthropic.com/engineering/building-effective-agents)
(Schluntz and Zhang, 2024) separates from an agent, which directs its own
process; the model's discretion lives inside a stage. That post argues for
composing simple patterns over adopting a framework, which is the case for a
loop written as skills a person can read.

<!-- trps-ignore-end -->

Fan-out stops at five sub-issues, each with its own worktree and its own
review sequence. [Cognition](https://cognition.com/blog/dont-build-multi-agents)
(Yan, 2025) is the reason for the ceiling: parallel agents make decisions none
of the others can see, and the conflicts surface late, in the merge.
