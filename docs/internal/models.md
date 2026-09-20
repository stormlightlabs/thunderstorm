---
name: models
last_updated: 2026-09-19
id: 01M2PXCQDS6JFP04C2JK1T6XMD
---

> Copied from `stormlightlabs/thunderus` on 2026-09-19 with the rest of the
> workflow. The identifier is unchanged so that open issues citing it still
> resolve. The role assignments here are thunderus's.

# Model assignments

Which model runs which thunderstorm role. A review comment names the model and
reasoning level it came from in its first line, so the record shows who found
what.

## Roles

| Role                 | Claude Code  | Codex and Pi                                                           | OpenCode Go | Cursor |
| -------------------- | ------------ | ---------------------------------------------------------------------- | ----------- | ------ |
| Implementer          | Opus, Sonnet | `5.6-sol` at low or medium, `5.6-luna` at high or above                | todo        | todo   |
| Reviewer             | Opus, Fable  | `5.6-terra` at high or above, `5.6-sol` at medium, `6-astra` at medium | todo        | todo   |
| Adversarial reviewer | Opus, Fable  | `5.6-sol` at high, `6-astra` at medium                                 | todo        | todo   |

## What a harness must provide

Four things, before the table above can name a harness at all:

- A model the dispatch chooses, rather than the one the session is already
  running.
- A reasoning level the dispatch chooses and the role can name.
- A second context within one run, so the implementer and the reviewer are
  different sessions.
- Evidence, after the run, of which model and level each pass used.

The fourth is separate from the third because asking for a different model is
not the same as getting one. On Codex a full-history fork — `fork_turns`
omitted or `"all"` — inherits the parent's model and reasoning effort and
refuses to override either, so a dispatch that forgets `fork_turns` runs the
reviewer on the implementer's model and says nothing about it. Separation there
is a property of the dispatch rather than of the harness, which is why the list
asks for evidence and not only for the capability.

| Harness     | Model                                                      | Level                                            | Second context                       | Evidence                                         |
| ----------- | ---------------------------------------------------------- | ------------------------------------------------ | ------------------------------------ | ------------------------------------------------ |
| Claude Code | the subagent definition                                    | the subagent definition                          | subagents                            | the review comment's first line                  |
| Codex       | `spawn_agent`, with `fork_turns` of `"none"` or an integer | `reasoning_effort` on the same call              | `spawn_agent`, three workers at once | the first line, if the dispatch forked correctly |
| Pi          | `--model` on the pane's command                            | `--thinking`, read back as `$PI_REASONING_LEVEL` | one session per tmux pane            | the pane's command and its `events.jsonl`        |
| OpenCode Go | not assessed                                               | not assessed                                     | not assessed                         | not assessed                                     |
| Cursor      | not assessed                                               | not assessed                                     | not assessed                         | not assessed                                     |

The mechanisms are in [hosts.md](hosts.md), verified against `codex-cli`
0.146.0 and `pi` 0.85.1 on 2026-09-19.

OpenCode Go and Cursor are planned but unsupported, and neither has been
assessed against the list above: no Cursor agent is installed on the machine
the host contracts were verified on, and OpenCode Go was not examined there.
stormlightlabs/thunderstorm#2 carries the decision for both. Until one of them
is assessed and the roles table names its models, do not run a thunderstorm
role on it.

## Picking within a row

Take the cheaper option first. Move up when the work has one of these
properties, not because the change feels important:

- The failure is ambiguous and the cause is not yet located.
- The change crosses a module boundary or alters released behavior.
- The issue carries `risk:high`.
- A cheaper model already ran and its output did not survive review.

## Rules

The implementer and the reviewer never share a run. A model reviewing its own
diff inherits the gap that produced the defect.

The two standard review passes should not both use the same model and level
when another assignment in the row is available. Two passes from one
configuration produce close to one pass of coverage.

The adversarial pass assumes the standard passes ran. Give it a configuration
tuned for finding what they missed rather than repeating them.

## Where the record lives

A review comment's opening line names the model and reasoning level it ran at,
under the `review` skill's **Post the findings**; an edit reply names both the
pass it answers and its own. That line is the only record, and it is what makes
the rules above checkable after the fact. Commits carry no signature.
