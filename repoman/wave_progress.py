#!/usr/bin/env python3
"""repoman/wave_progress.py — regenerates a wave-tracking document's own
"## 1. Progress at a glance" summary from each wave's own per-item
status table below it.

Ported from xolu's own scripts/wave_progress.py (2026-08-17), which
carries the original design rationale in full. That rationale is
preserved here verbatim where it's still exactly true; only the
xolu-specific bindings changed:

  - DOC (the wave-tracking document) and the register file it
    cross-references are now config-driven (wave_tracking, tracking
    in .repoman.json), not hardcoded paths.
  - SHORT_NAMES and WAVE_THEMES are now config-driven data
    (wave_short_names, wave_themes), not Python dict literals -- both
    default to empty, so a consumer that never sets them sees exactly
    the behaviour of a project with no waves configured yet: no
    summary line to regenerate, nothing to break.
  - The id-format handling (_ID_ALT / _id_num) now reads the same
    id_prefix / id_separator / legacy_id_prefix / legacy_id_separator
    keys register.py reads, generalizing the same forward-only
    mid-project prefix-migration need register.py was built for --
    duplicated here rather than imported, preserving this file's own
    original design choice to remain a standalone script with no
    cross-script dependency for one shared regex fragment.

Why this exists (xolu's own original rationale, unchanged): a
hand-maintained progress summary drifts from the data it summarises --
it did, in xolu, silently, for a stretch (see that project's own
T-113). This script makes the summary a pure function of the per-wave
tables, which stay hand-maintained (their per-item Notes are genuinely
bespoke prose, not something to generate) -- only the roll-up is
mechanical.

Per-wave percentage: a table row's Status column is one of the three
project-wide markers (✓ done, ◐ partial, ☐ not started). A row counts
as 1.0 item done, 0.5, or 0.0 respectively; percentage is that sum
over the row count. A "~" prefix marks any wave containing a ◐ row.

Each wave also gets a "debt:" subtitle line, listing open register
items whose theme is mapped to that wave via wave_themes -- open
technical debt that shares a wave's subject matter but was never one
of that wave's own planned items. Items already counted as SOME
wave's own item (per that wave's own table column 4, not a theme
guess) are excluded even if their theme happens to match a different
wave. Waves absent from wave_themes simply show no debt line.

A wave also gets a "blockers:" subtitle when any of its own items or
its debt is waiting on a currently-open prerequisite (an "After: X"
field in the register where X is itself still open).

Deliberately NOT preserved: hand-composed trailing annotations. Those
are narrative, not data, and belong in each wave's own detail section
below the summary (untouched by this script).

Requires the wave-tracking document to carry a "## 1. Progress at a
glance" section with a fenced code block (```...```) and, somewhere in
that document, a line of the form "Overall by item count: N of M items
≈ **P%**" -- a documented structural convention, the same way
register.py requires "## <theme>" groupings, not itself a config key.

Usage:
    python3 repoman/wave_progress.py           # regenerate in place
    python3 repoman/wave_progress.py --check   # exit 1 if it would change, don't write
    python3 repoman/wave_progress.py --show    # print to stdout, touch nothing
    python3 repoman/wave_progress.py --html PATH        # standalone HTML file
    python3 repoman/wave_progress.py --hide 6            # persist wave 6 as hidden
    python3 repoman/wave_progress.py --unhide 6          # persist wave 6 as visible again
    python3 repoman/wave_progress.py --show --include-hidden   # render everything anyway
"""
import re
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import config as _config

_ROOT, _CFG = _config.load()
DOC = _ROOT / _CFG["wave_tracking"]
TRACKING = _ROOT / _CFG["tracking"]
SHORT_NAMES = _CFG.get("wave_short_names", {})
WAVE_THEMES = _CFG.get("wave_themes", {})
WAVE_VISIBILITY = _CFG.get("wave_visibility", {})


