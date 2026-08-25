#!/usr/bin/env python3
"""
repoman/register.py — mechanized operations on the live register
(docs/TRACKING.md) and resolution record (docs/RESOLVED.md).

The closure procedure (TRACKING_PRACTICES.md, "Closure procedure") is
precisely specified and repeatedly hand-executed — which is exactly
where hand-editing mistakes live. This tool makes the procedure's
violations unmakeable rather than detected after the fact.

Diverges from upstream (github.com/ha1tch/repoman), not yet synced back:
[RESOLVED 2026-08-17, downstream in xolu] cmd_list's sort key was
hardcoded int(t[2:]), assuming a one-character id_prefix -- and
next_id() checked only TRACKING.md, never RESOLVED.md. Both fixed
independently in xolu (T-163 and an earlier session) before this note
was reconciled against xolu's actual current state, which had drifted
ahead of what this docstring described. Both fixes are now upstream
here too, generalized further: id_prefix, id_separator, and an
optional legacy_id_prefix/legacy_id_separator pair (config.py) replace
every hardcoded separator-plus-digits pattern and len(_P)+1 assumption
and an _id_num() helper -- additive, opt-in, byte-identical default
behaviour for any consumer that doesn't set the new keys. Motivated by
xolu's own real, permanent need: a mid-project id-prefix migration
(T-1..T-163 frozen in "T-NNN" shape forever; T-164 onward forward-only
in a new, unhyphenated "XOTNNN" shape) that this project's original
single-format model could not express at all.

Commands:

    list                       one line per open item
    show T-nn                  print an item's detail section
    add  --summary S --theme T --priority Pn [--status ☐]
         [--blocks TEXT] [--body TEXT | --body-file F] [--dry-run]
                               file a new item: next free id, row +
                               detail section inserted together
    close T-nn --version X.Y.Z [--date YYYY-MM-DD] [--dry-run]
                               closure procedure: detail text moved to
                               the top of RESOLVED.md stamped with
                               version+date; row AND detail removed
                               from the register in the same operation
    check                      register consistency (the release gate's
                               A1–A3 delegate here — one implementation,
                               so the gate and this editor cannot
                               disagree about what consistent means)

Always run `check` (or the release gate) after any manual edit to the
register. `--dry-run` prints what would change without writing.

Status symbols: ✓ done · ◐ partial · ☐ not started · ✗ dropped.
A ✓ item in the register is itself a defect — close it instead.
"""

import argparse
import datetime
import difflib
import re
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import config as _config

_ROOT, _CFG = _config.load()
TRACKING = _ROOT / _CFG["tracking"]
RESOLVED = _ROOT / _CFG["resolved"]
_P = _CFG["id_prefix"]
_SEP = _CFG.get("id_separator", "-")
_LEGACY_P = _CFG.get("legacy_id_prefix", "")
_LEGACY_SEP = _CFG.get("legacy_id_separator", "-")

# _ID_ALT: the id shape as a regex fragment (not compiled -- used in
# string concatenation throughout, matching this project's own
# existing style of building patterns from configured pieces). With
# no legacy prefix configured (the default), this is exactly
# re.escape(_P) + re.escape(_SEP) + r"\d+" -- identical to every
# consumer's existing, only-ever-tested behaviour. A legacy prefix
# adds a second alternative so ids from BEFORE a mid-project prefix
# migration are still recognised everywhere an id can appear, without
# ever being GENERATED again (next_id() always issues the primary
# shape; see next_id's own doc comment).
if _LEGACY_P:
    _ID_ALT = (r"(?:" + re.escape(_P) + re.escape(_SEP) + r"|"
               + re.escape(_LEGACY_P) + re.escape(_LEGACY_SEP) + r")\d+")
else:
    _ID_ALT = re.escape(_P) + re.escape(_SEP) + r"\d+"


def _id_num(tid: str) -> int:
    """The numeric portion of an id, primary or legacy shape -- the
    two may differ in both prefix and separator (xolu: "XOT194" vs.
    "T-71"), so a fixed-offset slice is wrong for one of them whenever
    a legacy format is configured. Raises ValueError on anything
    matching neither configured shape, rather than silently returning
    a wrong number."""
    primary = _P + _SEP
    if tid.startswith(primary):
        return int(tid[len(primary):])
    if _LEGACY_P:
        legacy = _LEGACY_P + _LEGACY_SEP
        if tid.startswith(legacy):
            return int(tid[len(legacy):])
    raise ValueError(f"unrecognized id format: {tid!r}")


