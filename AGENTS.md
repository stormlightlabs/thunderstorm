# thunderstorm

A development loop that installs into a repository and runs on whichever coding
agent is available. `workflow/` is the source of that loop, `payloads/` is what
each harness installs, and `tstorm` is the binary holding the parts that have to
be true rather than merely instructed.

## Gates

Run the narrowest relevant test first, then these:

```sh
gofmt -l .        # prints nothing when the tree is formatted
go vet ./...
go test ./...
```

A change under `workflow/` is not done until the payload is rendered again:

```sh
go run ./cmd/tstorm render --target claude
go run ./cmd/tstorm render --target claude --check   # what CI will ask
```

`payloads/` is generated. Edit `workflow/` and re-render; a hand edit there is
lost at the next render, and `--check` is what catches it.

## Commits

One deliverable per branch, and a commit message written for whoever reads
`git log` in a year:

```text
<type>: <what changed, imperative, lowercase, under 60 characters>

<why it changed, wrapped at 72 columns>
```

Types: `feat`, `fix`, `docs`, `refactor`, `test`, `chore`, `perf`. The body
carries the reasoning — what was true before, what the change makes true, and
what it deliberately left alone. No headings: `## What` reaches `git log` as
those literal characters.

Check the shape before committing:

```sh
python3 workflow/scripts/check-commit-message.py <file>
```

It checks the type, the 60-character subject, the blank line, and the 72-column
body, and exempts fenced blocks, trailers, and URLs from the column limit. The
`commits-and-prs` skill under `workflow/skills/` holds the rest, including the
pull request targets that become the squash commit.

## Closing an issue

A commit closes its issue with a `Closes #NN` line above the trailers. GitHub
acts on that keyword only when the commit reaches the default branch, which
here is `main`, so a commit merged through `edge` links the issue and leaves it
open until release. Say `Closes #NN` when the work finishes the issue, and name
the issue without the keyword when it only advances it.

GitHub also refuses to close an issue that still has open blocking
dependencies. Check `gh issue view <n>` for a `blocked-by:` line before relying
on the keyword.

Push only when asked. Nothing here merges its own work: `gh pr merge`,
`gh pr review`, `git merge` and a bare `git push` are denied, and a human
merges to `edge`.

## Prose

Documentation, commit bodies, issue text and skill bodies are all written for a
human reader. `workflow/skills/writing-docs/` is the standard, and it catalogues
the tells to avoid. Keep implementation detail out of anything a user reads, and
keep planning vocabulary out of everything.

## Where things live

```text
cmd/tstorm      entry point
internal/       tstorm source
workflow/       the loop, written once
payloads/       rendered per harness, committed, never hand-edited
docs/           the published site
docs/internal/  working documents, not published
CHANGELOG.md    what has landed, Keep a Changelog format
TODO.md         what is left, one section per umbrella issue
```
