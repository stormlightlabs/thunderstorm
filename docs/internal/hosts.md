---
name: hosts
last_updated: 2026-09-20
id: 01M2YGPF6QM3YGDKFDVGP3BR7W
---

# Host contracts

What Claude Code, Pi, and Codex actually load, verified on this machine against
Claude Code, `pi` 0.85.1, and `codex-cli` 0.146.0. Cursor has its own section
at the end, taken from its documentation on 2026-09-19: no Cursor agent is
installed here, so none of it was exercised.

Discovery was checked by building a fixture with a probe skill in each candidate
directory and asking each agent to name the skills it could see. Dispatch and
hooks come from Codex's own prompt, its feature list, and the strings in its
binary, which say what the tools are but not how well they work. Pi's dispatch
shape is read off a run that completed. Anything taken from documentation alone
is marked where it appears.

## Skill discovery

A skill is a directory holding `SKILL.md` with `name` and `description`
frontmatter. All three hosts agree on that shape, so the file itself needs no
translation. They disagree only on where they look.

| Directory              | Claude Code | Pi  | Codex | Cursor |
| ---------------------- | ----------- | --- | ----- | ------ |
| `.claude/skills/`      | yes         | no  | no    | no     |
| `.pi/skills/`          | no          | yes | no    | no     |
| `.codex/skills/`       | no          | no  | yes   | no     |
| `.cursor/skills/`      | no          | no  | no    | yes    |
| `.agents/skills/`      | no          | yes | yes   | yes    |
| `~/.claude/skills/`    | yes         | no  | no    | no     |
| `~/.agents/skills/`    | no          | yes | yes   | yes    |

`.agents/skills/` covers Pi, Codex and Cursor together. Claude Code reads neither the
project nor the user copy of it, so two locations cover all three hosts rather
than four. That is what the `.agents/skills -> ../.claude/skills` symlink in
thunderus was working around.

Pi walks from the working directory up to the git root collecting
`.agents/skills` at every level, and requires the project to be trusted before
it reads any of them. It also distinguishes the two roots: under `.pi/skills` a
bare `*.md` file is a skill, while under `.agents/skills` a skill must be a
subdirectory containing `SKILL.md`. Write the subdirectory form and both roots
accept it.

## Other resources

Skills are the portable part. The rest varies, but less than it first appears.

| Resource  | Claude Code             | Pi                   | Codex                         |
| --------- | ----------------------- | -------------------- | ----------------------------- |
| Commands  | `.claude/commands/*.md` | `.pi/prompts/*.md`, or a package's `prompts/` | plugin-qualified skills |
| Subagents | `.claude/agents/*.md`   | none; tmux instead   | built-in agents with role text |
| Hooks     | `.claude/settings.json` | extensions (TS/JS)   | plugin `hooks.json`           |
| Themes    | none                    | `.pi/themes/*.json`  | `config.toml`                 |

Pi's project resource roots are `.pi/{extensions,skills,prompts,themes}` and its
user roots are `~/.pi/agent/{extensions,skills,prompts,themes}`. Codex prompts
live at `~/.codex/prompts`; that path is from Codex's documentation and was not
exercised here, because the directory does not exist on this machine.

A pi package ships prompts. Its manifest takes `prompts` beside `extensions`,
`skills` and `themes`, and pi also loads a package's bare `prompts/`
directory; both are in pi's own bundled documentation at
`docs/packages.md` and `docs/prompt-templates.md`, read on 2026-09-19. The
template format is the one Claude Code already uses: `description` and
`argument-hint` in the frontmatter, the filename as the command name, and
`$ARGUMENTS` in the body, alongside `$1` and `${1:-default}`. So a command
body crosses to Pi untranslated.

Codex custom prompts are deprecated and live only under `~/.codex/prompts`.
Plugins expose their skills instead: `$thunderstorm:thunderstorm` invokes the
full loop. The rendered `prompts/` directory remains a compatibility artifact;
the marketplace install does not depend on it.

## Dispatch

Each harness expresses review dispatch differently.

**Codex has a real multi-agent system**, stable and enabled by default: the
`multi_agent` feature is on, and the model is told it is `/root` in a team. The
tools are `spawn_agent`, `followup_task`, `send_message`, `wait_agent`,
`interrupt_agent`, and `list_agents`, and a sub-agent may spawn its own. Two
constraints shape a run:

- **Four concurrency slots**, the orchestrator included, so three workers at
  once. `max_concurrent_threads_per_session` and `max_depth` configure it.
- **Every agent shares one working directory** and one filesystem. Codex says so
  outright, which makes the stolen-`HEAD` failure certain rather than likely:
  the orchestrator must create each worktree and tell the agent to work there.