STATUS_SYMBOLS = {"✓", "◐", "☐", "✗"}
ID_RE = re.compile(_ID_ALT)
ROW_RE = re.compile(r"^\| (" + _ID_ALT + r") \|")
HEAD_RE = re.compile(r"^### (" + _ID_ALT + r")\. (.*)$", re.M)
FIELD_RE = re.compile(
    r"^Theme: (\S+) · Priority: \*{0,2}(P\d)\*{0,2} · Status: (\S+)(?: · Blocks/after: (.*))?$",
    re.M)


class Item:
    def __init__(self, tid, title, theme, priority, status, blocks, body):
        self.tid, self.title = tid, title
        self.theme, self.priority, self.status = theme, priority, status
        self.blocks = blocks or ""
        self.body = body  # detail text below the field line, verbatim

    def field_line(self):
        s = f"Theme: {self.theme} · Priority: {self.priority} · Status: {self.status}"
        if self.blocks:
            s += f" · Blocks/after: {self.blocks}"
        return s


class Register:
    """Parsed view of TRACKING.md. parse() and render-back are
    edit-in-place on the original text (surgical splices), so untouched
    content — including formatting this parser does not model — is
    preserved byte-for-byte."""

    def __init__(self, text: str):
        self.text = text
        self.rows = {}     # tid -> (theme, pri, status, raw row line)
        self.items = {}    # tid -> Item
        self.spans = {}    # tid -> (start, end) offsets of the detail section
        self._parse()

    def _parse(self):
        for m in re.finditer(r"^\| (" + _ID_ALT + r") \|([^|]*)\|([^|]*)\|([^|]*)\|([^|]*)\|.*$",
                             self.text, re.M):
            self.rows[m.group(1)] = (m.group(3).strip(),
                                     m.group(4).strip().strip('*'),
                                     m.group(5).strip(), m.group(0))
        heads = list(HEAD_RE.finditer(self.text))
        for i, h in enumerate(heads):
            start = h.start()
            # section ends at the next ### / ## heading or trailing ---
            tail = self.text[h.end():]
            nm = re.search(r"^(### |## |---\s*$)", tail, re.M)
            end = h.end() + (nm.start() if nm else len(tail))
            block = self.text[start:end]
            fm = FIELD_RE.search(block)
            if not fm:
                continue
            body_start = block.index(fm.group(0)) + len(fm.group(0))
            self.items[h.group(1)] = Item(
                h.group(1), h.group(2).strip(), fm.group(1), fm.group(2),
                fm.group(3), fm.group(4), block[body_start:].rstrip() + "\n")
            self.spans[h.group(1)] = (start, end)

    def next_id(self) -> str:
        """Next free id, considering both TRACKING.md (open items) and
        RESOLVED.md (closed items) -- an id closed earlier in the same
        session must not be reissued. Found 2026-08-03 in seam-ui: a
        just-closed T-45 (release.sh fix) was silently reassigned to a
        new, unrelated item, since this method previously scanned only
        self.rows/self.items (TRACKING.md). RESOLVED.md may not exist
        yet (a brand-new register); that is not an error here.

        RESOLVED.md's own scan is deliberately anchored to the closure
        header's structured position ("## [version] T-nn -- ..."), not
        a bare id-shaped substring anywhere in the entry's free-form
        prose. Found 2026-08-04 in seam-ui: a closure narrative quoting
        another project's own internal tracking IDs verbatim (xolu's
        CHANGELOG mentioning its own "T-160"/"T-161") was misread as a
        real id in THIS repository's own sequence, jumping next_id()
        from T-70 to T-161. TRACKING.md's own rows/items were already
        immune (parsed from structured table rows and detail headers,
        never a raw full-text scan) -- this fix brings RESOLVED.md's
        scan to the same standard, not a new one.
        """
        ids = [_id_num(t) for t in set(self.rows) | set(self.items)]
        if RESOLVED.exists():
            resolved_text = RESOLVED.read_text()
            header_re = re.compile(r"^## \[.*?\]\s+(" + _ID_ALT + r")\s", re.MULTILINE)
            ids += [_id_num(m.group(1)) for m in header_re.finditer(resolved_text)]
        # Always issues the PRIMARY shape -- forward-only. A legacy
        # id's own numeric range is never reused as a collision check
        # here (both are unioned into one `ids` list above), but new
        # ids are never minted in the legacy shape regardless of which
        # format happened to hold the current maximum.
        next_n = (max(ids) + 1) if ids else 1
        digits = f"{next_n:02d}" if next_n < 100 else f"{next_n}"
        return f"{_P}{_SEP}{digits}"

    def check(self, r) -> None:
        """A1–A3 plus symbol validity. `r` needs .err(section, msg) and
        .warn(section, msg) — the release gate's Report qualifies."""
        for tid, (_, _, status, raw) in self.rows.items():
            if "✓" in raw:
                r.err("A1", f"closed item in register: {raw.strip()[:70]}")
        only_rows = set(self.rows) - set(self.items)
        only_items = set(self.items) - set(self.rows)
        if only_rows:
            r.err("A2", f"rows without detail blocks: {sorted(only_rows)}")
        if only_items:
            r.err("A2", f"detail blocks without rows: {sorted(only_items)}")
        for tid in set(self.rows) & set(self.items):
            it = self.items[tid]
            row = self.rows[tid][:3]
            det = (it.theme, it.priority, it.status)
            if row != det:
                r.err("A3", f"{tid}: table {row} vs detail {det}")
            if it.status not in STATUS_SYMBOLS:
                r.err("A3", f"{tid}: unknown status symbol {it.status!r}")


