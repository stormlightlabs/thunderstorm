---
name: github-board
description: Read and write thunderstorm board state on GitHub issues and the project board, through gh locally or the GitHub MCP tools in a cloud session. Use when claiming an issue, changing its status, filing a sub-issue, recording a block or a dependency, or reading what is queued.
---

# GitHub board

The only skill permitted to write board state. Every status change goes through
the operations here so one mechanism owns the transitions.

## Transport

Two transports reach the same board. Pick by what the session has:

| Session         | Transport        | How to tell                         |
| --------------- | ---------------- | ----------------------------------- |
| Cloud (web) run | GitHub MCP tools | `CLAUDE_CODE_REMOTE=true`           |
| Local checkout  | `gh`             | `gh auth status` succeeds otherwise |

`CLAUDE_CODE_REMOTE` decides first and on its own. A cloud image that happens to
carry `gh` almost certainly carries no token with it, and a run that picks `gh`
on the strength of the binary alone fails on its first write, or worse, decides
it is local and tries to create a worktree. Where the variable is unset, `gh`
needs a working credential, not merely a place on `PATH`.

Check once at the start of a run and use that transport throughout. Never
report a board change made through one transport as if it came from the other.

Two differences decide correctness, so read them before the first write:

- `gh issue edit` applies a **delta**: `--remove-label` and `--add-label` change
  only the labels named. The MCP `issue_write` update applies a **replacement**:
  the `labels` array becomes the issue's entire label set, and so does
  `assignees`. Read the current values first and send the whole intended set,
  or whatever you left out is dropped and nothing says so.