`fork_turns` decides how much context a child inherits, and it gates model
selection. A full-history fork — `fork_turns` omitted or `"all"` — inherits the
parent's model and reasoning effort **and refuses to override either**. Setting
`model` or `reasoning_effort` requires `fork_turns` of `"none"` or a positive
integer string. The loop's rule that an implementer and a reviewer never share a
model is therefore not merely a convention on Codex; a dispatch that forgets
`fork_turns` silently violates it. `default_subagent_model` and
`default_subagent_reasoning_effort` in `config.toml` set the fallback.

Codex can load custom agents from `.codex/agents/*.toml` in a trusted project
or `~/.codex/agents/*.toml` for one person. Thunderstorm does not install its
roles there because those agents would remain active when the plugin was off.
The renderer packages the roles as TOML instead. The orchestrator reads each
role and passes its developer instructions to a built-in agent. Roles without
`Write` or `Edit` specify a read-only sandbox.

Pi has no built-in subagent mechanism. Thunderstorm starts one session per tmux
pane through `tstorm dispatch`. A reviewer dispatch completed here on
2026-09-19, exit 0, in a directory of its own, answering from a file it found
there. The command the pane runs is

```sh
pi --mode json --print --approve --session-dir <out>/sessions \
   --model openai-codex/gpt-5.6-sol --thinking low \
   --tools bash,find,grep,ls,read \
   --append-system-prompt <out>/system.md -- '<task>'
```

with stdout redirected to `<out>/events.jsonl` and the shell's exit status
written to `<out>/status` before the pane ends. Completion is `tmux has-session`
asked every 200ms until it fails. `tmux wait-for` is woken only by a pane that
reached the end of its script, so a pane killed from outside would hold the run
open until the timeout. Each dispatch gets a tmux server of its own, because a
shared server hands every later session the environment of whichever client
started the server.

Per-role tool limits are `--tools` and `--exclude-tools` rather than a
subagent's allowlist, and `internal/dispatch/tools.go` holds the translation
from the names a definition uses: `Read` to `read`, `Write` to `write`, `Edit`
to `edit`, `Grep` to `grep`, `Glob` to `find` and `ls`, `Bash` to `bash`. Three
names have no counterpart, and a dispatch prints them rather than dropping
them. `Skill` is not a tool on pi, which discovers skills itself. `WebFetch`
has no built-in equivalent. The GitHub MCP tools are the cloud transport, and a
pane runs on the machine the operator is sitting at, where the transport is
`gh` through bash.

Narrowing the list is not what keeps a reviewer from editing the diff: it
loses `write` and `edit` and keeps `bash`. Pi's own `docs/security.md` says it
ships no sandbox and that isolation has to come from the operating system or a
container, so what separates one role's work from another's is the directory
its pane starts in. Sandboxing the process is
`internal/ideas/remote-operation.md` under bwrap.

The appended system prompt names the role's model and reasoning level, so a
finding still reports where it came from, and `events.jsonl` records the
provider and model that answered. `$PI_MODEL` and `$PI_REASONING_LEVEL` reach
only the commands pi's bash tool runs, not the session's own environment.

Codex can drive tmux the same way, since it has a shell and background
terminals, but it has no reason to: `spawn_agent` is the better route there.

## Hooks

Codex's hook contract is close enough to Claude Code's to share one
implementation. The events are `PreToolUse`, `PostToolUse`, `UserPromptSubmit`,
`Stop`, `SessionStart`, `PreCompact`, `PostCompact`, `PermissionRequest`,
`SubagentStart`, and `SubagentStop`. Inputs arrive as `session_id`, `turn_id`,
`cwd`, `tool_name`, `tool_input`, `tool_response`, `hook_event_name`, `model`,
`permission_mode`, and `agent_type`; a hook replies with `hookSpecificOutput`
carrying `hookEventName`, `additionalContext`, `permissionDecision`, and
`permissionDecisionReason`. Those are Claude Code's names.

So the deslop gate is one script behind two thin adapters, not three
implementations. Pi is the exception: its only equivalent is an extension in
TypeScript, and that is the shim the packaging has to carry.

A Claude Code plugin does carry hooks, verified by installing this
repository's payload on 2026-09-19: `claude plugin details thunderstorm`
reported `Hooks (1) SessionStart`. They travel in `hooks/hooks.json` at the
payload root, with commands written against `${CLAUDE_PLUGIN_ROOT}`, not in a
`hooks` field of the plugin manifest, which the `plugin-creator` scaffold
rejects.

A `settings.json` in a payload is read by nothing. The same install reported
`Hooks (0)` while the payload carried its hook registration there.

## Permissions

Only a human merges, and each host enforces that rule differently. Claude Code
needs a repository setting. Codex and Pi load their command gates from the
installed package.

| Host        | What stops a command                    | Where it lives           |
| ----------- | --------------------------------------- | ------------------------ |
| Claude Code | a deny rule per command prefix          | `.claude/settings.json`  |
| Codex       | a plugin `PreToolUse` hook               | the installed plugin    |
| Pi          | a package extension blocks each prefix  | the installed package    |
| Cursor      | unverified                              | —                        |

