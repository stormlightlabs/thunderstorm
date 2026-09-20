# Roadmap

Ordered by what unblocks the most. Every item links to its issue; the board
itself is the **Thunderstorm** track of [project
13](https://github.com/orgs/stormlightlabs/projects/13).

## Before anything else

The workflow source moved in under [#17] and lives at `workflow/`. It is the
`trps` copy, which carries the Projects V2 migration that
`docs/internal/thunderstorm.md` still describes as labels. Settling that
description, and folding in what `thndrs` has and `trps` does not, is still
open and still has no issue.

## 1. Render and install

Done. `docs/internal/models.md` says what a harness must provide before it can
carry a role ([#10]), `tstorm render --target claude` builds the payload from
`workflow/` ([#17]), and a marketplace at `.claude-plugin/marketplace.json`
installs it ([#1]).

[#1] is written but cannot close: GitHub holds it behind [#7] and [#8], which
are about the harnesses it does not yet serve. Close those, or drop the
dependency, and the commit's keyword takes effect.

Two things came out of that work rather than into it. The workflow's one hook
warmed a Cargo registry, so it stayed with the repository it came from and the
payload ships no hook of its own. And Codex and Pi render nothing, because
neither has a place for a command ([#7]) and Pi has no subagents ([#8]).

What is left is to run it. Install on `trps`, confirm `/decomp` and `/impl`
work there, and the repository does the thing it was created for.

## 2. The checks

| Issue |                                                       |
| ----- | ----------------------------------------------------- |
| [#19] | Release tstorm: goreleaser, brew, linux packages, NUR |
| [#16] | `tstorm check`: port the Python gates to one binary   |
| [#18] | Run a gate from a hook on Claude Code, Codex, and Pi  |

[#19] comes before [#16] so that a binary
exists to install once there are commands worth installing.

[#18] is cheaper than
it first looked. Codex's hook events and wire format turned out to be Claude
Code's names almost exactly, so one script covers both behind thin adapters, and
Pi's TypeScript extension is the only per-harness code to maintain.

## 3. The second harness

| Issue |                                                 |
| ----- | ----------------------------------------------- |
| [#7]  | Slash commands do not exist outside Claude Code |
| [#9]  | Merge denial is a Claude Code setting only      |
| [#8]  | Dispatch on Pi is tmux panes, not subagents     |
| [#11] | Run thunderstorm on a second harness            |

[#7] and [#9] are small and
independent of each other.
[#8] is the real work,
and part of it exists: a Pi reviewer dispatch completed on 2026-09-19 through a
tmux pane with its own worktree, exit 0. What it needs is to become something
the loop drives.

## 4. The rest

Take these in whatever order the friction demands.

| Issue |                                                                   |
| ----- | ----------------------------------------------------------------- |
| [#15] | `tstorm board`: move board writes off labels and onto Projects V2 |
| [#12] | Nothing checks prose against the tells it catalogues              |
| [#3]  | Make the plans-and-ideas convention work outside thunderus        |
| [#5]  | Make a stolen HEAD loud in `push-verified.sh`                     |
| [#6]  | Nothing checks that the review protocol agrees with itself        |
| [#2]  | Assign OpenCode Go and Cursor roles, or drop them                 |
| [#13] | Nothing in the loop reaches Cursor                                |
| [#20] | Investigate: record a real run as the home page cast              |
| [#21] | Investigate: generate the fan-out diagram from the real tree      |

## Two things worth knowing

[#12] is blocked
outside this repository. The tropius baseline is 387 findings, and on 2026-09-19
its rhetorical detectors reported nothing against prose that was full of
catalogued tells, which a reader caught by hand. Seven issues in
`stormlightlabs/trps` under **Usable as a linter** carry the calibration. Wiring
the gate before those land ships a check people learn to ignore.

Dogfood now. The payload installs, so run the loop on this repository. The
remaining issues become its first real workload, and whatever breaks is a
finding you would otherwise meet on somebody else's repository.

[#1]: https://github.com/stormlightlabs/thunderstorm/issues/1
[#2]: https://github.com/stormlightlabs/thunderstorm/issues/2
[#3]: https://github.com/stormlightlabs/thunderstorm/issues/3
[#5]: https://github.com/stormlightlabs/thunderstorm/issues/5
[#6]: https://github.com/stormlightlabs/thunderstorm/issues/6
[#7]: https://github.com/stormlightlabs/thunderstorm/issues/7
[#8]: https://github.com/stormlightlabs/thunderstorm/issues/8
[#9]: https://github.com/stormlightlabs/thunderstorm/issues/9
[#10]: https://github.com/stormlightlabs/thunderstorm/issues/10
[#11]: https://github.com/stormlightlabs/thunderstorm/issues/11
[#12]: https://github.com/stormlightlabs/thunderstorm/issues/12
[#13]: https://github.com/stormlightlabs/thunderstorm/issues/13
[#15]: https://github.com/stormlightlabs/thunderstorm/issues/15
[#16]: https://github.com/stormlightlabs/thunderstorm/issues/16
[#17]: https://github.com/stormlightlabs/thunderstorm/issues/17
[#18]: https://github.com/stormlightlabs/thunderstorm/issues/18
[#19]: https://github.com/stormlightlabs/thunderstorm/issues/19
[#20]: https://github.com/stormlightlabs/thunderstorm/issues/20
[#21]: https://github.com/stormlightlabs/thunderstorm/issues/21
