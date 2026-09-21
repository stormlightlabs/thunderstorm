# Roadmap

What is left, in the order to do it. Finished work is in
[CHANGELOG.md](CHANGELOG.md), and the board is the **Thunderstorm** track of
[project 13](https://github.com/orgs/stormlightlabs/projects/13).

[#4], [#11] and [#14] are tracks rather than work. Everything below sits under
one of them or under nothing.

## Before the tag

`v0.1.0` comes no earlier than 2026-09-26, and later if the work below is not
done. `v0.1.0-rc.1` is tagged and is not that tag: a prerelease publishes
binaries without becoming what `@latest` resolves to.

A git tag publishes nothing by itself: the first fetch through
proxy.golang.org does, and that version is immutable, so the README it carries
is what pkg.go.dev serves from then on. The README, the install page and the
payload are what a tag freezes.

Nothing waits on the tag. `go install` reaches GitHub without the proxy,
checked on 2026-09-19:

```sh
GOPROXY=direct go install github.com/stormlightlabs/thunderstorm/cmd/tstorm@main
```

That built and ran, reporting `v0.0.0-20260920015401-fa6318c9d4bb`.

## The sequence

1. [#19]'s second half. GoReleaser, the release workflow and the snapshot
   build landed, and `v0.1.0-rc.1` proves the path: a prerelease publishes
   binaries and `go install @latest` keeps resolving to `main`, so the README
   and the install page stay editable. What is left is `v0.1.0`, which the
   module proxy freezes, and the Homebrew tap the cask is waiting for.

2. A real run outside Claude Code, which is what [#11] waits for. Install on
   `trps` through the marketplace, which takes a local path, confirm `/decomp`
   and `/impl`, then take one issue through implementation and review on Codex
   or Pi.

3. [#18], one gate on a hook. Codex's hook events and wire format are Claude
   Code's names almost exactly, so one script covers both behind thin adapters
   and Pi's TypeScript extension is the only per-harness code to maintain. A
   Claude Code plugin carries hooks in `hooks/hooks.json`, verified against a
   real install.

4. [#2], then [#13]. [#2] decides whether Cursor and OpenCode Go carry roles,
   and [#13] is held behind it on the board. Cursor reads `.agents/skills/`,
   the directory Codex and Pi already need, so its skills cost a target in the
   renderer rather than a copy of the source. Nothing here has run Cursor,
   which is the first thing [#13] should fix.

5. [#6] and [#3]. The review protocol agrees with itself only by inspection,
   and a document is still named after its filename, so `hooks/plan.md` and
   `mcp/plan.md` are both called `plan`.

6. [#21], then [#20]. [#21] waited on the renderer and is free. [#20] waits on
   the release in step 1.

## Parked

Two things have no issue. `docs/internal/thunderstorm.md` still describes the
board as labels, which the Projects V2 migration replaced. And what `thndrs`
has that `workflow/` does not has not been folded in: the `release` command and
skill, `sync-labels.py`, and `tui-capture.sh`.

[#4] also holds three issues in `stormlightlabs/thunderus`.

## The prose gate

[#12] landed with three rules muted: `structure.short_punchy_fragments`,
`repetition.anaphora_abuse` and `formatting.unicode_decoration`, which
together were 191 of the 387 findings measured on 2026-09-19 and read as noise
against prose a reader called clean. Each is muted with its reason in
`.tstorm.json`, and the list shrinks as the **Usable as a linter** issues in
`stormlightlabs/trps` land. Raise the pinned revision in
`.github/workflows/check.yml` when they do, and rerun the count.

## Run it here

The loop is installed here, at project scope, from the payload this repository
renders ([#25]). The issues above are its first real workload, and whatever
breaks is a finding you would otherwise meet on somebody else's repository.

[#2]: https://github.com/stormlightlabs/thunderstorm/issues/2
[#3]: https://github.com/stormlightlabs/thunderstorm/issues/3
[#4]: https://github.com/stormlightlabs/thunderstorm/issues/4
[#6]: https://github.com/stormlightlabs/thunderstorm/issues/6
[#11]: https://github.com/stormlightlabs/thunderstorm/issues/11
[#12]: https://github.com/stormlightlabs/thunderstorm/issues/12
[#13]: https://github.com/stormlightlabs/thunderstorm/issues/13
[#14]: https://github.com/stormlightlabs/thunderstorm/issues/14
[#18]: https://github.com/stormlightlabs/thunderstorm/issues/18
[#19]: https://github.com/stormlightlabs/thunderstorm/issues/19
[#20]: https://github.com/stormlightlabs/thunderstorm/issues/20
[#21]: https://github.com/stormlightlabs/thunderstorm/issues/21
[#22]: https://github.com/stormlightlabs/thunderstorm/issues/22
[#23]: https://github.com/stormlightlabs/thunderstorm/issues/23
[#25]: https://github.com/stormlightlabs/thunderstorm/issues/25
