# Changelog

Notable changes to thunderstorm: the loop a repository installs, and the
`tstorm` binary that carries its checks. Changes to this repository's own site
and tests are left out.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and the project follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

Nothing is released yet, and no version is tagged. Everything below is on
`main`.

### Added

- `tstorm render --target claude|codex|pi|cursor` builds a harness's payload
  from one canonical source. An artifact the target cannot carry stops the
  render, which writes nothing and names every gap with the issue that would
  close it ([#17]).
- `tstorm render --check` reports whether the payload on disk still matches the
  source, and writes nothing.
- The workflow itself, under `workflow/`: twelve skills, nine commands and four
  agent definitions, with a manifest saying what each artifact needs of a
  harness ([#17]).
- A Claude Code payload under `payloads/claude`, and a marketplace at
  `.claude-plugin/marketplace.json` that installs it ([#1]).
- Codex and Pi payloads. Codex gets a marketplace plugin with packaged roles
  and a command-policy hook; Pi gets an installable package with prompts,
  skills, and role dispatch through `tstorm`.
- `/forecast` as a second name for `/triage`, which describes what the stage
  produces rather than how a hospital sorts casualties.
- `tstorm dispatch --role <role> --worktree <dir> --model <id>` runs one role
  as a Pi session in a tmux window or Zellij tab and prints its report. Provider,
  model, reasoning level, and multiplexer are configurable. The transcript
  directory holds the command, event stream, and exit status ([#8]).
- `docs/internal/models.md` states what a harness must provide before it can
  carry a role ([#10]).
- `tstorm check commit-message`, `tstorm check frontmatter` and `tstorm check
  isolation` are the loop's gates, with `tstorm push` and `tstorm ulid` beside
  them. Each exits 0 when it found nothing, 1 when it found something, and 2
  when the gate itself could not run, so a hook can tell bad prose from an
  unreadable file ([#16]).
- `tstorm push --branch <branch>` refuses to push from any other branch. Two
  workers sharing a checkout share one `HEAD`, so the second to create a branch
  carries the first's staged work onto it, and nothing downstream notices
  ([#5]).
- `.tstorm.json` at a repository's root names the tree the frontmatter gate
  walks, so the convention travels with the payload instead of assuming one
  repository's `internal/` ([#3]).
- `tstorm hook` answers a harness's tool hook, reading the event on stdin and
  writing the reply on stdout. A document written under the configured tree is
  checked for the frontmatter an issue cites it by, and what it finds is
  reported rather than refused ([#18]).
- `tstorm board` reads and writes the GitHub Projects board: what is queued,
  one issue's status, a claim, a transition, sub-issues, `blocked_by`
  dependencies, and filing new work. Every read takes `--json`, a write says
  what it changed, and a claim somebody else won exits 1 rather than being
  retried ([#15]).
- `.tstorm.json` also names the board: the project and its owner, the status
  field, the option name for each of the three states, and the field that
  separates one repository's work from another's on a shared project. A
  repository that configures none of it gets an error naming what is missing
  ([#15]).
- This repository installs its own payload. `.claude/settings.json` declares
  the marketplace beside it and enables the plugin at project scope, so a
  session here runs the loop it renders and a re-render reaches the next
  session ([#25]).
- The payload registers that gate itself. Claude Code reads it from
  `hooks/hooks.json`; Pi's package extension runs the same script, since Pi has
  no hooks. The script finds the binary through `TSTORM_BIN`, the payload's own
  `bin/`, then `PATH`, and warns rather than blocking when it finds none
  ([#18]).

### Changed

- `github-board` says which board operation to run and stops carrying the
  transport tables, the `gh project` invocations and the commentary on losing a
  race, which is work `tstorm board` now does ([#15]).
- The skills run the gates a repository's `AGENTS.md` names, rather than the
  Cargo commands they were written against ([#1]).
- Payload prose distinguishes the harness directory inside the repository being
  worked on from the directory an installed payload landed in ([#1]).
- Hook registrations render to `hooks/hooks.json`, which a plugin install
  reads. A `settings.json` inside a payload is read by nothing.
- The rules that stop a session merging or approving its own work render per
  harness: deny rules for Claude Code, a Codex plugin hook, and a Pi extension
  that blocks the same command prefixes ([#1], [#9]).
- The Go module path is `github.com/stormlightlabs/thunderstorm`, matching the
  repository it is fetched from, so `go install` resolves it. The binary is
  still `tstorm`.
- Commands render into `prompts/` for Codex and Pi ([#7]). Codex invokes the
  plugin skills and passes packaged TOML roles to built-in agents; Pi runs each
  role in a tmux window or Zellij tab through `tstorm dispatch` ([#8]).
- `docs/internal/thunderstorm.md` says what starts a review pass on each
  harness, and `docs/internal/hosts.md` what a role may reach on Pi and what
  each harness enforces ([#8], [#9]).

### Removed

- The `SessionStart` hook that warmed a Cargo registry. The payload ships no
  hook at all ([#1]).
- `github-board`'s `references/dependencies.md`, which held the `curl` calls
  for issue dependencies. `tstorm board blocked-by` makes them ([#15]).
- The five Python and shell checks under `workflow/scripts/`, and the copies
  each payload carried. A repository got them by copying, got a fix by copying
  again, and needed a Python on `PATH` to run one at all. `tstorm check` runs
  them now, and the skills call it by name ([#16]).

[Unreleased]: https://github.com/stormlightlabs/thunderstorm/commits/main
[#1]: https://github.com/stormlightlabs/thunderstorm/issues/1
[#3]: https://github.com/stormlightlabs/thunderstorm/issues/3
[#5]: https://github.com/stormlightlabs/thunderstorm/issues/5
[#7]: https://github.com/stormlightlabs/thunderstorm/issues/7
[#8]: https://github.com/stormlightlabs/thunderstorm/issues/8
[#9]: https://github.com/stormlightlabs/thunderstorm/issues/9
[#10]: https://github.com/stormlightlabs/thunderstorm/issues/10
[#15]: https://github.com/stormlightlabs/thunderstorm/issues/15
[#16]: https://github.com/stormlightlabs/thunderstorm/issues/16
[#18]: https://github.com/stormlightlabs/thunderstorm/issues/18
[#17]: https://github.com/stormlightlabs/thunderstorm/issues/17
[#25]: https://github.com/stormlightlabs/thunderstorm/issues/25