- **Status is neither a label nor an issue field.** It is a single-select on
  the project board, and neither transport in the table above reaches it.
  `gh project` does, given the `project` token scope; a cloud session goes to
  the GraphQL API the way [Dependencies](#dependencies) does. See
  [Status](#status).

## Read

| Operation  | `gh`                                                              | MCP                                  |
| ---------- | ----------------------------------------------------------------- | ------------------------------------ |
| One issue  | `gh issue view <n> --json number,title,body,assignees,state,url`  | `issue_read` method `get`            |
| Sub-issues | `gh issue view <n> --json subIssues`                              | `issue_read` method `get_sub_issues` |

What is queued, and what any one issue's status is, come off the board rather
than off the issue. [Status](#status) carries those reads.

Two shapes of issue. One with sub-issues is what a run takes, at most five of
them open at once; a sub-issue is the unit of work. Grouping above that is a
milestone, which is not an issue.

An issue with sub-issues is never claimed and nothing here writes its status,
because its state is whatever its children say. It outlives the runs that work
it. The `decompose` skill's **Three shapes** carries the rest.

Set a milestone with `gh issue edit <n> --milestone <name>`, read one with
`gh issue list --milestone <name>`, and create or edit one through
`gh api repos/{owner}/{repo}/milestones`; there is no `gh milestone` command.

## Status

Status is a single-select field on the **THNDRS** project board, number 13
under the `stormlightlabs` owner, which carries this repository's issues
alongside others. It is not a label and not a field on the issue. Three
options are the whole vocabulary:

| Option        | What it means                                             |
| ------------- | --------------------------------------------------------- |
| `Todo`        | Nobody holds it.                                            |
| `In Progress` | A run holds it, whether or not a pull request is open yet. |
| `Done`        | The merged change is confirmed on `main`.                   |

Read the board projecting only the fields you need. The raw item list repeats
every milestone's full description on every row and is large enough to matter:

```sh
gh project item-list 13 --owner stormlightlabs --format json --limit 100 \
  | jq -r '.items[]
           | select(.content.repository == "<owner>/<repo>")
           | "\(.content.number)\t\(.status)\t\(.content.title)"'
```

Write one field at a time, naming the issue by URL:

```sh
gh project item-edit 13 --owner stormlightlabs \
  --url https://github.com/<owner>/<repo>/issues/<n> \
  --field Status --value "In Progress"
```

| From          | To            | When                                           |
| ------------- | ------------- | ---------------------------------------------- |
| `Todo`        | `In Progress` | A run takes the issue and starts work on it.   |
| `In Progress` | `Todo`        | The run abandons it or is blocked. Unassign.   |
| `In Progress` | `Done`        | The merged change is confirmed on `main`.      |

A pull request opening is not a transition. The issue is already `In Progress`
and the board's **Linked pull requests** column is what says a review is
waiting, so report the pull request and change nothing. Nor is the merge
itself: a person merges in GitHub, and no skill, script or command here does.

Dropping an issue means closing it with a comment giving the reason. The
project's built-in workflow sets `Done` on close; read the item back and set it
by hand if it did not.

`Todo` and a missing value are the same thing to every reader here. An item
added to the board with no status is queued.

### Neither transport reaches it

The table under [Transport](#transport) is for issues. Board fields are
Projects V2, which the GitHub MCP server does not expose, so a cloud session
goes to the GraphQL API the way [Dependencies](#dependencies) does. Locally,
`gh project` needs the `project` scope on the token: `gh auth status` lists the
scopes, and `gh auth refresh -s project` adds it.

## Claim

An issue is claimable only when nobody holds it: no assignee, whoever would
have put them there, and `Todo` on the board. Read both before taking it.

A claim is a status change followed by an assignment, in that order:

```sh
gh project item-edit 13 --owner stormlightlabs \
  --url https://github.com/<owner>/<repo>/issues/<n> \
  --field Status --value "In Progress"
gh issue edit <n> --add-assignee @me
```

The order is what makes an interrupted claim safe. A session killed between the
two leaves `In Progress` with no assignee, which the 24-hour rule returns to
the queue. Assigning first would leave an owned issue still reading `Todo`,
which the next claimant takes as free.

Through MCP, `@me` has no equivalent: call `get_me` for the login and send it
in the `assignees` array of an `issue_write` update. That write replaces rather
than adds, so it sends exactly `[me]`.

Re-read after claiming. The claim succeeded only when `assignees` is exactly
your login and nothing else. Two `gh` runs that claim at once both succeed at
`--add-assignee`, which adds rather than replaces, so each finds itself present
and each believes it won; comparing against the whole list is what tells them
apart.

Losing the race means removing your own assignment and nothing else. Do not
touch the status: `In Progress` belongs to the run that won it, and putting the
issue back to `Todo` hands work in progress to a third run. If both runs back
off, the issue is left `In Progress` with no assignee, which the 24-hour rule
already covers.

## Block

The board carries no blocked option, so a block is recorded as a release
rather than a state: say what is needed, give the issue back, stop.

```sh
gh issue comment <n> --body "<what is needed to unblock, and from whom>"
gh issue edit <n> --remove-assignee @me
gh project item-edit 13 --owner stormlightlabs \
  --url https://github.com/<owner>/<repo>/issues/<n> \
  --field Status --value "Todo"
```

Through MCP: `add_issue_comment`, then an `issue_write` update sending an
empty `assignees` array, then the board write above.

Comment first, and name what would unblock it. An issue back in `Todo` with no
assignee and no comment cannot be told from one nobody has started, so the next
run takes it and walks into the same wall.

## File new work

Work found mid-run goes in a new issue, never into the one being worked:

```sh
gh issue create --title <title> --body-file <file>
gh project item-add 13 --owner stormlightlabs --url <the new issue's URL>
gh project item-edit 13 --owner stormlightlabs --url <the new issue's URL> \
  --field Status --value "Todo"
```

An issue that never reaches the board is invisible to every read here, so add
it in the same breath as filing it.

Through MCP: `issue_write` method `create`. Attach it to a parent in the same
call with `parent_issue_number`, or afterwards with `sub_issue_write` method
`add`, which takes the sub-issue's ID rather than its number.

Link it from the originating issue with a comment. Do not start it in this run.

## Dependencies

A sub-issue says what an issue is part of. A dependency says what it has to wait
for, and the two are different relations: #31 is a sub-issue of #26 and blocked
by #28 and #29 at the same time. Recording the second one is what stops a run
dispatching a harness before the thing it starts exists, and GitHub enforces it
by refusing to close an issue whose blockers are open.

A dependency is an ordering known when the issues are filed. It is not a
block, which stops a run (see the `thunderstorm` skill's stop conditions) and
belongs to something discovered while working. An issue waiting on a sibling
stays `Todo` and keeps its place in the dispatch order.

### Neither transport performs them

The GitHub MCP server exposes no dependency tool: `sub_issue_write` writes
hierarchy and nothing writes `blocked_by`, and a cloud image carries no `gh`
binary to fall back to. So this is the one board operation that goes to the
REST API directly, and it is the only place this skill reaches past the
transport table. Every other board write still goes through `gh` or MCP.

`references/dependencies.md` holds the read, write, and verify calls, and the
token rule that goes with them. `triage`'s `references/reading-the-board.md`
holds the read path for the counts the issue list carries and neither transport
exposes.

## Labels

Labels carry no board state. Status lives on the project, and the repository's
own labels are whatever `gh label list` reports, which today is GitHub's
default set and nothing this skill reads or writes.

A run that wants a label the repository does not have is not licensed to
create one. Say which label is missing and stop. A label invented mid-run
splits the board in two, and the half nobody knows about is invisible to every
query written against the other.

## Rules

- One writer at a time. Two runs editing one issue produce a board nobody trusts.
- Do not close an issue to express any state other than dropping it. `Done` is
  set once the merged change is confirmed on `main`.
- Do not edit an issue body written by a human. Add a comment instead.
- Do not invent labels. The repository's own labels are the set.
- Keep the issue and the board agreeing. An assignee with `Todo`, or `In
  Progress` with nobody on it, is a claim that half happened.
- Report what changed. A status transition nobody announced is a transition
  nobody can question.