def is_visible(wave_id: str) -> bool:
    """Absent entry = visible. This is the ONLY place either renderer
    consults visibility state -- render_table() and render_html() both
    call this rather than each keeping their own notion of what's
    shown, so ASCII and HTML can never disagree about which waves are
    currently hidden."""
    return WAVE_VISIBILITY.get(wave_id, True)

_P = _CFG["id_prefix"]
_SEP = _CFG.get("id_separator", "-")
_LEGACY_P = _CFG.get("legacy_id_prefix", "")
_LEGACY_SEP = _CFG.get("legacy_id_separator", "-")

if _LEGACY_P:
    _ID_ALT = (r"(?:" + re.escape(_P) + re.escape(_SEP) + r"|"
               + re.escape(_LEGACY_P) + re.escape(_LEGACY_SEP) + r")\d+")
else:
    _ID_ALT = re.escape(_P) + re.escape(_SEP) + r"\d+"


def _id_num(tid: str) -> int:
    """The numeric portion of an id, primary or legacy shape. Raises
    ValueError on anything matching neither configured shape, rather
    than silently returning a wrong number."""
    primary = _P + _SEP
    if tid.startswith(primary):
        return int(tid[len(primary):])
    if _LEGACY_P:
        legacy = _LEGACY_P + _LEGACY_SEP
        if tid.startswith(legacy):
            return int(tid[len(legacy):])
    raise ValueError(f"unrecognized id format: {tid!r}")


STATUS_WEIGHT = {"✓": 1.0, "◐": 0.5, "☐": 0.0}

HEADING_RE = re.compile(r"^### Wave (\S+) — [^(]+\(", re.M)
ROW_RE = re.compile(r"^\|\s*\d+\s*\|.*\|\s*([✓◐☐])\s*\|", re.M)

_PREFIX_ALT = (r"(?:" + re.escape(_P) + re.escape(_SEP) + r"|"
               + re.escape(_LEGACY_P) + re.escape(_LEGACY_SEP) + r")") \
              if _LEGACY_P else re.escape(_P) + re.escape(_SEP)
# Captures the matched PREFIX itself (group 1, "T-" or "XOT", whichever
# side of "through" it is -- a range is never written mixing prefixes,
# e.g. nobody writes "XOT07 through T-13"), then lo/hi digits.
RANGE_RE = re.compile(r"(" + _PREFIX_ALT + r")(\d+)\s+through\s+(?:"
                       + _PREFIX_ALT + r")(\d+)")
TNUM_RE = re.compile(r"(" + _ID_ALT + r")")
# Column 4 of a wave's own item table: the register item(s) that ARE
# this wave's own numbered items. Scoped to table ROWS specifically
# (line starts with "| <digits> |"), not prose -- a file-wide id
# regex can sweep up a bare mention inside an explanatory footnote
# that explicitly says an id is NOT one of this wave's items (found
# in xolu's own T-68 footnote under wave 6). Only the table's own
# column 4 is authoritative for "this id IS one of this wave's items."
ITEM_COL4_RE = re.compile(r"^\|\s*\d+\s*\|[^|]*\|\s*[✓◐☐]\s*\|([^|]*)\|", re.M)


def all_wave_tnums(text: str) -> set[str]:
    """Every id that is literally one of SOME wave's own numbered
    items, per that wave's own table column 4 -- not just literal id
    mentions (which also silently drop the middle of a "X through Y"
    range notation) but the expanded range too. Used as a global
    exclusion set for the debt calculation: a theme match alone isn't
    enough -- an id already counted as one wave's own item must never
    double up as a different wave's debt just because its theme
    happens to match."""
    result = set()
    for col4 in ITEM_COL4_RE.findall(text):
        for prefix, lo, hi in RANGE_RE.findall(col4):
            for n in range(int(lo), int(hi) + 1):
                result.add(f"{prefix}{n:02d}" if n < 100 else f"{prefix}{n}")
        for n in TNUM_RE.findall(col4):
            result.add(n)
    return result


