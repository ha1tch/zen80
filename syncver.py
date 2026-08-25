#!/usr/bin/env python3
"""repoman/syncver.py — keep a repository's version stamps in sync.

The canonical version lives in a plain-text file (config
`version_file`, default `VERSION`). Additional files carry the version
embedded in code or docs; each is declared in config `version_targets`
as {"file": path, "match": regex-with-exactly-one-capture-group}, and
`set` rewrites only the captured group, leaving everything else
byte-identical.

CLI:  show | set <version> | check | bump-patch|bump-minor|bump-major
Importable: set_version(v), check() -> bool (safe for `if not
check():`), check_detail() -> (bool, str) when the message matters --
relcore.py's own syncver builtin uses check_detail().
"""

import re
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import config as _config

VERSION_RE = re.compile(r"^[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.]+)?$")


def _env(root, cfg):
    if root is None or cfg is None:
        root, cfg = _config.load()
    return root, cfg


def get_version(root=None, cfg=None) -> str:
    root, cfg = _env(root, cfg)
    f = root / cfg["version_file"]
    return f.read_text().strip() if f.is_file() else ""


def set_version(new: str, root=None, cfg=None) -> None:
    root, cfg = _env(root, cfg)
    if not VERSION_RE.match(new):
        raise ValueError(f"invalid version {new!r} (want X.Y.Z[-suffix])")
    (root / cfg["version_file"]).write_text(new + "\n")
    for t in cfg["version_targets"]:
        p = root / t["file"]
        text = p.read_text()
        pat = re.compile(t["match"], re.M)
        m = pat.search(text)
        if not m or m.lastindex != 1:
            raise ValueError(
                f"{t['file']}: pattern must match with exactly one group")
        p.write_text(text[:m.start(1)] + new + text[m.end(1):])


def check_detail(root=None, cfg=None) -> tuple[bool, str]:
    """(in_sync, detail) -- the canonical version, or the first
    mismatch found. Use this when the message matters (CLI output,
    logging); use check() for a plain boolean test.

    A version returning ONLY a tuple here was a real, found footgun:
    a non-empty tuple is always truthy in Python regardless of its
    content, so a caller writing the natural `if not check():` idiom
    against a two-element tuple silently never triggers, no matter
    what the actual sync state is. Found 2026-08-17 tracing a real
    downstream consumer's own release orchestration, which does
    exactly that. check() below exists so that idiom is simply
    correct; this function is for the cases that need more than a
    bool."""
    root, cfg = _env(root, cfg)
    canon = get_version(root, cfg)
    if not canon:
        return False, f"{cfg['version_file']} missing or empty"
    for t in cfg["version_targets"]:
        text = (root / t["file"]).read_text()
        m = re.search(t["match"], text, re.M)
        got = m.group(1) if m else "<no match>"
        if got != canon:
            return False, f"{t['file']}: {got} != {canon}"
    return True, canon


def check(root=None, cfg=None) -> bool:
    """Plain boolean: True iff VERSION and every configured target
    agree. Safe for `if not check():` -- see check_detail's own doc
    comment for why that safety is stated explicitly rather than
    assumed."""
    return check_detail(root, cfg)[0]


def bump(part: str, root=None, cfg=None) -> str:
    cur = get_version(root, cfg).split("-")[0]
    major, minor, patch = (int(x) for x in cur.split("."))
    if part == "major":
        major, minor, patch = major + 1, 0, 0
    elif part == "minor":
        minor, patch = minor + 1, 0
    else:
        patch += 1
    new = f"{major}.{minor}.{patch}"
    set_version(new, root, cfg)
    return new


def main(argv) -> int:
    cmd = argv[0] if argv else "show"
    try:
        if cmd == "show":
            ok, detail = check_detail()
            print(f"version: {get_version()}  sync: "
                  f"{'ok' if ok else 'MISMATCH — ' + detail}")
        elif cmd == "set":
            set_version(argv[1])
            print(f"version set to {argv[1]}")
        elif cmd == "check":
            ok, detail = check_detail()
            print(("OK: versions in sync (%s)" if ok
                   else "MISMATCH: %s") % detail)
            return 0 if ok else 1
        elif cmd in ("bump-patch", "bump-minor", "bump-major"):
            print(f"version set to {bump(cmd.split('-')[1])}")
        else:
            print(f"unknown command {cmd!r}", file=sys.stderr)
            return 1
    except (ValueError, IndexError, OSError) as e:
        print(f"error: {e}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
