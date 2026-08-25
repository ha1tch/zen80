#!/usr/bin/env python3
"""repoman/add_wave.py -- adds a new wave to a project's staged-work
programme deterministically: the wave-tracking document's own
per-item table (that wave_progress.py reads) and the wave-plan
document's short pointer paragraph, with the wave number and item
numbers computed from the actual current state of both documents --
not supplied by the caller and not guessed.

Ported from xolu's own scripts/add_wave.py (2026-08-17), which carries
the original design rationale in full, reproduced below with only the
xolu-specific bindings changed to config-driven equivalents (see
wave_progress.py's own header for the parallel list). Two differences
worth calling out specifically, not just parameterization:

  - The tracking-document insertion point no longer depends on a
    hardcoded section marker (xolu's own original used a literal
    "## 3." anchor -- fragile, and specific to that document's own
    current numbering). This now finds the LAST existing "### Wave N"
    heading's own section end and inserts immediately after it, the
    same robust pattern this file's own insert_plan_paragraph already
    used for the plan document -- one technique, applied consistently,
    rather than two different techniques for what is the same problem
    in two documents.
  - SHORT_NAMES is no longer maintained by injecting a new dict-literal
    line into a Python source file at runtime. It's config data
    (wave_short_names in .repoman.json) now, so this writes it there
    via config.save_key -- mechanical JSON, not source-code surgery.

Why this exists (xolu's own original rationale, unchanged): adding a
wave by hand once nearly collided with an EXISTING soft reservation --
the tracking document's own prose already reserved a wave number for
unrelated future work in a sentence, not a formal heading, which a
naive "highest heading + 1" scan would have missed entirely. This
script scans for BOTH: formal headings and any other "wave N" mention
anywhere in either document, so a soft reservation like that is caught
and refused rather than silently overwritten.

Item numbers are global and sequential across the whole programme,
never reused once assigned -- this script continues from the highest
"| NN |" row it finds in any existing wave table.

What this script does NOT do: write the load-bearing prose. The plan
document's own paragraph explaining why a wave exists, what it depends
on, and how it's sequenced is exactly the kind of judgement call a
human (or an LLM, thinking about it deliberately) writes each time --
this script places it correctly and consistently, it doesn't compose
it. Pass that text in with --plan-note.

Usage:
    python3 repoman/add_wave.py \\
        --name "some staged workstream" \\
        --ideal-days 2 \\
        --items-json '[{"summary": "...", "register_item": "T-1"}]' \\
        --plan-note "Why this wave exists, what it depends on..." \\
        [--wave-number 12]   # override the computed number; still checked for collision
        [--dry-run]          # print what would change, touch nothing
"""
import argparse
import json
import re
import subprocess
import sys
from datetime import date
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import config as _config

_ROOT, _CFG = _config.load()
ROOT = _ROOT
TRACKING = _ROOT / _CFG["wave_tracking"]
PLAN = _ROOT / _CFG["wave_plan"]

WAVE_HEADING_RE = re.compile(r"^### Wave (\S+) —", re.M)
WAVE_MENTION_RE = re.compile(r"\bwave (\d+)\b", re.I)
ITEM_ROW_RE = re.compile(r"^\|\s*(\d+)\s*\|", re.M)


def _require_docs() -> None:
    missing = [str(p) for p in (TRACKING, PLAN) if not p.is_file()]
    if missing:
        raise SystemExit(
            "wave-tracking and/or wave-plan document not found: "
            + ", ".join(missing)
            + " -- set wave_tracking/wave_plan in .repoman.json and create "
              "them (see repoman/README.md for the expected shape) before "
              "adding a wave."
        )


def existing_wave_numbers(text: str) -> set[int]:
    """Every integer wave number already in use OR reserved -- formal
    headings AND prose mentions like "plausibly wave 11". Alphabetic
    sub-waves (e.g. "9b") are excluded from this integer set
    deliberately; they share a numeric wave's own number by design,
    not a new one."""
    nums = set()
    for m in WAVE_HEADING_RE.finditer(text):
        digits = re.match(r"\d+", m.group(1))
        if digits:
            nums.add(int(digits.group()))
    for m in WAVE_MENTION_RE.finditer(text):
        nums.add(int(m.group(1)))
    return nums


def next_wave_number(requested: int | None) -> int:
    tracking_text = TRACKING.read_text(encoding="utf-8")
    plan_text = PLAN.read_text(encoding="utf-8")
    taken = existing_wave_numbers(tracking_text) | existing_wave_numbers(plan_text)

    if requested is not None:
        if requested in taken:
            raise SystemExit(
                f"--wave-number {requested} collides with an existing heading or "
                f"prose reservation in {TRACKING.name}/{PLAN.name}. "
                f"Taken/reserved: {sorted(taken)}"
            )
        return requested

    candidate = max(taken, default=0) + 1
    while candidate in taken:
        candidate += 1
    return candidate