def parse_waves(text: str) -> list[dict]:
    headings = list(HEADING_RE.finditer(text))
    waves = []
    for i, m in enumerate(headings):
        wave_id = m.group(1)
        start = m.end()
        end = headings[i + 1].start() if i + 1 < len(headings) else len(text)
        section = text[start:end]
        statuses = ROW_RE.findall(section)
        if not statuses:
            print(f"WARNING: Wave {wave_id} heading found but no table rows parsed "
                  f"-- skipped, check the table format", file=sys.stderr)
            continue
        weights = [STATUS_WEIGHT[s] for s in statuses]
        done_equiv = sum(weights)
        total = len(weights)
        has_partial = "◐" in statuses
        waves.append({
            "id": wave_id, "done_equiv": done_equiv, "total": total,
            "has_partial": has_partial, "item_tnums": all_wave_tnums(section),
        })
    return waves


def debt_by_wave(full_text: str) -> dict[str, list[str]]:
    """Open register items, grouped by wave via wave_themes, excluding
    anything that's already SOME wave's own item. Themes with no wave
    mapping are simply absent from the result."""
    if not TRACKING.is_file():
        return {}
    text = TRACKING.read_text(encoding="utf-8")
    already_a_wave_item = all_wave_tnums(full_text)
    theme_to_wave = {}
    for wave_id, themes in WAVE_THEMES.items():
        for t in themes:
            theme_to_wave[t] = wave_id
    row_t_re = re.compile(r"^\|\s*(" + _ID_ALT + r")\s*\|[^|]*\|\s*([a-z0-9-]+)\s*\|", re.M)
    result: dict[str, list[str]] = {}
    for tnum, theme in row_t_re.findall(text):
        if tnum in already_a_wave_item:
            continue
        wave_id = theme_to_wave.get(theme)
        if wave_id:
            result.setdefault(wave_id, []).append(tnum)
    return result


def bar(pct: float) -> str:
    filled = round(pct / 5)
    return "█" * filled + "░" * (20 - filled)


def fmt_item_count(done_equiv: float, total: int) -> str:
    d = f"{done_equiv:g}"
    return f"{d}/{total} items"


AFTER_RE = re.compile(r"After:\s*(" + _ID_ALT + r")")
TRACKING_ROW_RE = re.compile(
    r"^\| (" + _ID_ALT + r") \| ([^|]*) \| ([a-z0-9-]+) \| (P\d) \| ([✓◐☐]) \| ([^|]*) \|", re.M)


def blockers_by_wave(full_text: str, waves: list[dict]) -> dict[str, list[tuple[str, list[str]]]]:
    """For each wave's own items plus its debt items, checks their
    "After: X" field and keeps only references that resolve to a
    CURRENTLY OPEN item. Returns wave_id -> [(blocker, [items it
    blocks in this wave]), ...], grouped by blocker. A blocker can be
    outside the wave entirely -- that cross-wave case is the useful
    signal, not filtered out."""
    if not TRACKING.is_file():
        return {}
    tracking_text = TRACKING.read_text(encoding="utf-8")
    rows = TRACKING_ROW_RE.findall(tracking_text)
    open_tnums = {tnum for tnum, *_ in rows}
    after_field = {}
    for tnum, _s, _th, _p, _st, blocks in rows:
        m = AFTER_RE.search(blocks)
        if m and m.group(1) in open_tnums:
            after_field[tnum] = m.group(1)

    debt = debt_by_wave(full_text)
    result: dict[str, list[tuple[str, list[str]]]] = {}
    for w in waves:
        wave_scope = set(w["item_tnums"]) | set(debt.get(w["id"], []))
        by_blocker: dict[str, list[str]] = {}
        for item in sorted(wave_scope, key=_id_num):
            blocker = after_field.get(item)
            if blocker:
                by_blocker.setdefault(blocker, []).append(item)
        if by_blocker:
            result[w["id"]] = sorted(by_blocker.items())
    return result


