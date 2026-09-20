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

CI runs all of that, plus the Python suites and the site's Playwright tests,
on every pull request and every push to `main` and `edge`.

A change under `workflow/` is not done until the payload is rendered again:

```sh
go run ./cmd/tstorm render --target claude
go run ./cmd/tstorm render --target claude --check   # what CI will ask
```

`payloads/` is generated. Edit `workflow/` and re-render; a hand edit there is
lost at the next render, and `--check` is what catches it.

## The board

Issues live on GitHub Projects board **13**, owner `stormlightlabs`, under the
**THNDRS** track. The `github-board` skill writes `Todo`, `In Progress` and
`Done` there and reads it filtered to this repository; it takes the number and
the owner from here.

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
git config core.hooksPath .githooks   # once, to have git run it for you
```

`.githooks/commit-msg` calls the same script. Set `core.hooksPath` only when
you mean to: it replaces `.git/hooks` wholesale, so every hook you already had
stops firing.

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

Everything written here is read by a person, so everything written here gets a
deslop pass before it lands. That covers documentation, commit messages, pull
request titles and bodies, issue text, review and edit comments, and the
replies a session writes back. A commit message is communication: it is read
far more often than the diff it describes.

`workflow/skills/writing-docs/` is the standard and
`workflow/skills/writing-docs/references/tells.md` is the catalogue. Read the
catalogue and check the draft against it before committing, opening, or
posting. The pass is not optional because the text is short, internal, or
written by an agent.

`check-commit-message.py` does not do this. It checks the type, the subject
length, the blank line, and the column limit, and it will pass a message full
of tells; a clean run from it means the shape is right and nothing more.

What the pass looks for, in the catalogue's terms: bold-first bullets used as a
template, ceremonial endings, the same point restated at three levels, a
recurring three-part rhythm, negative reframes, self-answered questions,
standalone fragments for emphasis, manufactured stakes, promotional adjectives
in place of a named property, and the reserved words — bounded, contract,
boundary, invariant, guarantee, safe, minimal — used loosely.

Two rules that are about accuracy rather than style. Quote a file only after
opening it, since a characterisation repeated from a summary is how a phrase
nothing says ends up in a commit message. And keep implementation detail and
planning vocabulary out of anything a user reads.

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