def load() -> Register:
    return Register(TRACKING.read_text())


def write_with_diff(path: Path, new_text: str, dry: bool, label: str) -> None:
    old = path.read_text() if path.exists() else ""
    if old == new_text:
        print(f"   no change: {path.name}")
        return
    if dry:
        diff = difflib.unified_diff(old.splitlines(keepends=True),
                                    new_text.splitlines(keepends=True),
                                    fromfile=f"a/{path.name}", tofile=f"b/{path.name}")
        sys.stdout.writelines(list(diff)[:80])
        print(f"   (dry-run) {label}: {path.name} not written")
        return
    path.write_text(new_text)
    print(f"   {label}: {path.name} updated")


def cmd_list(reg: Register) -> int:
    for tid in sorted(reg.items, key=_id_num):
        it = reg.items[tid]
        print(f"{tid}  {it.status}  {it.priority}  [{it.theme}]  {it.title}")
    return 0


def cmd_show(reg: Register, tid: str) -> int:
    if tid not in reg.items:
        print(f"no such item: {tid}", file=sys.stderr)
        return 1
    s, e = reg.spans[tid]
    print(reg.text[s:e].rstrip())
    return 0


def cmd_add(reg: Register, args) -> int:
    tid = args.id or reg.next_id()
    if tid in reg.rows or tid in reg.items:
        print(f"id already exists: {tid} (ids are never reused)", file=sys.stderr)
        return 1
    if args.status not in STATUS_SYMBOLS:
        print(f"invalid status {args.status!r}; use one of {STATUS_SYMBOLS}",
              file=sys.stderr)
        return 1
    body = args.body or ""
    if args.body_file:
        body = Path(args.body_file).read_text()
    if not body.strip():
        print("a register item needs a body (--body / --body-file): at "
              "minimum a Trigger line and a Scope line", file=sys.stderr)
        return 1

    text = reg.text
    # Row: insert after the last existing T-row in the status table.
    row_matches = list(re.finditer(r"^\| " + _ID_ALT + r" \|.*$", text, re.M))
    if not row_matches:
        print("cannot locate the status table", file=sys.stderr)
        return 1
    last = row_matches[-1]
    row = (f"| {tid} | {args.summary} | {args.theme} | {args.priority} "
           f"| {args.status} | {args.blocks or '—'} |")
    text = text[:last.end()] + "\n" + row + text[last.end():]

    # Detail: into the matching `## <theme>` group, else a new group at
    # the tail (before the trailing --- if present).
    section = (f"### {tid}. {args.summary}\n\n"
               f"Theme: {args.theme} · Priority: {args.priority} · "
               f"Status: {args.status}"
               + (f" · Blocks/after: {args.blocks}" if args.blocks else "")
               + "\n\n" + body.rstrip() + "\n\n")
    gm = re.search(rf"^## {re.escape(args.theme)}\s*$", text, re.M)
    if gm:
        tail = text[gm.end():]
        nm = re.search(r"^## |^---\s*$", tail, re.M)
        ins = gm.end() + (nm.start() if nm else len(tail))
        text = text[:ins] + section + text[ins:]
    else:
        tm = re.search(r"^---\s*$", text[::-1], re.M)
        if text.rstrip().endswith("---"):
            idx = text.rstrip().rfind("\n---")
            text = text[:idx] + f"\n## {args.theme}\n\n" + section + text[idx:]
        else:
            text = text.rstrip() + f"\n\n## {args.theme}\n\n" + section
    write_with_diff(TRACKING, text, args.dry_run, f"add {tid}")
    if not args.dry_run:
        print(f"filed {tid}; run `register.py check` — and remember the "
              "status table and field lines must not diverge")
    return 0