The workflow manifest names four commands: `gh pr merge`, `gh pr review`,
`git push` and `git merge`. Claude Code is the only host that takes them as
written, one `Bash(<prefix>:*)` rule each. The renderer translates, so the
manifest carries no host's spelling.

The Codex plugin registers a synchronous `PreToolUse` hook for `Bash`. The hook
reads `tool_input.command` and returns `permissionDecision = "deny"` for a
matching prefix. Codex asks the user to trust a plugin hook before it runs.

The payload also includes Starlark execpolicy rules for users who want the
same commands blocked while the plugin is disabled. Codex reads `rules/` in
every active config layer: `~/.codex/rules/` for one person and
`<repo>/.codex/rules/` for a trusted project.

`codex execpolicy check --rules <file> -- <command>` reports what a command
gets, and is how the rendered file was checked against `codex-cli` 0.146.0 on
2026-09-19: the four commands came back `forbidden`, while `git status`,
`gh pr view` and `gh pr comment` matched nothing.

The approval policy is not the route. `approval_policy = "untrusted"` is
retired, and Codex's configuration reference says to remove it; what survives
is `on-request`, `never`, or a table of booleans, which choose when a prompt
appears rather than which commands are refused. Sandboxing is a third axis:
`workspace-write` turns network access off by default, which would stop
`gh pr merge` and `git push` but leave a local `git merge` alone.

Pi ships no sandbox and leaves isolation to the operating system, a container,
or a micro-VM. The thunderstorm package handles its four denied command
prefixes: its extension intercepts Pi's `bash` tool and rejects a matching
command before the tool runs. Per-role `--tools` still narrows what a session
can call, and the worktree its pane starts in separates one role's files from
another's. Broader process isolation remains the bwrap work in
`internal/ideas/remote-operation.md`.

## Plugin manifests

Each harness has its own package entry point.

| Host        | Manifest path                    | Marketplace                            |
| ----------- | -------------------------------- | -------------------------------------- |
| Claude Code | `.claude-plugin/plugin.json`     | `.claude-plugin/marketplace.json`      |
| Codex       | `plugin.json`                    | `.agents/plugins/marketplace.json`     |
| Pi          | `package.json`, under a `pi` key | npm, or a git URL                      |

New Codex packages use the Agent Plugins manifest at the package root. Skills
under `skills/` need no manifest field. The payload also keeps
`.codex-plugin/plugin.json` as a compatibility manifest for clients that still
read it. Pi uses an npm package whose `pi` key names its extensions, skills,
and prompts:

```json
{ "pi": { "extensions": ["./extensions"], "skills": ["./skills"], "prompts": ["./prompts"] } }
```

The root Codex manifest uses the vendor-neutral Agent Plugins schema. Codex
puts its marketplace under `.agents/plugins/`, beside the shared skills
namespace.

## Installation

All three use artifacts from this repository:

```sh
# Claude Code
/plugin marketplace add stormlightlabs/thunderstorm

# Codex
codex plugin marketplace add stormlightlabs/thunderstorm
codex plugin add thunderstorm@stormlightlabs

# Pi
pi install git:github.com/stormlightlabs/thunderstorm
```

Codex reads `.agents/plugins/marketplace.json` from this repository. A trusted
project can also declare the Git marketplace and enable the plugin in its own
`.codex/config.toml`, without adding the marketplace to the user's config. Pi
installs from `npm:`, `git:`, an HTTPS or SSH URL, or a local path, and `-l`
installs into the project's own `.pi/settings.json` rather than the user's.

## Cursor

From Cursor's documentation on 2026-09-19, not from a run: no Cursor agent is
installed here, and every line below needs checking against one before the
renderer relies on it.

Cursor loads skills from `.agents/skills/` as well as `.cursor/skills/`, and
from `~/.agents/skills/` and `~/.cursor/skills/` for a user-wide copy. The
same `SKILL.md` shape applies, with `name` matching the folder and
`description` saying when to use it, and a skill directory may carry
`scripts/`, `references/` and `assets/`. Nested copies anywhere in a
repository are picked up and scoped to that subtree.

Two consequences, if it holds. The `.agents/skills/` directory the Codex and
Pi payloads already need serves Cursor too, so Cursor skills cost a target in
the renderer rather than a third copy of the skills. And commands are the
thin part: `.cursor/commands/*.md` is the older form, retired in favour of
skills, which is the same gap as #7 on the other two harnesses.

Cursor has subagents, added in its 2.4 release alongside skills, each running
in its own context. Whether it can carry a thunderstorm role also needs a
model the dispatch chooses and a reasoning level the role can name, which is
what `models.md` asks of every harness and what #2 decides for this one.