def render_table(waves: list[dict], full_text: str) -> str:
    lines = []
    debt = debt_by_wave(full_text)
    blockers = blockers_by_wave(full_text, waves)
    label_w = max(len(f"Wave {w['id']}") for w in waves) + 2
    for w in waves:
        pct = 100 * w["done_equiv"] / w["total"] if w["total"] else 0
        pct_s = f"~{round(pct)}%" if w["has_partial"] else f"{round(pct)}%"
        label = f"Wave {w['id']}".ljust(label_w)
        name = SHORT_NAMES.get(w["id"], f"(unnamed wave {w['id']})")
        lines.append(
            f"{label}{name:<26}  {bar(pct)}  {pct_s:>5}  "
            f"({fmt_item_count(w['done_equiv'], w['total'])})"
        )
        indent = " " * (label_w + 26 + 2)
        wave_debt = debt.get(w["id"])
        if wave_debt:
            lines.append(f"{indent}debt: {', '.join(wave_debt)}")
        wave_blockers = blockers.get(w["id"])
        if wave_blockers:
            parts = [f"{blocker} blocks {', '.join(blocked)}"
                     for blocker, blocked in wave_blockers]
            lines.append(f"{indent}blockers: {'; '.join(parts)}")
    return "\n".join(lines)


def render_overall(waves: list[dict]) -> str:
    total_done = sum(w["done_equiv"] for w in waves)
    total_items = sum(w["total"] for w in waves)
    pct = round(100 * total_done / total_items) if total_items else 0
    done_s = f"{total_done:g}"
    return f"Overall by item count: {done_s} of {total_items} items \u2248 **{pct}%**"


_HTML_HEAD = """<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{title}</title>
<style>
:root {
  --bg: #ffffff; --surface: #f4f3f1; --text: #0b0b0b; --text-2: #52514e; --text-3: #898781;
  --border: #e1e0d9;
  --success: #0ca30c;
  --warn-track: #fab219; --warn-bg: #faeeda; --warn-text: #854f0b;
  --acc-track: #2a78d6; --acc-bg: #e6f1fb; --acc-text: #0c447c;
  --muted-track: #d3d1c7;
}
@media (prefers-color-scheme: dark) {
  :root {
    --bg: #16161a; --surface: #232327; --text: #f0efec; --text-2: #c3c2b7; --text-3: #898781;
    --border: #333338;
    --success: #2ed92e;
    --warn-track: #fab219; --warn-bg: #4a3417; --warn-text: #f5c876;
    --acc-track: #4a95e6; --acc-bg: #163a5c; --acc-text: #a9d1f7;
    --muted-track: #3a3a3e;
  }
}
* { box-sizing: border-box; }
body {
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif;
  background: var(--bg); color: var(--text); margin: 0; padding: 2rem 1.25rem;
}
.wrap { max-width: 720px; margin: 0 auto; }
h1 { font-size: 15px; font-weight: 600; margin: 0 0 1.25rem; color: var(--text-2); }
.row { display: grid; grid-template-columns: 160px 1fr; gap: 10px; padding: 5px 0; }
.row.sub { padding: 0 0 5px; }
.wname { font-size: 13px; font-weight: 600; text-align: right; }
.sublabel { font-size: 11px; color: var(--text-3); text-align: right; padding-top: 1px; }
.barwrap { display: flex; align-items: center; gap: 8px; }
.track { flex: 1; height: 7px; border-radius: 4px; background: var(--surface); overflow: hidden; }
.fill { height: 100%; border-radius: 4px; }
.fill.success { background: var(--success); }
.fill.warn { background: var(--warn-track); }
.fill.muted { background: var(--muted-track); }
.fill.acc { background: var(--acc-track); }
.count { font-size: 11px; color: var(--text-2); white-space: nowrap; }
.pills { display: flex; gap: 6px; flex-wrap: wrap; }
.pill { font-size: 11px; padding: 1px 7px; border-radius: 6px; white-space: nowrap; }
.pill.debt { background: var(--warn-bg); color: var(--warn-text); }
.pill.blocker { background: var(--acc-bg); color: var(--acc-text); }
.overall { border-top: 1px solid var(--border); margin-top: 6px; padding-top: 10px; }
.overall .wname { font-size: 14px; }
.overall .count { font-weight: 600; }
</style>
</head>
<body>
<div class="wrap">
"""