def cmd_close(reg: Register, args) -> int:
    tid = args.item
    if tid not in reg.items or tid not in reg.spans:
        print(f"no such item in the register: {tid}", file=sys.stderr)
        return 1
    if tid not in reg.rows:
        print(f"{tid} has a detail section but no table row — fix A2 first",
              file=sys.stderr)
        return 1
    it = reg.items[tid]
    date = args.date or datetime.date.today().isoformat()
    version = args.version

    # 1. Resolution entry: full detail text as at closure, stamped.
    entry = (f"## [{version}] {tid} — {it.title} (v{version}, {date})\n\n"
             f"Theme: {it.theme} · closed {version} · {date}\n"
             f"{it.body.rstrip()}\n\n"
             f"Cross-ref: CHANGELOG {version}.\n\n")
    resolved = RESOLVED.read_text()
    first = re.search(r"^## ", resolved, re.M)
    if not first:
        print("RESOLVED.md: cannot find insertion point (no '## ' entry)",
              file=sys.stderr)
        return 1
    new_resolved = resolved[:first.start()] + entry + resolved[first.start():]

    # 2. Register: remove detail section and row in one operation.
    s, e = reg.spans[tid]
    text = reg.text[:s] + reg.text[e:]
    raw_row = reg.rows[tid][3]
    text = text.replace(raw_row + "\n", "", 1)
    # Drop a theme group emptied by the removal.
    text = re.sub(rf"^## {re.escape(it.theme)}\s*\n+(?=(## |---\s*$))", "",
                  text, flags=re.M)

    write_with_diff(RESOLVED, new_resolved, args.dry_run, f"close {tid} (record)")
    write_with_diff(TRACKING, text, args.dry_run, f"close {tid} (register)")
    if not args.dry_run:
        print(f"closed {tid} at v{version}. Remaining by hand: the CHANGELOG "
              f"entry for {version} should cross-reference this closure "
              "(the changelog says what shipped; RESOLVED.md says what was "
              "wrong — they reference, never duplicate).")
    return 0


def cmd_check(reg: Register) -> int:
    class R:
        def __init__(self):
            self.errors = []

        def err(self, s, m):
            self.errors.append(f"[{s}] {m}")

        def warn(self, s, m):
            print(f"WARN [{s}] {m}")

    r = R()
    reg.check(r)
    for e in r.errors:
        print(f"ERROR {e}")
    if r.errors:
        print(f"REGISTER CHECK FAIL: {len(r.errors)} error(s)")
        return 1
    print(f"REGISTER CHECK OK: {len(reg.items)} open item(s)")
    return 0


def main() -> int:
    ap = argparse.ArgumentParser(
        description="Live-register operations (docs/TRACKING.md)",
        epilog="See module docstring for the closure procedure this enforces.")
    sub = ap.add_subparsers(dest="cmd", required=True)
    sub.add_parser("list")
    p = sub.add_parser("show")
    p.add_argument("item")
    p = sub.add_parser("add")
    p.add_argument("--id", help="explicit id (default: next free)")
    p.add_argument("--summary", required=True)
    p.add_argument("--theme", required=True)
    p.add_argument("--priority", required=True, help="P1 (highest) .. P4")
    p.add_argument("--status", default="☐")
    p.add_argument("--blocks", default="")
    p.add_argument("--body")
    p.add_argument("--body-file")
    p.add_argument("--dry-run", action="store_true")
    p = sub.add_parser("close")
    p.add_argument("item")
    p.add_argument("--version", required=True)
    p.add_argument("--date")
    p.add_argument("--dry-run", action="store_true")
    sub.add_parser("check")
    args = ap.parse_args()

    reg = load()
    if args.cmd == "list":
        return cmd_list(reg)
    if args.cmd == "show":
        return cmd_show(reg, args.item)
    if args.cmd == "add":
        return cmd_add(reg, args)
    if args.cmd == "close":
        return cmd_close(reg, args)
    if args.cmd == "check":
        return cmd_check(reg)
    return 1


if __name__ == "__main__":
    sys.exit(main())
