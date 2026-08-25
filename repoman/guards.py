#!/usr/bin/env python3
"""
repoman/guards.py — the dormant-guard registry with teeth.

A dormant guard is any verification that does not run in the default
test invocation: stress/build-tagged tests, fuzz targets, multi-core
race harnesses, long-running local suites. The registry lives in
docs/KNOWN_ISSUES.md ("Dormant guards" section, G-nn entries).

THE PRINCIPLE THIS TOOL SERVES: a shipped guard that never runs guards
nothing. A guard's specification (the test exists) and its execution
record (it ran, when, where) are different facts — only the second is
evidence. This tool makes the execution record queryable and updatable
so the release process can check guard CURRENCY mechanically instead of
by memory.

Commands:

    list                     one line per guard: id, last exercised, title
    show G-nn                print a guard's full registry block
    handoff [G-nn ...]       emit a ready-to-run block (invocations +
                             reporting instructions) for guards needing
                             hardware this environment lacks — hand the
                             output to whoever has the multi-core box;
                             default: all guards
    record G-nn --date YYYY-MM-DD --env ENV [--note TEXT]
                             update a guard's Last exercised line after
                             a run is reported; the previous record is
                             preserved as a "Previous:" suffix
    stale [--since YYYY-MM-DD]
                             guards not exercised since the date
                             (default: the previous release's changelog
                             date). Exit 1 if any are stale — run each,
                             hand it off, or record the skip explicitly
                             in the release's changelog entry.

At release time: run `stale`; every listed guard must be exercised,
handed off, or have its skip recorded in the changelog entry — Part 3
§6/§8 of the working practices, mechanized.

Env values by convention: sandbox (single-core Linux), m1 (8-core
macOS), gh-runner (GitHub Actions multi-core).
"""

import argparse
import datetime
import re
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import config as _config

_ROOT, _CFG = _config.load()
ROOT = _ROOT
KNOWN_ISSUES = _ROOT / _CFG["known_issues"]
# guard_id_prefix: the full prefix including separator (unlike
# register.py's id_prefix/id_separator split, guards have no
# mid-project migration need -- a plain, single configured prefix
# string is sufficient generality here). Default "G-" reproduces this
# project's original, only-ever-tested behaviour for any consumer that
# doesn't set this key.
_GP = _CFG.get("guard_id_prefix", "G-")
_GP_RE = re.escape(_GP)

HEAD_RE = re.compile(r"^### (" + _GP_RE + r"\d+[a-z]?)\.\s*(.*)$", re.M)
LAST_RE = re.compile(
    r"^- \*\*Last exercised[^:*]*:\*\*\s*(.*(?:\n[ \t]+\S.*)*)", re.M)
DATE_RE = re.compile(r"(\d{4}-\d{2}-\d{2})")
INVOKE_RE = re.compile(r"^\s*[-|]?\s*\*?\*?Invocation:?\*?\*?\s*`([^`]+)`", re.M)
TABLE_INVOKE_RE = re.compile(
    r"^\|\s*(" + _GP_RE + r"\d+[a-z])\s*\|[^|]*\|[^|]*\|\s*`([^`]+)`\s*\|", re.M)


class Guard:
    def __init__(self, gid, title, block, span):
        self.gid, self.title, self.block, self.span = gid, title, block, span

    @property
    def last_line(self):
        m = LAST_RE.search(self.block)
        return m.group(1).strip() if m else None

    @property
    def last_date(self):
        """The most recent ISO-8601 date mentioned in the bullet, not
        just the first. A "Last exercised" bullet can carry multiple
        dated addenda -- a tool appending new-first ("DATE ... Previous:
        OLDER-DATE ..."), but this document also carries many
        hand-written, free-form bullets where dates appear in whatever
        order the prose put them, not that convention. max() over
        "YYYY-MM-DD" strings is correct for both shapes -- lexicographic
        comparison is equivalent to chronological comparison for this
        format, so position in the text never matters. Backported from
        xolu's own local fork 2026-08-17, verified there against a real
        hand-written multi-sentence bullet before being trusted."""
        if not self.last_line:
            return None
        dates = DATE_RE.findall(self.last_line)
        return max(dates) if dates else None

    @property
    def invocations(self):
        out = [m.group(1) for m in INVOKE_RE.finditer(self.block)]
        out += [f"{m.group(2)}  # {m.group(1)}"
                for m in TABLE_INVOKE_RE.finditer(self.block)]
        return out


def parse() -> tuple[str, list[Guard]]:
    text = KNOWN_ISSUES.read_text()
    sec = re.search(r"^## Dormant guards.*$", text, re.M)
    if not sec:
        print("docs/KNOWN_ISSUES.md: no 'Dormant guards' section — Part 3 §8 "
              "requires one; create it before registering guards",
              file=sys.stderr)
        sys.exit(2)
    heads = list(HEAD_RE.finditer(text, sec.end()))
    guards = []
    for i, h in enumerate(heads):
        end = heads[i + 1].start() if i + 1 < len(heads) else len(text)
        guards.append(Guard(h.group(1), h.group(2).strip(),
                            text[h.start():end], (h.start(), end)))
    return text, guards


