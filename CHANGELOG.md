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
- The workflow itself, under `workflow/`: twelve skills, nine commands, four
  agent definitions and five check scripts, with a manifest saying what each
  artifact needs of a harness ([#17]).
- A Claude Code payload under `payloads/claude`, and a marketplace at
  `.claude-plugin/marketplace.json` that installs it ([#1]).
- Codex and Pi payloads. Codex gets a marketplace plugin with packaged roles
  and a command-policy hook; Pi gets an installable package with prompts,
  skills, scripts, and role dispatch through `tstorm`.
- `/forecast` as a second name for `/triage`, which describes what the stage
  produces rather than how a hospital sorts casualties.
- `tstorm dispatch --role <role> --worktree <dir> --model <id>` runs one role
  as a Pi session in a tmux window or Zellij tab and prints its report. Provider,
  model, reasoning level, and multiplexer are configurable. The transcript
  directory holds the command, event stream, and exit status ([#8]).
- `docs/internal/models.md` states what a harness must provide before it can
  carry a role ([#10]).

### Changed

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

[Unreleased]: https://github.com/stormlightlabs/thunderstorm/commits/main
[#1]: https://github.com/stormlightlabs/thunderstorm/issues/1
[#7]: https://github.com/stormlightlabs/thunderstorm/issues/7
[#8]: https://github.com/stormlightlabs/thunderstorm/issues/8
[#9]: https://github.com/stormlightlabs/thunderstorm/issues/9
[#10]: https://github.com/stormlightlabs/thunderstorm/issues/10
[#17]: https://github.com/stormlightlabs/thunderstorm/issues/17
