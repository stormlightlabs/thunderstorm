#!/bin/sh
# Run one of the loop's gates over what a session just did.
#
# The harness pipes the hook event in and reads the reply out, so this script's
# whole job is to find tstorm and hand it both streams. Which gate runs is
# tstorm's to decide from the event: a write is read for the frontmatter an
# issue cites it by and for the tells the catalogue lists, and a commit is read
# for its shape.
#
# Resolution order, which is the one every caller is told: TSTORM_BIN, then the
# payload's own bin/, then PATH. A payload staged somewhere unexpected is a
# setting away from working rather than a bug.
#
# A missing binary lets everything through, and says so once: this runs on
# every shell command a session makes, so a warning per call would be the
# loudest thing in the transcript. The stamp file lives in TMPDIR, so the
# reminder comes back after a reboot.
#
# Nothing below runs an external command before the binary is found: a hook on
# a stripped PATH should degrade rather than fail.
set -eu

here=${0%/*}
if [ "$here" = "$0" ]; then
  here=.
fi

if [ -n "${TSTORM_BIN:-}" ] && [ -x "$TSTORM_BIN" ]; then
  tstorm=$TSTORM_BIN
elif [ -x "$here/../bin/tstorm" ]; then
  tstorm=$here/../bin/tstorm
elif command -v tstorm >/dev/null 2>&1; then
  tstorm=tstorm
else
  stamp="${TMPDIR:-/tmp}/tstorm-gate-missing.$(id -u 2>/dev/null || echo 0)"
  if [ ! -f "$stamp" ]; then
    echo "thunderstorm: tstorm is not installed, so the loop's gates did not run." >&2
    echo "thunderstorm: install it, or point TSTORM_BIN at it." >&2
    : >"$stamp" 2>/dev/null || true
  fi
  exit 0
fi

exec "$tstorm" hook
