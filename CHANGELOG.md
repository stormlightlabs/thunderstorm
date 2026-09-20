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
  from one canonical source. An artifact that needs a capability the target
  does not provide stops the render, which writes nothing and names every gap
  with the issue that would close it ([#17]).
- `tstorm render --check` reports whether the payload on disk still matches the
  source, and writes nothing.
- The workflow itself, under `workflow/`: twelve skills, nine commands, four
  agent definitions and five check scripts, with a manifest saying what each
  artifact is and which harness capabilities it needs ([#17]).
- A Claude Code payload under `payloads/claude`, and a marketplace at
  `.claude-plugin/marketplace.json` that installs it. `claude plugin details
  thunderstorm` reports twelve skills, sixteen commands and four agents
  ([#1]).
- `/forecast` as a second name for `/triage`, which describes what the stage
  produces rather than how a hospital sorts casualties.
- `docs/internal/models.md` now states what a harness must provide before it
  can carry a role: a model the dispatch chooses, a reasoning level it chooses
  and the role can name, a second context within one run, and evidence
  afterwards of which model each pass used ([#10]).
- `tstorm dispatch --role <role> --worktree <dir> --model <id>` runs one role
  as a pi session in a tmux pane and prints its report. The transcript
  directory holds the command, the event stream and the exit status ([#8]).

### Changed

- The skills run the gates a repository's `AGENTS.md` names, rather than the
  Cargo commands and crate names they carried out of the repository they were
  written in ([#1]).
- Payload prose distinguishes the harness directory inside the repository being
  worked on from the directory an installed payload landed in. They were one
  path until an install put the payload outside the repository entirely
  ([#1]).
- Hook registrations render to `hooks/hooks.json`, which a plugin install
  reads. A `settings.json` inside a payload is read by nothing.
- The deny rules that stop a session merging or approving its own work ship as
  a `settings.json` for a repository to merge into its own, because no plugin
  mechanism carries a permission ([#1]).
- The Go module path is `github.com/stormlightlabs/thunderstorm`, matching the
  repository it is fetched from. It named a `tstorm` repository that does not
  exist, so `go install` could not resolve it. The binary is still `tstorm`:
  `go install github.com/stormlightlabs/thunderstorm/cmd/tstorm@latest`.
- Codex and Pi are recorded as unsupported with the reason, rather than left
  looking like options nobody thought about. Neither has a place for a command
  ([#7]), and Pi has no subagents ([#8]).
- `docs/internal/thunderstorm.md` says what starts a review pass on each
  harness, and `docs/internal/hosts.md` says what a role may reach on Pi
  ([#8]).

### Removed

- The `SessionStart` hook that warmed a Cargo registry. It belongs to the
  repository it came from, so the payload now ships no hook at all ([#1]).

[Unreleased]: https://github.com/stormlightlabs/thunderstorm/commits/main
[#1]: https://github.com/stormlightlabs/thunderstorm/issues/1
[#7]: https://github.com/stormlightlabs/thunderstorm/issues/7
[#8]: https://github.com/stormlightlabs/thunderstorm/issues/8
[#10]: https://github.com/stormlightlabs/thunderstorm/issues/10
[#17]: https://github.com/stormlightlabs/thunderstorm/issues/17