_HTML_TAIL = """</div>
</body>
</html>
"""


def _html_escape(s: str) -> str:
    return (s.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;")
             .replace('"', "&quot;"))


def render_html(waves: list[dict], full_text: str, title: str = "wave progress") -> str:
    """Standalone HTML rendering of the same wave/debt/blocker data
    render_table() already computes for the terminal -- genuinely
    portable: real hex colours (light + dark via prefers-color-scheme),
    no dependency on any host page's own CSS variables, opens
    correctly in any browser with the file handed to it directly, not
    just inside a chat context that happens to define the tokens this
    was first prototyped against. Layout matches this project's own
    established terminal convention exactly, not a redesign: wave name
    (or a "debt"/"blocked by" sublabel) right-aligned in a fixed left
    column, bar/pills left-aligned in the column beside it, starting
    at the same point the bar itself starts."""
    debt = debt_by_wave(full_text)
    blockers = blockers_by_wave(full_text, waves)
    parts = [_HTML_HEAD.replace("{title}", _html_escape(title))]
    parts.append(f'<h1>{_html_escape(title)}</h1>\n')

    def fill_class(pct: float, has_partial: bool) -> str:
        if pct <= 0:
            return "muted"
        if pct >= 100 and not has_partial:
            return "success"
        return "warn"

    for w in waves:
        pct = 100 * w["done_equiv"] / w["total"] if w["total"] else 0
        pct_s = f"~{round(pct)}%" if w["has_partial"] else f"{round(pct)}%"
        name = SHORT_NAMES.get(w["id"], f"(unnamed wave {w['id']})")
        cls = fill_class(pct, w["has_partial"])
        parts.append(
            f'<div class="row"><span class="wname">Wave {_html_escape(w["id"])} — '
            f'{_html_escape(name)}</span><div class="barwrap">'
            f'<div class="track"><div class="fill {cls}" style="width:{min(pct,100):.4g}%"></div></div>'
            f'<span class="count">{pct_s} · {fmt_item_count(w["done_equiv"], w["total"])}</span>'
            f'</div></div>\n'
        )
        wave_debt = debt.get(w["id"])
        if wave_debt:
            pills = "".join(f'<span class="pill debt">{_html_escape(d)}</span>' for d in wave_debt)
            parts.append(f'<div class="row sub"><span class="sublabel">debt</span>'
                          f'<div class="pills">{pills}</div></div>\n')
        wave_blockers = blockers.get(w["id"])
        if wave_blockers:
            pills = "".join(
                f'<span class="pill blocker">{_html_escape(b)} → '
                f'{_html_escape(", ".join(blocked))}</span>'
                for b, blocked in wave_blockers
            )
            parts.append(f'<div class="row sub"><span class="sublabel">blocked by</span>'
                          f'<div class="pills">{pills}</div></div>\n')

    total_done = sum(w["done_equiv"] for w in waves)
    total_items = sum(w["total"] for w in waves)
    overall_pct = 100 * total_done / total_items if total_items else 0
    parts.append(
        f'<div class="row overall"><span class="wname">Overall</span><div class="barwrap">'
        f'<div class="track"><div class="fill acc" style="width:{min(overall_pct,100):.4g}%"></div></div>'
        f'<span class="count">{round(overall_pct)}% · {fmt_item_count(total_done, total_items)}</span>'
        f'</div></div>\n'
    )
    parts.append(_HTML_TAIL)
    return "".join(parts)


def _set_visibility(wave_id: str, visible: bool) -> int:
    """--hide/--unhide: a single, distinct mutation, same shape as
    guards.py's own `record` command -- persists to .repoman.json via
    config.save_key(), confirms, exits. Never combined with a render
    operation in the same invocation, so it's always obvious from the
    command alone whether state changed or something was only
    displayed."""
    doc_text = DOC.read_text(encoding="utf-8") if DOC.is_file() else ""
    known = {w["id"] for w in parse_waves(doc_text)} if doc_text else set()
    if known and wave_id not in known:
        print(f"warning: {wave_id!r} is not a wave id found in {DOC.name} "
              f"(known: {', '.join(sorted(known))}) -- setting anyway, in "
              f"case the wave document hasn't been regenerated yet",
              file=sys.stderr)
    visibility = dict(WAVE_VISIBILITY)
    visibility[wave_id] = visible
    _config.save_key(_ROOT, "wave_visibility", visibility)
    print(f"wave {wave_id}: {'visible' if visible else 'hidden'} "
          f"(persisted to .repoman.json)")
    return 0


