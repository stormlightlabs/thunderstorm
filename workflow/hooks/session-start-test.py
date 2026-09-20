#!/usr/bin/env python3
"""Check session-start.sh against stub toolchains.

    .claude/hooks/session-start-test.py

The hook warms caches, and every step of it is written to report a failure and
carry on. That rule is invisible in the diff of any one line: dropping a `||`
leaves a script that still reads correctly and now fails a session whose
container happened to be offline. These cases hold it in place.

Each case runs the hook against a directory of stub `rustup` and `cargo`
commands, so nothing here downloads anything or depends on what the runner
already has.

Stdout is asserted empty throughout: a SessionStart hook's stdout lands in the
session's context, and a package list there costs tokens on every session.
"""

from __future__ import annotations

import shutil
import subprocess
import tempfile
from pathlib import Path

HOOKS = Path(__file__).resolve().parent
HOOK = HOOKS / "session-start.sh"
REPOSITORY = HOOKS.parent.parent

TOOLS = ("rustup", "cargo")

# The stubs are the whole PATH, so a cargo the machine already carries cannot
# answer for the one the hook calls. Nothing else on PATH is needed: the hook
# calls no other command, and bash is launched by the path it resolves to here
# rather than through the PATH the hook is handed.
BASH = shutil.which("bash") or "/bin/bash"

failures: list[str] = []


def write_stubs(directory: Path, status: int) -> Path:
    """Write a stub for every command the hook calls and return their directory."""
    binaries = directory / "bin"
    binaries.mkdir()
    for tool in TOOLS:
        stub = binaries / tool
        stub.write_text(
            f'#!/bin/sh\nprintf "%s\\n" "$*" >> "$STUB_LOG"\necho "{tool}: stub" \nexit {status}\n'
        )
        stub.chmod(0o755)
    return binaries


def run(
    *, remote: bool = True, status: int = 1
) -> tuple[subprocess.CompletedProcess[str], str]:
    """Run the hook against fresh stubs and return it with what they recorded."""
    with tempfile.TemporaryDirectory() as temporary:
        directory = Path(temporary)
        binaries = write_stubs(directory, status)
        log = directory / "calls.txt"
        log.touch()
        environment = {
            "PATH": str(binaries),
            "STUB_LOG": str(log),
            "CLAUDE_PROJECT_DIR": str(REPOSITORY),
        }
        if remote:
            environment["CLAUDE_CODE_REMOTE"] = "true"
        completed = subprocess.run(
            [BASH, str(HOOK)], env=environment, capture_output=True, text=True, check=False
        )
        return completed, log.read_text()


def check(case: str, condition: bool, detail: str) -> None:
    if not condition:
        failures.append(f"{case}: {detail}")


outside, _ = run(remote=False)
check("outside the cloud", outside.returncode == 0, f"exited {outside.returncode}")
check("outside the cloud", outside.stdout == "", f"wrote {outside.stdout!r} to stdout")
check("outside the cloud", outside.stderr == "", f"wrote {outside.stderr!r} to stderr")

broken, calls = run(status=1)
check("every step fails", broken.returncode == 0, f"exited {broken.returncode}")
check("every step fails", broken.stdout == "", f"wrote {broken.stdout!r} to stdout")
for message in (
    "updating the stable toolchain failed",
    "selecting the stable toolchain failed",
    "warming the cargo registry failed",
):
    check("every step fails", message in broken.stderr, f"did not report {message!r}")

check("--locked reaches cargo fetch", "fetch --locked" in calls, f"cargo was called as {calls!r}")

working, working_calls = run(status=0)
check("every step succeeds", working.returncode == 0, f"exited {working.returncode}")
check("every step succeeds", working.stdout == "", f"wrote {working.stdout!r} to stdout")
check(
    "every step succeeds",
    "failed" not in working.stderr,
    f"reported a failure anyway: {working.stderr!r}",
)
check(
    "every step succeeds",
    "update --no-self-update stable" in working_calls,
    f"rustup was called as {working_calls!r}",
)

if failures:
    for failure in failures:
        print(failure)
    raise SystemExit(f"{len(failures)} case(s) failed")

print("session-start.sh reports every failure and exits zero in all of them")
