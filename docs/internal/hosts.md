# Host contracts

What Claude Code, Pi, and Codex actually load, verified on this machine against
Claude Code, `pi` 0.85.1, and `codex-cli` 0.146.0. Cursor is not covered; no
Cursor agent is installed here, so nothing below is claimed about it.

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

| Directory              | Claude Code | Pi  | Codex |
| ---------------------- | ----------- | --- | ----- |
| `.claude/skills/`      | yes         | no  | no    |
| `.pi/skills/`          | no          | yes | no    |
| `.codex/skills/`       | no          | no  | yes   |
| `.agents/skills/`      | no          | yes | yes   |
| `~/.claude/skills/`    | yes         | no  | no    |
| `~/.agents/skills/`    | no          | yes | yes   |

`.agents/skills/` covers Pi and Codex together. Claude Code reads neither the
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

| Resource  | Claude Code             | Pi                   | Codex                   |
| --------- | ----------------------- | -------------------- | ----------------------- |
| Commands  | `.claude/commands/*.md` | `.pi/prompts/*.md`   | `~/.codex/prompts/*.md` |
| Subagents | `.claude/agents/*.md`   | none; tmux instead   | built in, on by default |
| Hooks     | `.claude/settings.json` | extensions (TS/JS)   | `hooks.json`            |
| Themes    | none                    | `.pi/themes/*.json`  | `config.toml`           |

Pi's project resource roots are `.pi/{extensions,skills,prompts,themes}` and its
user roots are `~/.pi/agent/{extensions,skills,prompts,themes}`. Codex prompts
live at `~/.codex/prompts`; that path is from Codex's documentation and was not
exercised here, because the directory does not exist on this machine.

## Dispatch

Three harnesses, three mechanisms, and the review fan-out has to be expressed in
all of them.

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

A skill becomes a dispatchable agent by carrying `agents/openai.yaml` beside its
`SKILL.md`, which names the display metadata and a
`policy.allow_implicit_invocation` flag; Codex's own `review-agent` is built
this way and is invoked as `$review-agent`.

**Pi has no subagents and does not want them.** Its route is a session per tmux
pane, and that route has been run: a reviewer dispatch completed here on
2026-09-19, exit 0, against a worktree of its own. The shape was

```sh
pi --mode json --print --approve --session-dir <dir>/sessions \
   --model openai-codex/gpt-5.6-terra --thinking high \
   --tools read,bash,grep,find,ls,mcp \
   --append-system-prompt <dir>/system.md -- '<task>'
```

with the pane signalling completion through `tmux wait-for`, stdout captured as
`events.jsonl`, and the exit status written to a file. The role prompt names the
model through `$PI_MODEL` and `$PI_REASONING_LEVEL`, so a finding still reports
where it came from. Per-role tool limits are `--tools` and `--exclude-tools`
rather than a subagent's allowlist.

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
`Hooks (0)` while the payload carried its hook registration there. Permissions
have no plugin mechanism at all, so the deny rules that stop a session merging
its own work are a file a repository merges into its own settings.

## Plugin manifests

Three manifest formats, one shape.

| Host        | Manifest path                    | Marketplace                            |
| ----------- | -------------------------------- | -------------------------------------- |
| Claude Code | `.claude-plugin/plugin.json`     | `.claude-plugin/marketplace.json`      |
| Codex       | `.codex-plugin/plugin.json`      | `.agents/plugins/marketplace.json`     |
| Pi          | `package.json`, under a `pi` key | npm, or a git URL                      |

Codex's manifest carries `name`, `version`, `description`, `author`, and an
`interface` block of presentation metadata; `version` must be strict semver, and
the validator rejects unknown fields. Pi's is an npm package whose `pi` key
names its extensions and skill directories:

```json
{ "pi": { "extensions": ["./index.ts"], "skills": ["./skills"] } }
```

There is also a fourth, vendor-neutral format. Pi validates a `plugin.json`
carrying `$schema` of `https://agent-plugins.org/schemas/1.0.0/plugin.schema.json`
and Codex puts its marketplace under `.agents/plugins/`, the same namespace as
`.agents/skills`. The standard is young and neither host requires it, so it is
worth tracking rather than building on.

## Installation

All three install from a git repository, so one repository can serve all three:

```sh
# Claude Code
/plugin marketplace add stormlightlabs/thunderstorm

# Codex
codex plugin marketplace add stormlightlabs/thunderstorm
codex plugin add thunderstorm@stormlightlabs

# Pi
pi install git:github.com/stormlightlabs/thunderstorm
```

Codex's `marketplace add` takes `owner/repo[@ref]`, an HTTPS URL, an SSH URL, or
a local path, and `--sparse` restricts the checkout to named paths. Pi installs
from `npm:`, `git:`, an HTTPS or SSH URL, or a local path, and `-l` installs
into the project's own `.pi/settings.json` rather than the user's.
