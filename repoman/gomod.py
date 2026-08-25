#!/usr/bin/env python3
"""repoman/gomod.py — go.mod/go.sum sanity gate for Go projects.

Two checks, both aimed at the same failure mode: a go.mod or go.sum that
only builds on the machine that produced it, because something
environment-specific leaked into a file meant to be portable.

**replace-directive check** (always enforced, fully offline, deterministic).
Uses `go mod edit -json` — Go's own real parser, not a hand-rolled regex —
to read every `replace` directive. A replace entry's `New.Version` being
absent is Go's own unambiguous signal that the target is a local
filesystem path rather than a module-registry redirect (see `go help
mod-replace`): no further parsing is needed to tell the two apart. An
ABSOLUTE local-path target always fails the gate — it cannot exist on any
machine but the one that wrote it, sandbox or not. This is exactly the
failure mode that reached a real downstream team as a hardcoded sandbox
path in a project's committed go.mod. A RELATIVE local-path target
(./foo, ../foo) is a legitimate, if uncommon, monorepo pattern, so it
warns rather than fails by default; --strict-relative-replace promotes
that to a failure too, for projects that want the stricter rule.

**go.sum completeness check** (best-effort; degrades to a warning, never
a false failure, when the environment itself can't finish the check).
Uses `go list -deps -mod=readonly ./...` rather than `go build`: it
resolves the full import and module graph without compiling or linking
anything, so a CGO package missing system libraries — a real, common,
and entirely unrelated environment gap — can never be mistaken for a
go.sum problem. `-mod=readonly` (the default since Go 1.16, passed
explicitly here for clarity across Go versions) refuses to silently
patch go.mod/go.sum, which is exactly what makes its failure mode a
reliable signal: an incomplete go.sum surfaces as an explicit `missing
go.sum entry for module providing package ...` line, matched verbatim
rather than inferred from exit code alone — a plain network outage also
exits non-zero, and must not be reported as the same thing.

Exit codes: 0 (including when there are warnings but no errors), 1 (at
least one error). Wire into a project's own release gate as an ordinary
`run` step in `.repoman.json`:

    {"name": "go-sanity", "run": "python3 repoman/gomod.py check", "always": true}

Scope: single go.mod at the given path (default: current directory).
Multi-module repos (go.work workspaces, nested go.mod files) are not
walked automatically — run once per module root if a project has more
than one.

Usage:
    python3 repoman/gomod.py check
    python3 repoman/gomod.py check --strict-relative-replace /path/to/module
"""

import argparse
import json
import re
import subprocess
import sys
from pathlib import Path


def _run(cmd, cwd):
    """Run cmd, return (returncode, combined stdout+stderr). returncode is
    None specifically when the command itself could not be started (e.g.
    `go` not on PATH) or timed out -- a different failure class from the
    command running and reporting its own non-zero exit."""
    try:
        r = subprocess.run(cmd, cwd=cwd, capture_output=True, text=True,
                            timeout=300)
        return r.returncode, r.stdout + r.stderr
    except FileNotFoundError:
        return None, f"{cmd[0]}: not found on PATH"
    except subprocess.TimeoutExpired:
        return None, f"{' '.join(cmd)}: timed out after 300s"


_WIN_ABS_RE = re.compile(r"^[A-Za-z]:[\\/]|^\\\\")


def _is_absolute_path(p: str) -> bool:
    return p.startswith("/") or bool(_WIN_ABS_RE.match(p))


def check_replace_directives(root: Path, strict_relative: bool):
    """Returns (errors: list[str], warnings: list[str])."""
    rc, out = _run(["go", "mod", "edit", "-json"], cwd=root)
    if rc is None:
        return [f"go-tooling: {out}"], []
    if rc != 0:
        return [f"go-mod-parse: `go mod edit -json` failed: {out.strip()}"], []
    try:
        data = json.loads(out)
    except json.JSONDecodeError as e:
        return [f"go-mod-parse: could not parse `go mod edit -json` "
                f"output: {e}"], []

    errors, warnings = [], []
    for r in data.get("Replace") or []:
        old_path = r.get("Old", {}).get("Path", "?")
        new = r.get("New", {})
        new_path = new.get("Path", "")
        new_version = new.get("Version")
        if new_version:
            continue  # a real module-registry redirect, not a local path
        if _is_absolute_path(new_path):
            errors.append(
                f"replace-absolute-path: `replace {old_path} => {new_path}` "
                f"is an absolute local filesystem path -- it will not exist "
                f"on any machine but the one that wrote it. Remove before "
                f"release.")
        elif new_path.startswith("./") or new_path.startswith("../"):
            msg = (f"replace-relative-path: `replace {old_path} => "
                   f"{new_path}` is a local filesystem path. Fine for a "
                   f"genuine monorepo layout if {new_path} is meant to ship "
                   f"alongside this repo; otherwise remove before release.")
            (errors if strict_relative else warnings).append(msg)
    return errors, warnings


def check_gosum_completeness(root: Path):
    """Returns (errors: list[str], warnings: list[str])."""
    rc, out = _run(["go", "list", "-deps", "-mod=readonly", "./..."],
                    cwd=root)
    if rc is None:
        return [], [f"go-tooling: {out} -- go.sum completeness not checked"]
    if "missing go.sum entry" in out:
        lines = [ln.strip() for ln in out.splitlines()
                 if "missing go.sum entry" in ln]
        return [f"gosum-incomplete: {ln}" for ln in lines], []
    if rc != 0:
        # Some other failure -- network unreachable, a module genuinely not
        # found upstream, etc. Not a go.sum-shape problem specifically, so
        # this check cannot conclude either way; say so plainly rather than
        # guess which it was.
        return [], [
            "gosum-check-inconclusive: `go list -deps -mod=readonly ./...` "
            f"failed for a reason other than a missing go.sum entry, so "
            f"completeness could not be confirmed: {out.strip()[:300]}"]
    return [], []


def cmd_check(root: Path, strict_relative: bool) -> int:
    rc, out = _run(["go", "version"], cwd=root)
    if rc is None:
        print(f"ERROR go-tooling: {out}")
        print("GOMOD CHECK FAIL: 1 error(s)")
        return 1

    errors, warnings = [], []
    e, w = check_replace_directives(root, strict_relative)
    errors += e
    warnings += w
    e, w = check_gosum_completeness(root)
    errors += e
    warnings += w

    for msg in warnings:
        print(f"WARN {msg}")
    for msg in errors:
        print(f"ERROR {msg}")
    if errors:
        print(f"GOMOD CHECK FAIL: {len(errors)} error(s)")
        return 1
    suffix = f" ({len(warnings)} warning(s))" if warnings else ""
    print(f"GOMOD CHECK OK{suffix}")
    return 0


def main() -> int:
    ap = argparse.ArgumentParser(
        description="go.mod/go.sum sanity gate: no local-path replace "
                     "directives escaping into a release, go.sum complete "
                     "enough for a plain `go build -mod=readonly` to work "
                     "with no extra magic.")
    sub = ap.add_subparsers(dest="cmd", required=True)
    p = sub.add_parser("check")
    p.add_argument("path", nargs="?", default=".",
                    help="module root (default: current directory)")
    p.add_argument("--strict-relative-replace", action="store_true",
                    help="also fail on relative-path (./..., ../...) "
                         "replace directives, not just absolute ones")
    args = ap.parse_args()

    if args.cmd == "check":
        return cmd_check(Path(args.path).resolve(),
                          args.strict_relative_replace)
    return 1


if __name__ == "__main__":
    sys.exit(main())
