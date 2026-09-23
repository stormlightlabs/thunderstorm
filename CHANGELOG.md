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

- `tstorm install` puts the loop in a repository, rendering the payload from
  the workflow the binary carries. It also merges the commands the workflow
  reserves for a person into the repository's `permissions.deny`, registers the
  gate hook against the same settings file, and writes a `.tstorm.toml` from
  what the flags name. `--check` reports every step and writes nothing ([#36],
  [#35]).
- An install into a repository that already uses Claude Code needs no flag: it
  writes its own files, leaves every other file alone, and stops only on a
  path the payload itself writes, naming each one. `--replace` overwrites
  those and names what it replaced. `--adopt`, which took a directory over and
  reported a count, is gone.
- `tstorm uninstall` takes the loop back out: the files the marker says the
  payload owns, and the deny rules and gate registrations out of the settings
  file. A file the repository put in the payload directory stays, a directory
  goes only once nothing is left in it, and `.tstorm.toml` is left alone.
- `tstorm update` moves an installed payload to what the binary carries,
  naming the version it moves from, and merges the settings again so a newly
  reserved command reaches a repository that installed months ago. It ends by
  saying when a newer release exists, and `--self` takes it, verified against
  the checksums published beside it.
- The workflow source travels inside the binary, so a render needs no
  checkout. `--source` still names a directory, which is how the loop itself
  is worked on.
- A build from a checkout names the tag it descends from and the commit under
  it, `v0.1.0-rc.1+g1969674`. `tstorm check version` compares a release tag to
  that constant as well as to the payload's version.
- The JSON tstorm reads from a person takes comments and trailing commas,
  which covers a `.tstorm.json` written before TOML and the workflow manifest.
- `tstorm render --target claude|codex|pi|cursor` builds a harness's payload
  from one canonical source. An artifact the target cannot carry stops the
  render, which writes nothing and names every gap with the issue that would
  close it ([#17]).
- `tstorm render --check` reports whether the payload on disk still matches the
  source, and writes nothing.
- `.tstorm-payload` records the version that wrote a payload, beside the files
  it owns. A marker written before that line reads as an unknown version and
  updates anyway.
- The workflow itself, under `workflow/`: twelve skills, nine commands and four
  agent definitions, with a manifest saying what each artifact needs of a
  harness ([#17]).
- A Claude Code payload under `payloads/claude`, and a marketplace at
  `.claude-plugin/marketplace.json` that installs it ([#1]).
- Codex and Pi payloads. Codex gets a marketplace plugin with packaged roles
  and a command-policy hook; Pi gets an installable package with prompts,
  skills, and role dispatch through `tstorm`.
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
- A tagged release publishes `tstorm` for linux and macOS on amd64 and arm64,
  with checksums, built by GoReleaser with the version and commit set at link
  time so `tstorm version` names the build it came from. Every branch builds a
  snapshot, so a release does not fail for a reason a branch could have
  caught. The Homebrew cask is written and held until the tap exists ([#19]).
- A commit whose message fails the shape gate is refused before git takes it.
  The hook reads the message out of the `git commit` a session is about to
  run, denies on a shape a reader cannot recover from, and puts the prose
  findings to the person instead of deciding for them. A message it cannot
  read, an editor commit among them, reaches `.githooks/commit-msg` as before
  ([#26]).
- One hook script answers both events, so `hooks/check-documents.sh` is
  `hooks/gate.sh` and the manifest's hook artifact carries a list of
  registrations. `tstorm hook` picks the gate by event name ([#26]).
- `tstorm check version` fails a payload that changed since the last tag
  without a version bump, and a release whose tag disagrees with the manifest.
  A harness caches an installed plugin by version and reports it current while
  that string is unchanged, so the bump is what carries a re-render to a
  machine that already installed one ([#34]).
- `tstorm check policy` compares a repository's settings against the commands
  the workflow reserves for a person, names every one that is not denied, and
  says what to add. The expected list comes from the manifest, or from a
  rendered payload's `settings.json` with `--expected`, which is what an
  installed repository has ([#27]).
- `tstorm check prose` runs [tropius] over a file or a tree and reports the
  writing tells it finds, minus the rules `.tstorm.json` mutes with a reason
  beside each one. The detection is tropius's; what this repository decides is
  which of its rules it can read yet. Over `README.md`, `CHANGELOG.md`,
  `TODO.md`, `AGENTS.md`, `docs/` and `workflow/` it reports 24 findings at
  revision `5e9333b`, against the 387 the same tree measured on 2026-09-19
  with nothing tuned ([#12]).
- A write to a Markdown file reports the same findings through the hook, so
  the writing pass is asked for at the moment it is owed. It reports and never
  refuses: a trope count is not a quality score ([#12]).
- Tropius missing is a warning and exit 0, with `TRPS_BIN` naming it where it
  is not on `PATH` ([#12]).
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

- `tstorm check frontmatter` names a document for its whole path rather than
  its filename, so `features/mcp/plan.md` is `features-mcp-plan` and two plans
  under two feature directories no longer ask for one name. The per-feature
  waiver list is gone with it, and two documents whose paths fold to one name
  fail the check the way two carrying one identifier already did. The skills
  that write plans and ideas name the directory `.tstorm.toml` configures
  instead of `internal/` ([#3]).

- `tstorm` reads `.tstorm.toml` first and the older `.tstorm.json` second, in
  the directory a command runs in and then each directory above it, with the
  same settings under the same names. The reason a prose rule is muted is a
  comment beside the rule rather than a `why` key. A repository that installed
  the loop before this needs no conversion ([#28]).

- Both review passes read the change's communication, not only the prose in
  its diff: the pull request title and body, the branch's commit messages, and
  the comments the run posted, with `tstorm check prose` run over what it
  reads. A catalogued tell is `medium` where it was a nit, and a pattern
  repeated through a document is one finding naming the pattern.

- `render --out` defaults to the harness's own directory: `.claude` for Claude
  Code, `.codex` for Codex, `.pi` for Pi. Installing the workflow into a
  repository is `tstorm render --target claude` and nothing else, and building
  the payloads this repository publishes is the case that names `--out`.

- `render --out --adopt` takes over a directory carrying no marker, which is
  every repository that installed the workflow by copying. It replaces the
  files the payload writes, names the files it leaves so an older payload's
  leftovers can be deleted, and writes the marker that makes the next render
  ordinary. With `--check` it reports what adopting would do and writes
  nothing ([#23]).
- `render --out` writes into a directory the repository also keeps files in.
  The marker decides what a render may replace and delete; a settings file, a
  hook or a worktrees directory beside the payload is left where it is and
  counted in the summary. A path the repository owns and the payload also
  wants stops the render and names the file ([#22]).
- `settings.json` is written where there is none and left alone where there is
  one. The permissions block is the repository's, merged by hand, so a render
  that replaced it would take the deny rules with it ([#22]).
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
[tropius]: https://github.com/stormlightlabs/trps
[#1]: https://github.com/stormlightlabs/thunderstorm/issues/1
[#3]: https://github.com/stormlightlabs/thunderstorm/issues/3
[#5]: https://github.com/stormlightlabs/thunderstorm/issues/5
[#7]: https://github.com/stormlightlabs/thunderstorm/issues/7
[#8]: https://github.com/stormlightlabs/thunderstorm/issues/8
[#9]: https://github.com/stormlightlabs/thunderstorm/issues/9
[#10]: https://github.com/stormlightlabs/thunderstorm/issues/10
[#12]: https://github.com/stormlightlabs/thunderstorm/issues/12
[#15]: https://github.com/stormlightlabs/thunderstorm/issues/15
[#16]: https://github.com/stormlightlabs/thunderstorm/issues/16
[#18]: https://github.com/stormlightlabs/thunderstorm/issues/18
[#19]: https://github.com/stormlightlabs/thunderstorm/issues/19
[#17]: https://github.com/stormlightlabs/thunderstorm/issues/17
[#22]: https://github.com/stormlightlabs/thunderstorm/issues/22
[#23]: https://github.com/stormlightlabs/thunderstorm/issues/23
[#25]: https://github.com/stormlightlabs/thunderstorm/issues/25
[#26]: https://github.com/stormlightlabs/thunderstorm/issues/26
[#27]: https://github.com/stormlightlabs/thunderstorm/issues/27
[#28]: https://github.com/stormlightlabs/thunderstorm/issues/28
[#34]: https://github.com/stormlightlabs/thunderstorm/issues/34
[#35]: https://github.com/stormlightlabs/thunderstorm/issues/35
[#36]: https://github.com/stormlightlabs/thunderstorm/issues/36