def next_item_number(n_items: int) -> int:
    text = TRACKING.read_text(encoding="utf-8")
    existing = [int(n) for n in ITEM_ROW_RE.findall(text)]
    return (max(existing, default=0) + 1) if existing else 1


def build_tracking_section(wave_num: int, name: str, ideal_days: float,
                            items: list[dict], start_item: int) -> str:
    today = date.today().isoformat()
    lines = [
        f"### Wave {wave_num} — {name} ({len(items)} item{'s' if len(items) != 1 else ''}, "
        f"ideal {ideal_days}d, added {today})",
        "",
        "| # | Summary | Status | Register item |",
        "|---|---|---|---|",
    ]
    for i, item in enumerate(items):
        item_num = start_item + i
        reg = item.get("register_item", "not yet filed")
        lines.append(f"| {item_num} | {item['summary']} | ☐ | {reg} |")
    lines.append("")
    lines.append(f"**Wave {wave_num}: 0/{len(items)}, not started.**")
    lines.append("")
    return "\n".join(lines)


def build_plan_paragraph(wave_num: int, name: str, ideal_days: float, plan_note: str) -> str:
    today = date.today().isoformat()
    return (f"**Wave {wave_num} — {name} (≈ {ideal_days}d, added {today}).** "
            f"{plan_note}\n")


def _section_end_after_last_heading(text: str, heading_re: re.Pattern,
                                     fallback_re: re.Pattern | None = None) -> int | None:
    """Find where the LAST match's own "section" ends -- the next
    blank line after it for a paragraph-style match, or None if
    nothing matched at all (caller decides the fallback). Shared by
    both insertion functions below so the tracking document and the
    plan document use one technique for what is the same problem in
    two documents, rather than two different techniques."""
    headings = list(heading_re.finditer(text))
    if not headings and fallback_re is not None:
        headings = list(fallback_re.finditer(text))
    if not headings:
        return None
    last = headings[-1]
    para_end = text.find("\n\n", last.end())
    return (para_end + 2) if para_end != -1 else len(text)


def insert_tracking_section(section: str) -> None:
    """Inserts after the LAST existing "### Wave N" heading's own
    section (the next "### ", "## ", or "---" separator, whichever
    comes first) -- or at the end of the document if this is the very
    first wave. No hardcoded section-number anchor: this works
    regardless of how the rest of the document is structured or
    numbered, which xolu's own original ("## 3.", a literal marker
    for whatever section happened to follow the wave list there) did
    not.

    A trailing "---" separator is added after the new section ONLY
    when none exists yet at the insertion point. When a PREVIOUS
    insertion's own trailing "---" is what's found instead, it is
    reused, not duplicated -- the new section is inserted before it,
    with a blank line restored ahead of it (`section` itself always
    ends in exactly one newline; one more is needed for correct
    blank-line spacing). Getting this reuse-vs-add distinction wrong
    was a real bug, not a hypothetical one: this function's own first
    version here always added a trailing separator regardless, which
    produces a duplicate on every insertion after the first -- found
    while backporting this exact function into xolu's own copy
    (xolu's original had the separator-adding behaviour correct, via
    a different, more fragile mechanism this port was replacing;
    losing the correct behaviour while fixing the fragility was the
    bug). Verified directly: four sequential wave insertions against a
    realistic fixture, checked for exactly one separator between each
    pair and no duplication, before trusting this version."""
    text = TRACKING.read_text(encoding="utf-8")
    headings = list(WAVE_HEADING_RE.finditer(text))
    if headings:
        last = headings[-1]
        tail = text[last.end():]
        nm = re.search(r"^(### |## |---\s*$)", tail, re.M)
        if nm and nm.group(1).strip() == "---":
            insertion_point = last.end() + nm.start()
            new_text = text[:insertion_point] + section + "\n" + text[insertion_point:]
        elif nm:
            insertion_point = last.end() + nm.start()
            new_text = text[:insertion_point] + section + "\n---\n\n" + text[insertion_point:]
        else:
            # Nothing at all follows the last wave's own content (it
            # is the literal end of the document, or of the relevant
            # region) -- unlike the other two branches, there is no
            # PRE-EXISTING blank line to rely on here, since there is
            # no following heading or separator whose own leading
            # spacing this insertion can inherit. Must add one
            # explicitly. Missing this was the second real bug found
            # while testing this function: the first fix for the
            # duplicate-separator bug above still landed a brand-new
            # wave's own heading directly against the previous wave's
            # closing line with no blank line between them, in exactly
            # this branch specifically -- caught by testing a second
            # real insertion against a fixture with no document
            # structure following the first wave at all, not assumed
            # correct from the first fix's own passing test alone.
            insertion_point = last.end() + len(tail)
            new_text = (text[:insertion_point].rstrip() + "\n\n" + section
                        + "\n---\n\n" + text[insertion_point:])
    else:
        # First wave in this document: append at the end.
        new_text = text.rstrip() + "\n\n" + section
    TRACKING.write_text(new_text, encoding="utf-8")


