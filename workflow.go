// Package thunderstorm carries workflow source that tstorm reads at runtime.
//
// The role definitions are the only part so far. Claude Code provisions a
// subagent from one and Codex spawns an agent; Pi has neither mechanism, so
// tstorm dispatch starts the session and needs the role's prompt and its tool
// list on a machine where no payload carries either.
//
// These are the files render copies, so a role edited under workflow/ reaches
// dispatch at the next build and the payload at the next render.
package thunderstorm

import "embed"

// Roles holds workflow/agents, one Markdown definition per role.
//
//go:embed workflow/agents/*.md
var Roles embed.FS
