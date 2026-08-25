#!/usr/bin/env python3
"""repoman/config.py — repository root discovery and configuration.

A repository opts in by carrying `.repoman.json` at its root (an empty
object is a valid opt-in: every key has a default). Root discovery
walks upward from the current directory for `.repoman.json`, then for
`.git`, else uses the current directory.

Defaults encode a documented set of repository conventions (tracking
register, resolution record, known-issues/dormant-guards documents,
plain VERSION file); any of them can be overridden per repository.
"""

import json
from pathlib import Path

DEFAULTS = {
    "id_prefix": "T",
    # id_separator: the character(s) between prefix and digits for
    # NEW ids (what next_id() generates). Default "-" reproduces this
    # project's original, only-ever-tested behaviour exactly for any
    # consumer that doesn't set this key.
    "id_separator": "-",
    # legacy_id_prefix / legacy_id_separator: for a project that
    # migrated id shape mid-project (a real xolu need, not
    # hypothetical -- T-1..T-163 permanently frozen in "T-NNN" shape,
    # T-164 onward forward-only in a new "XOTNNN" shape). Empty
    # legacy_id_prefix (the default) means single-format behaviour,
    # byte-identical to before this key existed -- these two keys are
    # additive and change nothing for a consumer that never sets them.
    "legacy_id_prefix": "",
    "legacy_id_separator": "-",
    "tracking": "docs/TRACKING.md",
    "resolved": "docs/RESOLVED.md",
    "known_issues": "docs/KNOWN_ISSUES.md",
    # guard_id_prefix: full prefix (including separator) for dormant-
    # guard ids, e.g. "G-" -> "G-13". A single string is sufficient
    # generality here -- guards have no mid-project migration need the
    # way register ids sometimes do.
    "guard_id_prefix": "G-",
    "changelog": "CHANGELOG.md",
    "version_file": "VERSION",
    # Extra files that must carry the version; each entry:
    #   {"file": path, "match": regex-with-one-capture-group}
    "version_targets": [],
    # Staged-work ("wave") tracking -- optional module, config keys
    # exist with safe empty/generic defaults so a consumer that never
    # touches waves sees no behaviour change. wave_short_names is
    # auto-maintained by add_wave.py (never hand-edited); wave_themes
    # is hand-curated when used at all -- a theme-to-wave mapping is a
    # judgement call about which open debt genuinely belongs to a
    # wave's own subject matter, not mechanically derivable, so an
    # empty default (no debt cross-referencing) is the correct
    # behaviour for a consumer that hasn't made those calls yet.
    "wave_tracking": "docs/WAVE_TRACKING.md",
    "wave_plan": "docs/WAVE_PLAN.md",
    "wave_short_names": {},
    "wave_themes": {},
    # wave_visibility: wave_id -> bool. Absent = visible (default),
    # matching this project's own additive-default rule -- a consumer
    # that never sets this sees every wave, same as before this key
    # existed. Persisted DATA, not a rendering concern: both
    # render_table() (ASCII) and render_html() read the same dict, so
    # visibility state cannot drift between the two display forms the
    # way it would if each carried its own separate notion of it.
    "wave_visibility": {},
    # wave_html_title: heading text for wave_progress.py --html output.
    # Cosmetic only; default is generic on purpose.
    "wave_html_title": "wave progress",
    "release": {
        "steps": [],          # see relcore.py for the step schema
        "archive": {},        # see relcore.py step_archive
    },
}


def find_root(start: Path | None = None) -> Path:
    p = (start or Path.cwd()).resolve()
    for candidate in [p, *p.parents]:
        if (candidate / ".repoman.json").is_file():
            return candidate
    for candidate in [p, *p.parents]:
        if (candidate / ".git").exists():
            return candidate
    return p


def load(root: Path | None = None) -> tuple[Path, dict]:
    root = root or find_root()
    cfg = json.loads(json.dumps(DEFAULTS))  # deep copy
    f = root / ".repoman.json"
    if f.is_file():
        user = json.loads(f.read_text())
        for k, v in user.items():
            if isinstance(v, dict) and isinstance(cfg.get(k), dict):
                cfg[k].update(v)
            else:
                cfg[k] = v
    return root, cfg


def save_key(root: Path, key: str, value) -> None:
    """Persist a single top-level key's value into .repoman.json,
    creating the file if absent. Reads and writes only the file's own
    on-disk JSON (never the DEFAULTS-merged view load() returns) --
    a consumer who has never customised .repoman.json keeps a minimal
    file after this runs, not a full dump of every default. Used by
    add_wave.py for wave_short_names, the one piece of wave state this
    module ever writes on its own rather than leaving to a human."""
    f = root / ".repoman.json"
    doc = json.loads(f.read_text()) if f.is_file() else {}
    doc[key] = value
    f.write_text(json.dumps(doc, indent=2, ensure_ascii=False) + "\n")