def insert_plan_paragraph(paragraph: str) -> None:
    text = PLAN.read_text(encoding="utf-8")
    headings = list(WAVE_HEADING_RE.finditer(text)) or list(
        re.finditer(r"^\*\*Wave \S+ —", text, re.M))
    if headings:
        last = headings[-1]
        para_end = text.find("\n\n", last.end())
        insertion_point = (para_end + 2) if para_end != -1 else len(text)
    else:
        insertion_point = len(text)
    new_text = text[:insertion_point] + "\n" + paragraph + "\n" + text[insertion_point:]
    PLAN.write_text(new_text, encoding="utf-8")


def insert_short_name(wave_num: int, name: str) -> None:
    """Adds this wave's entry to wave_short_names in .repoman.json.
    Without this, wave_progress.py renders "(unnamed wave N)" -- a
    real gap found by running xolu's own original tool end to end the
    first time, not a hypothetical: short display names are
    hand-curated by design (a wave's short form isn't mechanically
    derivable from its full name), but leaving that as a separate
    manual follow-up undermines the determinism this tool exists for.
    Guarded: refuses if the key already exists rather than silently
    overwriting."""
    key = str(wave_num)
    names = dict(_CFG.get("wave_short_names", {}))
    if key in names:
        print(f"wave_short_names already has an entry for {key!r} -- not touching it",
              file=sys.stderr)
        return
    names[key] = name
    _config.save_key(ROOT, "wave_short_names", names)


def insert_default_visibility(wave_num: int) -> None:
    """Adds an explicit visible=True entry to wave_visibility in
    .repoman.json for this wave. wave_progress.py's own is_visible()
    already treats an ABSENT entry as visible, so this call is not
    strictly required for correctness -- it exists so wave_visibility
    reads as a complete, honest record of every wave's own status at a
    glance (matching wave_short_names's own precedent: explicitly
    written for every wave, not left to a fallback a reader would need
    to know about). Guarded the same way: refuses if the key already
    exists, since a wave that was deliberately hidden before this call
    somehow ran should never be silently un-hidden by it."""
    key = str(wave_num)
    visibility = dict(_CFG.get("wave_visibility", {}))
    if key in visibility:
        return
    visibility[key] = True
    _config.save_key(ROOT, "wave_visibility", visibility)


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__,
                                  formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--name", required=True)
    ap.add_argument("--ideal-days", type=float, required=True)
    ap.add_argument("--items-json", required=True,
                     help='JSON list: [{"summary": "...", "register_item": "T-1"}]')
    ap.add_argument("--plan-note", required=True,
                     help="Rationale paragraph for the wave-plan document -- written "
                          "deliberately, not generated.")
    ap.add_argument("--wave-number", type=int, default=None,
                     help="Override the computed wave number. Still checked for collision.")
    ap.add_argument("--dry-run", action="store_true")
    args = ap.parse_args()

    _require_docs()

    try:
        items = json.loads(args.items_json)
    except json.JSONDecodeError as e:
        raise SystemExit(f"--items-json is not valid JSON: {e}")
    if not items or not isinstance(items, list):
        raise SystemExit("--items-json must be a non-empty JSON list")
    for item in items:
        if "summary" not in item:
            raise SystemExit(f"item missing 'summary': {item}")

    wave_num = next_wave_number(args.wave_number)
    start_item = next_item_number(len(items))
    end_item = start_item + len(items) - 1

    tracking_section = build_tracking_section(
        wave_num, args.name, args.ideal_days, items, start_item)
    plan_paragraph = build_plan_paragraph(
        wave_num, args.name, args.ideal_days, args.plan_note)

    print(f"Wave number: {wave_num}  (computed; checked against headings + prose reservations)")
    print(f"Item numbers: {start_item}"
          + (f"-{end_item}" if end_item != start_item else "") + "\n")
    print(f"--- {TRACKING.name} section ---")
    print(tracking_section)
    print(f"--- {PLAN.name} paragraph ---")
    print(plan_paragraph)

    if args.dry_run:
        print("(--dry-run: nothing written)")
        return 0

    insert_tracking_section(tracking_section)
    insert_plan_paragraph(plan_paragraph)
    insert_short_name(wave_num, args.name)
    insert_default_visibility(wave_num)

    rc = subprocess.run([sys.executable, str(Path(__file__).resolve().parent
                                              / "wave_progress.py")]).returncode
    if rc != 0:
        print(f"wave_progress.py regeneration failed -- check {TRACKING.name}'s "
              "own fenced block by hand", file=sys.stderr)
        return rc

    print(f"\nadd_wave: Wave {wave_num} added, items {start_item}-{end_item}, "
          f"bars regenerated.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
