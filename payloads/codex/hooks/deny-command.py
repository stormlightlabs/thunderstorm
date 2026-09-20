#!/usr/bin/env python3
import json
import re
import sys

DENIED = ["gh pr merge","gh pr review","git push","git merge"]


def command_pattern(prefix):
    words = [r'''["']?''' + re.escape(word) + r'''["']?''' for word in prefix.split()]
    return re.compile(r'''(^|[;&|()\n]\s*)''' + r'''\s+'''.join(words) + r'''(?=\s|$|[;&|()])''')


try:
    event = json.load(sys.stdin)
    command = event.get("tool_input", {}).get("command", "")
except (AttributeError, json.JSONDecodeError):
    print("Thunderstorm could not inspect the shell command.", file=sys.stderr)
    raise SystemExit(2)
if not isinstance(command, str):
    print("Thunderstorm received a shell command in an unknown format.", file=sys.stderr)
    raise SystemExit(2)
for prefix in DENIED:
    if command_pattern(prefix).search(command):
        json.dump({
            "hookSpecificOutput": {
                "hookEventName": "PreToolUse",
                "permissionDecision": "deny",
                "permissionDecisionReason": prefix + " is reserved for a human in a Thunderstorm run.",
            }
        }, sys.stdout)
        break
