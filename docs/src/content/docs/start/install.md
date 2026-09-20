---
title: Install
description: Add thunderstorm to Claude Code, Codex, or Pi.
---

Every harness installs from this repository. Pick the one you use.

## Claude Code

```sh
/plugin marketplace add stormlightlabs/thunderstorm
```

## Codex

```sh
codex plugin marketplace add stormlightlabs/thunderstorm
codex plugin add thunderstorm@stormlightlabs
```

`marketplace add` also takes an HTTPS URL, an SSH URL, or a local path, and
`--sparse` restricts the checkout to named paths.

## Pi

```sh
pi install git:github.com/stormlightlabs/thunderstorm
```

Add `-l` to install into the project's own `.pi/settings.json` rather than your
user settings.

## The checks

The skills are prose; `tstorm` is the half that can fail. Install it alongside:

```sh
go install github.com/stormlightlabs/tstorm/cmd/tstorm@latest
```

Released builds are also published for macOS and Linux. Once it is on your
`PATH`, the hooks each harness installs will find it.
