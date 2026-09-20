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
describes as labels. Settling that description is still open, and so is folding in what `thndrs`
has and `trps` does not: the `release` command and skill, `sync-labels.py`,
and `tui-capture.sh`. Neither has an issue.

## Publishing waits a week

Not before 2026-09-26, and later if the work below is not done.

A git tag does not publish anything by itself. The first fetch through
proxy.golang.org does, and that version is then immutable and permanent: the
README it carries is what pkg.go.dev serves for it from then on, so the
README, the install page and the payload are right before the tag.

Nothing is blocked in the meantime. `go install` reaches GitHub directly
without the proxy, checked on 2026-09-19:

```sh
GOPROXY=direct go install github.com/stormlightlabs/thunderstorm/cmd/tstorm@main
```

That built and ran, reporting `v0.0.0-20260920015401-fa6318c9d4bb`. Until the
tag, that command and a GitHub release are the two routes, and both are
[#19].

## 1. Install it somewhere: [#1]

The payload builds and installs on Claude Code. What it has not done is run
anywhere else: install on `trps`, confirm `/decomp` and `/impl` work there.

GitHub holds [#1] behind [#7] and [#8]. [#7] is done on this branch and closes
when it lands, which leaves [#8].

## 2. The checks: [#14]

| Issue |                                                                    |
| ----- | ------------------------------------------------------------------ |
| [#19] | Release tstorm: `go install` and Homebrew                          |
| [#16] | `tstorm check`: port the Python gates to one binary                |
| [#18] | Run a gate from a hook on Claude Code, Codex, and Pi               |
| [#15] | `tstorm board`: move board writes off labels and onto Projects V2  |

[#19] first, and its GitHub half before its go.dev half. A release carrying
built binaries is what a hook on any harness can fetch, and it needs neither
the module proxy nor a permanent tag. Homebrew and the tagged module follow at
the end of the week. [#19] also blocks [#20].

[#16] then ports the gates, and [#18] wires one to a hook. [#18] is cheaper
than it first looked: Codex's hook events and wire format turned out to be
Claude Code's names almost exactly, so one script covers both behind thin
adapters, and Pi's TypeScript extension is the only per-harness code to
maintain. A Claude Code plugin carries hooks in `hooks/hooks.json`, verified
against a real install, so the renderer already has somewhere to put one.

[#15] last, and it is the largest of the four.

## 3. The other three harnesses: [#11]

| Issue |                                                 |
| ----- | ----------------------------------------------- |
| [#8]  | Dispatch on Pi is tmux panes, not subagents     |
| [#9]  | Merge denial is a Claude Code setting only      |
| [#13] | Nothing in the loop reaches Cursor              |

These three and [#7] all waited on [#10], which is closed. They are the
week's work, and [#7] is already done on this branch: commands render into
`prompts/` for Codex and Pi, so the renderer stops on the review fan-out for
both.

Both harnesses install from GitHub already, checked on 2026-09-19:
`codex plugin marketplace add owner/repo[@ref]` then `codex plugin add`, and
`pi install git:github.com/user/repo`. Neither is waiting on a delivery
mechanism. They are waiting on a payload worth delivering.

[#9] matters more than it did: an installed payload carries no permission on
any harness, so the deny rules are a file a repository merges by hand
everywhere.

[#8] is what both payloads now wait on, and part of it exists: a Pi reviewer
dispatch completed on 2026-09-19 through a tmux pane with its own worktree,
exit 0. What it needs is to become something the loop drives.

[#13] is cheaper than it looked. Cursor reads `.agents/skills/`, the same
directory the Codex and Pi payloads need, so its skills cost a target in the
renderer rather than a third copy of the source. It has subagents as of its
2.4 release. All of that is from Cursor's documentation, and nothing here has
run it, which is the first thing [#13] should fix. [#2] still decides whether
Cursor and OpenCode Go carry roles at all.

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
