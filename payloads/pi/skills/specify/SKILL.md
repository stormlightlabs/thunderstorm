---
name: specify
description: Turn an idea into a design document a decomposition can read. Use for /spec-ify, /specify, or when an idea cannot yet produce issues because something has to be decided first.
---

# Specify

One design, written down, so that work can be cut from it. A spec exists to
make a decision that issues would otherwise each have to make separately and
inconsistently.

## Write one only when it earns itself

Ask one question of the idea: **can you write a sub-issue's stop rule without
deciding anything else first?**

If yes, there is nothing for a spec to add. Go to `decompose` and file the
work. Most chores, bug fixes, and anything already described by a finding are
in this case.

If no, the missing decision is what the spec is for. "Give tool activities
distinct semantic kinds" cannot produce a checkable stop rule until someone
settles what the kinds are, and five issues that each invent their own answer
is worse than one document that picks.

A spec that records no decision is a summary of the idea with more words.

## Where it goes

Under the directory `documents` names in `.tstorm.toml`:

```text
<documents>/features/<feature-name>/plan.md
```

A repository that names none creates `docs/internal/` and sets
`documents = "docs/internal"`. That one setting is what the skills write into
and what `tstorm check frontmatter` walks.

One directory per feature track, named for the feature rather than for the
change. Every document under that tree carries the frontmatter the check
requires:

```yaml
---
name: <path under the tree, separators as dashes>
last_updated: <YYYY-MM-DD>
id: <ULID>
---
```

The name is the document's path with the extension dropped and the slashes
turned to dashes, so `features/mcp/plan.md` is named `features-mcp-plan` and
two plans under two feature directories have two names. A `README.md` takes
the name of the directory holding it. Where two paths do fold to one name, the
check names both and fails; rename one.

Generate the identifier with `tstorm ulid`. The idea the spec came
from cites its own identifier, and issues cut from the spec cite the spec's, so
the trail reads from either end.

## What a spec holds

- **The decision.** What was chosen, and what it rules out. This is the reason
  the document exists.
- **The shape.** How the pieces fit: boundaries, ownership, the vocabulary the
  issues will use. Enough that two people cutting issues from it cut the same
  ones.
- **What it does not cover.** Scope the work must not absorb, so a worker who
  finds adjacent work knows it is adjacent.

## What a spec does not hold

**No task list.** The board holds the work. A checklist inside a spec is a
second copy of the board's state that nothing updates and nobody reviews, and
it goes stale the first time an issue closes. `decompose` reads the spec and
files issues; that is what replaces the checklist.

**No implementation.** Code in a spec is a sketch nobody compiles. Name the
types and the boundaries if they carry the decision; stop there.

## Review

A spec is a document with a separate review, which is the whole reason the
`rubber-duck` skill refuses to expand an idea into one. Open it as a pull
request and run `/rev` against that, the same as any other change. A spec that
merges unreviewed sets the vocabulary for every issue cut from it.

## Then

Hand it to `decompose` once the pull request above has merged. Issues cite the
spec's identifier and a worker reads the spec from a worktree cut from `main`,
so issues filed off the branch point at a file nobody else has, and at decisions
review can still change. A spec that produces no issues is either premature or
was not needed.

## Do not

- Write a spec for work whose criteria you could already state.
- Restate the idea. Link it and record what the idea left open.
- Add a `tasks.md`.
- File issues here. That is `decompose`.
