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
# A missing binary lets everything through. The gates report what they find and
# refuse only a commit message that will not read in git log, and a hook that
# fails closed on a machine missing a binary gets uninstalled by the end of the
# day. Nothing below runs an external command before the binary is found, for
# the same reason: a hook on a stripped PATH should still degrade rather than
# fail.
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
  echo "thunderstorm: tstorm is not installed, so the loop's gates did not run." >&2
  echo "thunderstorm: install it, or point TSTORM_BIN at it." >&2
  exit 0
fi

exec "$tstorm" hook
