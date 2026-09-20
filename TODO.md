# Roadmap

What is left, ordered by what unblocks the most. Finished work is in
[CHANGELOG.md](CHANGELOG.md).

Four issues are tracks rather than work: [#4], [#11], [#14] and, at the top of
its own tree, [#1]. Each section below is one of them, and the table under it
holds the issues you actually pick up. The board is the **Thunderstorm** track
of [project 13](https://github.com/orgs/stormlightlabs/projects/13).

## Before anything else

The workflow source lives at `workflow/` now. It is the `trps` copy, which
carries the Projects V2 migration that `docs/internal/thunderstorm.md` still
describes as labels. Settling that description, and folding in what `thndrs`
has and `trps` does not — the `release` command and skill, `sync-labels.py`,
`tui-capture.sh` — is still open and still has no issue.

## 1. Install it somewhere: [#1]

The payload builds and installs. What it has not done is run anywhere else:
install on `trps`, confirm `/decomp` and `/impl` work there, and the repository
does the thing it was created for.

GitHub holds [#1] behind [#7] and [#8], which are about the two harnesses it
does not serve. The work is finished either way, so close those or drop the
dependency when the `trps` install confirms it.

## 2. The checks: [#14]

| Issue |                                                                   |
| ----- | ----------------------------------------------------------------- |
| [#19] | Release tstorm: `go install` and Homebrew                         |
| [#16] | `tstorm check`: port the Python gates to one binary               |
| [#18] | Run a gate from a hook on Claude Code, Codex, and Pi              |
| [#15] | `tstorm board`: move board writes off labels and onto Projects V2 |

[#19] first. `tstorm render` exists and the install page already tells people
to `go install` it, which is the one instruction on that page that is not yet
true. It also blocks [#20].

Two routes, not five: `go install` from go.dev, and Homebrew. The `.deb`,
`.rpm`, `.apk` and NUR packages in the issue title are dropped. The issue is
still titled for all five.

[#16] then ports the gates, and [#18] wires one to a hook. [#18] is cheaper
than it first looked: Codex's hook events and wire format turned out to be
Claude Code's names almost exactly, so one script covers both behind thin
adapters, and Pi's TypeScript extension is the only per-harness code to
maintain. A Claude Code plugin carries hooks in `hooks/hooks.json`, verified
against a real install, so the renderer already has somewhere to put one.

[#15] last, and it is the largest of the four.

## 3. The second harness: [#11]

| Issue |                                                 |
| ----- | ----------------------------------------------- |
| [#7]  | Slash commands do not exist outside Claude Code |
| [#9]  | Merge denial is a Claude Code setting only      |
| [#8]  | Dispatch on Pi is tmux panes, not subagents     |
| [#13] | Nothing in the loop reaches Cursor              |

All four waited on [#10], which is closed.

[#7] and [#9] are small and independent of each other, and [#7] is what the
renderer stops on for both Codex and Pi. [#9] matters more than it did: an
installed payload cannot carry a permission at all, so the deny rules are a
file a repository merges by hand on every harness.

[#8] is the real work, and part of it exists: a Pi reviewer dispatch completed
on 2026-09-19 through a tmux pane with its own worktree, exit 0. What it needs
is to become something the loop drives.

[#13] waits on [#2] under the next section, which decides whether Cursor gets
roles at all.

## 4. Keeping it honest: [#4]

| Issue |                                                            |
| ----- | ---------------------------------------------------------- |
| [#2]  | Assign OpenCode Go and Cursor roles, or drop them          |
| [#5]  | Make a stolen HEAD loud in `push-verified.sh`              |
| [#6]  | Nothing checks that the review protocol agrees with itself |
| [#12] | Nothing checks prose against the tells it catalogues       |

[#12] is blocked outside this repository. The tropius baseline is 387 findings,
and on 2026-09-19 its rhetorical detectors reported nothing against prose that
was full of catalogued tells, which a reader caught by hand. Seven issues in
`stormlightlabs/trps` under **Usable as a linter** carry the calibration.
Wiring the gate before those land ships a check people learn to ignore.

[#4] also holds three issues in `stormlightlabs/thunderus`.

## 5. No parent

| Issue |                                                              |
| ----- | ------------------------------------------------------------ |
| [#3]  | Make the plans-and-ideas convention work outside thunderus   |
| [#20] | Investigate: record a real run as the home page cast         |
| [#21] | Investigate: generate the fan-out diagram from the real tree |

[#21] waited on the renderer and is free now. [#20] waits on [#19].

## Dogfood

The payload installs, so run the loop on this repository. The issues above
become its first real workload, and whatever breaks is a finding you would
otherwise meet on somebody else's repository.

[#1]: https://github.com/stormlightlabs/thunderstorm/issues/1
[#2]: https://github.com/stormlightlabs/thunderstorm/issues/2
[#3]: https://github.com/stormlightlabs/thunderstorm/issues/3
[#4]: https://github.com/stormlightlabs/thunderstorm/issues/4
[#5]: https://github.com/stormlightlabs/thunderstorm/issues/5
[#6]: https://github.com/stormlightlabs/thunderstorm/issues/6
[#7]: https://github.com/stormlightlabs/thunderstorm/issues/7
[#8]: https://github.com/stormlightlabs/thunderstorm/issues/8
[#9]: https://github.com/stormlightlabs/thunderstorm/issues/9
[#10]: https://github.com/stormlightlabs/thunderstorm/issues/10
[#11]: https://github.com/stormlightlabs/thunderstorm/issues/11
[#12]: https://github.com/stormlightlabs/thunderstorm/issues/12
[#13]: https://github.com/stormlightlabs/thunderstorm/issues/13
[#14]: https://github.com/stormlightlabs/thunderstorm/issues/14
[#15]: https://github.com/stormlightlabs/thunderstorm/issues/15
[#16]: https://github.com/stormlightlabs/thunderstorm/issues/16
[#18]: https://github.com/stormlightlabs/thunderstorm/issues/18
[#19]: https://github.com/stormlightlabs/thunderstorm/issues/19
[#20]: https://github.com/stormlightlabs/thunderstorm/issues/20
[#21]: https://github.com/stormlightlabs/thunderstorm/issues/21
