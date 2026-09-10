#!/usr/bin/env python3
"""Reject tracked paths that cannot be checked out portably on Windows."""

import argparse
import os
from pathlib import Path
import re
import subprocess
import sys


RESERVED = re.compile(r"^(CON|PRN|AUX|NUL|COM[1-9¹²³]|LPT[1-9¹²³])$", re.IGNORECASE)
FORBIDDEN = set('<>:"\\|?*')


def check_paths(paths: list[str]) -> list[str]:
    """Check every component, including shared directory spellings and types."""
    errors = set()
    seen = {}
    for path in sorted(set(paths)):
        parts = path.split("/")
        for index, part in enumerate(parts):
            prefix = "/".join(parts[: index + 1])
            kind = "file" if index == len(parts) - 1 else "directory"
            if not part or part in {".", ".."}:
                errors.add(f"{path!r}: empty or relative component")
            elif any(c in FORBIDDEN or ord(c) < 32 or 0xD800 <= ord(c) <= 0xDFFF for c in part):
                errors.add(f"{path!r}: reserved character or invalid Unicode")
            elif part.endswith((" ", ".")):
                errors.add(f"{path!r}: component ends with a space or period")
            elif RESERVED.fullmatch(part.split(".", 1)[0].rstrip(" ")):
                errors.add(f"{path!r}: reserved Windows device name")

            # Prefixes catch Src/a + src/b and a file A + directory a/b too.
            key = prefix.casefold()
            previous = seen.setdefault(key, (prefix, kind))
            if previous != (prefix, kind):
                errors.add(f"{prefix!r} ({kind}) collides with {previous[0]!r} ({previous[1]})")
    return sorted(errors)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--stdin", action="store_true", help="read NUL-separated Git paths from stdin")
    args = parser.parse_args()
    try:
        data = sys.stdin.buffer.read() if args.stdin else subprocess.run(
            ["git", "ls-files", "--cached", "-z"],
            cwd=Path(__file__).resolve().parents[1], check=True, capture_output=True,
        ).stdout
    except (OSError, subprocess.CalledProcessError) as error:
        print(f"tracked-paths: could not read path list: {error}", file=sys.stderr)
        return 2
    paths = [os.fsdecode(path) for path in data.split(b"\0") if path]
    errors = check_paths(paths)
    if errors:
        for error in errors:
            print(f"tracked-paths: {error}", file=sys.stderr)
        return 1
    print(f"tracked-paths: OK ({len(paths)} files)")
    return 0


if __name__ == "__main__":
    sys.exit(main())