def previous_release_date() -> str:
    """Date of the SECOND changelog entry — i.e. the previous release:
    the default staleness horizon (exercised since the last tag)."""
    dates = re.findall(r"^## \[[^\]]+\] - (\d{4}-\d{2}-\d{2})",
                       (ROOT / _CFG["changelog"]).read_text(), re.M)
    return dates[1] if len(dates) > 1 else (dates[0] if dates else "1970-01-01")


def cmd_list(guards):
    for g in guards:
        print(f"{g.gid:<6} last={g.last_date or 'NEVER':<12} {g.title}")
    return 0


def cmd_show(guards, gid):
    for g in guards:
        if g.gid == gid:
            print(g.block.rstrip())
            return 0
    print(f"no such guard: {gid}", file=sys.stderr)
    return 1


def cmd_handoff(guards, ids):
    chosen = [g for g in guards if not ids or g.gid in ids]
    print("# Dormant-guard handoff — run on multi-core hardware and report back")
    print("# For each command: run from the repository root; capture the full")
    print("# output; report PASS/FAIL, iteration counts, and wall time.")
    print("# On report, the registry updates with:")
    print(f"#   python3 {Path(sys.argv[0]).name} record <{_GP}nn> --date <date> --env m1")
    print()
    for g in chosen:
        print(f"## {g.gid}. {g.title}")
        print(f"#  last exercised: {g.last_line or 'NEVER'}")
        for inv in g.invocations or ["# (no invocation recorded in registry "
                                     "— see the guard's block)"]:
            print(inv)
        print()
    return 0


def cmd_record(text, guards, gid, date, env, note, dry):
    if not re.match(r"^\d{4}-\d{2}-\d{2}$", date):
        print(f"invalid date: {date} (want YYYY-MM-DD)", file=sys.stderr)
        return 1
    for g in guards:
        if g.gid != gid:
            continue
        m = LAST_RE.search(g.block)
        if not m:
            print(f"{gid}: no '- **Last exercised:**' line in its block — "
                  "add one by hand first", file=sys.stderr)
            return 1
        old = m.group(1).strip()
        new_val = f"{date} env:{env}"
        if note:
            new_val += f" — {note}"
        prev = re.sub(r"\s*Previous( exercise)?:.*$", "", old)
        new_val += f" Previous: {prev}"
        new_block = g.block[:m.start(1)] + new_val + g.block[m.end(1):]
        new_text = text[:g.span[0]] + new_block + text[g.span[1]:]
        if dry:
            print(f"(dry-run) {gid} Last exercised would become:\n  {new_val}")
            return 0
        KNOWN_ISSUES.write_text(new_text)
        print(f"{gid} recorded: {new_val}")
        return 0
    print(f"no such guard: {gid}", file=sys.stderr)
    return 1


def cmd_stale(guards, since):
    stale = []
    for g in guards:
        # Sub-entries (G-03a...) are covered by their parent's collective
        # Last exercised line; only top-level blocks carry the record.
        if g.last_line is None:
            continue
        if g.last_date is None or g.last_date < since:
            stale.append(g)
    if not stale:
        print(f"all guards exercised since {since}")
        return 0
    print(f"STALE (not exercised since {since}):")
    for g in stale:
        print(f"  {g.gid:<6} last={g.last_date or 'NEVER'}  {g.title}")
    print("Each must be run, handed off (guards.py handoff), or its skip "
          "recorded explicitly in the release's changelog entry.")
    return 1


def main() -> int:
    ap = argparse.ArgumentParser(description="Dormant-guard registry operations")
    sub = ap.add_subparsers(dest="cmd", required=True)
    sub.add_parser("list")
    p = sub.add_parser("show")
    p.add_argument("guard")
    p = sub.add_parser("handoff")
    p.add_argument("guards", nargs="*")
    p = sub.add_parser("record")
    p.add_argument("guard")
    p.add_argument("--date", required=True)
    p.add_argument("--env", required=True)
    p.add_argument("--note")
    p.add_argument("--dry-run", action="store_true")
    p = sub.add_parser("stale")
    p.add_argument("--since", help="default: previous release's changelog date")
    args = ap.parse_args()

    text, guards = parse()
    if args.cmd == "list":
        return cmd_list(guards)
    if args.cmd == "show":
        return cmd_show(guards, args.guard)
    if args.cmd == "handoff":
        return cmd_handoff(guards, args.guards)
    if args.cmd == "record":
        return cmd_record(text, guards, args.guard, args.date, args.env,
                          args.note, args.dry_run)
    if args.cmd == "stale":
        return cmd_stale(guards, args.since or previous_release_date())
    return 1


if __name__ == "__main__":
    sys.exit(main())
