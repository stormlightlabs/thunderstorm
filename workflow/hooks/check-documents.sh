#!/bin/sh
# Run the document gate over a file a session just wrote.
#
# The harness pipes the hook event in and reads the reply out, so this script's
# whole job is to find tstorm and hand it both streams.
#
# Resolution order, which is the one every caller is told: TSTORM_BIN, then the
# payload's own bin/, then PATH. A payload staged somewhere unexpected is a
# setting away from working rather than a bug.
#
# A missing binary is a warning and never a refusal. The gate reports a
# document that cannot be cited, which is not worth stopping a session over,
# and a hook that blocks every write gets uninstalled by the end of the day.
# Nothing below runs an external command before the binary is found, for the
# same reason: a hook on a stripped PATH should still degrade rather than fail.
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
  echo "thunderstorm: tstorm is not installed, so the document gate did not run." >&2
  echo "thunderstorm: install it, or point TSTORM_BIN at it." >&2
  exit 0
fi

exec "$tstorm" hook