def main() -> int:
    if "--hide" in sys.argv or "--unhide" in sys.argv:
        flag = "--hide" if "--hide" in sys.argv else "--unhide"
        i = sys.argv.index(flag)
        if i + 1 >= len(sys.argv):
            print(f"{flag} requires a wave id argument", file=sys.stderr)
            return 1
        return _set_visibility(sys.argv[i + 1], visible=(flag == "--unhide"))

    check = "--check" in sys.argv
    show = "--show" in sys.argv
    include_hidden = "--include-hidden" in sys.argv
    html_out = None
    if "--html" in sys.argv:
        i = sys.argv.index("--html")
        if i + 1 >= len(sys.argv):
            print("--html requires a path argument", file=sys.stderr)
            return 1
        html_out = sys.argv[i + 1]
    if not DOC.is_file():
        print(f"no wave-tracking document at {DOC} -- nothing to do "
              f"(set wave_tracking in .repoman.json if waves are wanted)",
              file=sys.stderr)
        return 1
    text = DOC.read_text(encoding="utf-8")
    all_waves = parse_waves(text)
    if not all_waves:
        print("no waves parsed -- aborting, not touching the file", file=sys.stderr)
        return 1
    # Overall always reflects every wave, regardless of what's currently
    # displayed -- visibility is a display concern (per is_visible's own
    # docstring), and hiding a wave from view doesn't mean its work
    # stops counting toward the real total.
    visible_waves = all_waves if include_hidden else [
        w for w in all_waves if is_visible(w["id"])]
    if not visible_waves:
        print("every wave is currently hidden -- pass --include-hidden to "
              "render anyway, or --unhide <id> to bring one back",
              file=sys.stderr)
        return 1

    new_table = render_table(visible_waves, text)
    new_overall = render_overall(all_waves)

    if html_out:
        title = _CFG.get("wave_html_title", "wave progress")
        html = render_html(visible_waves, text, title=title)
        Path(html_out).write_text(html, encoding="utf-8")
        hidden_n = len(all_waves) - len(visible_waves)
        note = f", {hidden_n} hidden" if hidden_n else ""
        print(f"wave_progress: wrote {html_out} ({len(visible_waves)} waves{note})")
        if not show and not check:
            return 0

    if show:
        print(new_table)
        print()
        print(new_overall)
        return 0

    table_re = re.compile(r"(## 1\. Progress at a glance\n.*?```\n).*?(\n```\n)", re.S)
    m = table_re.search(text)
    if not m:
        print("could not find the '## 1. Progress at a glance' fenced block", file=sys.stderr)
        return 1
    new_text = text[:m.start()] + m.group(1) + new_table + m.group(2) + text[m.end():]

    overall_re = re.compile(r"Overall by item count: [\d.]+ of \d+ items \u2248 \*\*\d+%\*\*")
    if not overall_re.search(new_text):
        print("could not find the 'Overall by item count' line", file=sys.stderr)
        return 1
    new_text = overall_re.sub(new_overall, new_text)

    if new_text == text:
        print("wave_progress: already up to date")
        return 0

    if check:
        print("wave_progress: wave-tracking document is stale -- run without --check to regenerate")
        return 1

    DOC.write_text(new_text, encoding="utf-8")
    hidden_n = len(all_waves) - len(visible_waves)
    note = f", {hidden_n} hidden" if hidden_n else ""
    print(f"wave_progress: regenerated ({len(visible_waves)} waves{note})")
    return 0


if __name__ == "__main__":
    sys.exit(main())
