#!/usr/bin/env python3
"""selftest.py — repoman's acceptance gate.

Prints doctor.py's own environment summary first -- informational
only, never affecting this gate's own pass/fail, since an absent
optional tool (gofmt/bash/node/PyYAML) is a supported operating mode
here, not a defect. See doctor.py's own module docstring for why that
is a separate tool rather than folded into this one's own pass/fail
logic.

Builds a synthetic repository in a temp directory and exercises every
tool against it: ed (its own nine-path selftest), roles, syncver
(set/check/regex target), register (add/close/check round-trip),
guards (list/stale/record), waves (add_wave creates a wave, config
persistence, staleness detection, collision refusal), relcore (full
run, failure halt, resume, archive with manifest + contamination
guard), and a final adversarial section (§8) that targets specific
real bugs found in this project's own history rather than speculative
edge cases: the syncver check()/bool footgun, the RESOLVED.md
quoted-foreign-id bug, legacy/primary id-format coexistence, register
check() genuinely catching a real field mismatch (not just passing on
well-formed data), a non-default guard_id_prefix working end to end,
the multiline/out-of-order "Last exercised" date fix, wave debt
cross-referencing on a synthetic case (proving the logic, not just
matching one already-correct dataset), and a malformed wave heading
warning instead of crashing. Exit 0 green.
"""

import base64
import json
import os
import re
import subprocess
import sys
import tempfile
import zipfile
from pathlib import Path

HERE = Path(__file__).resolve().parent
PKG = HERE
sys.path.insert(0, str(HERE))
import doctor  # noqa: E402


def run(tool, *args, cwd=None):
    return subprocess.run([sys.executable, str(PKG / tool), *args],
                          capture_output=True, text=True, cwd=cwd)


def main() -> int:
    print("-- environment --")
    doctor._print_report(doctor.check(), quiet=True)
    print("-- acceptance gate --")
    checks = 0

    def ok(cond, label, detail=""):
        nonlocal checks
        if not cond:
            print(f"FAIL: {label}\n{detail}")
            sys.exit(1)
        checks += 1
        print(f"ok  {label}")

    # 1. ed's own selftest.
    r = run("ed.py", "selftest")
    ok(r.returncode == 0 and "9 paths green" in r.stdout, "ed selftest",
       r.stdout + r.stderr)

    with tempfile.TemporaryDirectory() as d:
        root = Path(d)
        os.chdir(root)
        (root / "docs").mkdir()
        (root / ".repoman.json").write_text(json.dumps({
            "id_prefix": "Q",
            "version_targets": [
                {"file": "app.py",
                 "match": r'VERSION = "([0-9.]+)"'}],
            "release": {
                "steps": [
                    {"name": "sync", "builtin": "syncver", "always": True},
                    {"name": "build", "run": "echo building > built.txt",
                     "resumable": True},
                    {"name": "archive", "builtin": "archive", "always": True},
                ],
                "archive": {"sources": ["VERSION", "app.py", "docs"],
                            "exclude": ["*.secret"]},
            },
        }))
        (root / "VERSION").write_text("0.0.1\n")
        (root / "app.py").write_text('VERSION = "0.0.1"\n')
        (root / "CHANGELOG.md").write_text(
            "## [0.0.2] - 2026-01-02\n\n## [0.0.1] - 2026-01-01\n")
        (root / "docs" / "TRACKING.md").write_text(
            "# Register\n\nVersion: 0.0.1\n\n## Status table\n\n"
            "| ID | Summary | Theme | Priority | Status | Blocks |\n"
            "|---|---|---|---|---|---|\n"
            "| Q-01 | seed item | core | P2 | ☐ | — |\n\n"
            "## core\n\n### Q-01. seed item\n\n"
            "Theme: core · Priority: P2 · Status: ☐\n\n"
            "- **Trigger:** fixture.\n\n---\n")
        (root / "docs" / "RESOLVED.md").write_text(
            "# Resolved\n\nClosed items, newest first.\n\n"
            "## [0.0.0] Q-00 — genesis (v0.0.0, 2026-01-01)\n\ndone.\n")
        (root / "docs" / "KNOWN_ISSUES.md").write_text(
            "# Known issues\n\nVersion: 0.0.1\n\n## Dormant guards\n\n"
            "### G-01. fixture guard (`x_test.go`)\n\n"
            "- **Gate:** build tag `stress`\n"
            "- **Invocation:** `go test -tags stress ./...`\n"
            "- **Last exercised:** 2025-12-01 env:m1\n")

        # 2. syncver: set writes both the file and the regex target.
        r = run("syncver.py", "set", "0.0.2")
        ok(r.returncode == 0 and 'VERSION = "0.0.2"' in
           (root / "app.py").read_text(), "syncver set + regex target",
           r.stderr)
        r = run("syncver.py", "check")
        ok(r.returncode == 0, "syncver check", r.stdout + r.stderr)

        # 3. register with a non-default prefix: add, check, close.
        r = run("register.py", "add", "--summary", "second", "--theme",
                "core", "--priority", "P3", "--body", "- **Trigger:** t.")
        ok(r.returncode == 0 and "Q-02" in r.stdout, "register add Q-02",
           r.stdout + r.stderr)
        r = run("register.py", "check")
        ok(r.returncode == 0 and "2 open" in r.stdout, "register check",
           r.stdout)
        r = run("register.py", "close", "Q-02", "--version", "0.0.2")
        ok(r.returncode == 0, "register close", r.stdout + r.stderr)
        r = run("register.py", "check")
        ok(r.returncode == 0 and "1 open" in r.stdout,
           "close removed row AND detail", r.stdout)
        ok("Q-02" in (root / "docs" / "RESOLVED.md").read_text(),
           "closure recorded in RESOLVED")

        # 4. guards: list, stale against previous release date, record.
        r = run("guards.py", "list")
        ok("G-01" in r.stdout, "guards list", r.stdout + r.stderr)
        r = run("guards.py", "stale")
        ok(r.returncode == 1 and "G-01" in r.stdout,
           "stale detects unexercised guard", r.stdout)
        r = run("guards.py", "record", "G-01", "--date", "2026-01-03",
                "--env", "ci")
        ok(r.returncode == 0, "guards record", r.stderr)
        r = run("guards.py", "stale")
        ok(r.returncode == 0, "stale clean after record", r.stdout)

        # 5. waves: add_wave creates a wave, wave_progress regenerates
        # the summary, --check confirms it's fresh, wave_short_names
        # persists to .repoman.json, and an explicit collision with
        # an already-taken number is correctly refused.
        (root / "docs" / "WAVE_TRACKING.md").write_text(
            "# Waves\n\n## 1. Progress at a glance\n\n```\n(placeholder)\n```\n\n"
            "Overall by item count: 0 of 0 items \u2248 **0%**\n")
        (root / "docs" / "WAVE_PLAN.md").write_text("# Wave plan\n")
        r = run("add_wave.py", "--name", "first wave", "--ideal-days", "1.0",
                "--items-json", '[{"summary": "seed item", "register_item": "Q-01"}]',
                "--plan-note", "fixture wave.")
        ok(r.returncode == 0 and "Wave number: 1" in r.stdout,
           "add_wave creates wave 1", r.stdout + r.stderr)
        ok("### Wave 1" in (root / "docs" / "WAVE_TRACKING.md").read_text(),
           "wave section inserted")
        ok('"1": "first wave"' in (root / ".repoman.json").read_text(),
           "wave_short_names persisted to config")
        r = run("wave_progress.py", "--check")
        ok(r.returncode == 0, "wave_progress already fresh after add_wave",
           r.stdout + r.stderr)
        r = run("add_wave.py", "--name", "collision", "--ideal-days", "1.0",
                "--items-json", '[{"summary": "x"}]', "--plan-note", "t",
                "--wave-number", "1", "--dry-run")
        ok(r.returncode == 1 and "collides" in r.stderr,
           "add_wave refuses an explicit collision", r.stdout + r.stderr)

        # 5b. A SECOND real (non-dry-run) wave insertion, specifically
        # targeting a real bug found this session: the first version
        # of the separator-reuse logic always added a trailing "---"
        # regardless of whether one already existed from the PREVIOUS
        # insertion, producing a duplicate. The single-insertion check
        # above could never have caught this -- it only manifests on
        # the second insertion onward. Checked directly: exactly one
        # "---" between the two waves, not zero, not two, and a proper
        # blank line on both sides of it.
        r = run("add_wave.py", "--name", "second wave", "--ideal-days", "1.0",
                "--items-json", '[{"summary": "another item", "register_item": "Q-02"}]',
                "--plan-note", "fixture wave two.")
        ok(r.returncode == 0 and "Wave number: 2" in r.stdout,
           "add_wave creates wave 2 (second real insertion)", r.stdout + r.stderr)
        wt_text = (root / "docs" / "WAVE_TRACKING.md").read_text()
        ok("**Wave 1: 0/1, not started.**\n\n### Wave 2" in wt_text,
           "proper blank-line spacing between wave 1 and wave 2, no run-together "
           "heading and no gratuitous separator between two adjacent waves",
           repr(wt_text))
        standalone_seps = re.findall(r"^---\s*$", wt_text, re.M)
        ok(len(standalone_seps) == 1 and wt_text.rstrip().endswith("---"),
           "exactly one trailing separator, after the last wave (nothing else "
           "follows it in this minimal fixture) -- not zero, not a duplicate, "
           "and not confused with a markdown table's own |---|---| row",
           repr(wt_text))

        # 6. relcore: full run, journal, archive with manifest.
        (root / "leak.secret").write_text("x")  # excluded, proves policy
        r = run("relcore.py", "0.0.2")
        ok(r.returncode == 0 and "release v0.0.2 prepared" in r.stdout,
           "relcore full run", r.stdout + r.stderr)
        zips = list(root.glob("*-v0.0.2-checkpoint.zip"))
        ok(len(zips) == 1, "archive produced")
        with zipfile.ZipFile(zips[0]) as z:
            names = z.namelist()
            ok("MANIFEST.sha256" in names, "manifest embedded")
            ok(not any("secret" in n for n in names), "exclusion honoured")

        # 6b. Adversarial, targeting the EXACT bug found and fixed this
        # session: a stale MANIFEST.sha256 already sitting in the source
        # tree (e.g. left over from a prior checkpoint extracted back into
        # a working copy) must not produce a duplicate entry. The archive
        # builtin generates its own manifest and must treat that filename
        # as reserved output, not a source file to sweep in on top of.
        (root / "docs" / "MANIFEST.sha256").write_text("stale, from a prior release\n")
        r = run("relcore.py", "0.0.2", "--resume")
        ok(r.returncode == 0,
           "relcore re-run with a stale docs/MANIFEST.sha256 present",
           r.stdout + r.stderr)
        with zipfile.ZipFile(zips[0]) as z:
            names2 = z.namelist()
            ok("docs/MANIFEST.sha256" not in names2 and
               names2.count("MANIFEST.sha256") == 1,
               "the exclude list matches by basename, the same as every "
               "other pattern already in it (*.log, *.png, ...), so a "
               "same-named file anywhere in the tree is excluded too, not "
               "just one sitting at archive root -- confirms the fix is "
               "consistent with how this mechanism already works "
               "elsewhere, not a special case bolted on beside it", names2)

        # The real bug needs a stale MANIFEST.sha256 at the ARCHIVE ROOT,
        # matching how a real repo's archive.sources typically includes
        # "." (the whole tree) -- that's the shape that collides with the
        # builtin's own top-level z.writestr("MANIFEST.sha256", ...).
        # Reproduced and confirmed red-before/green-after by hand against
        # this exact fix before writing this assertion (not just inferred
        # from the diff): a temporary source list including a literal
        # root-level "MANIFEST.sha256" entry, mirroring what "." would
        # sweep up in a real repo.
        cfgf2 = root / ".repoman.json"
        cfg2 = json.loads(cfgf2.read_text())
        orig_sources = cfg2["release"]["archive"]["sources"]
        cfg2["release"]["archive"]["sources"] = orig_sources + ["MANIFEST.sha256"]
        cfgf2.write_text(json.dumps(cfg2))
        (root / "MANIFEST.sha256").write_text("stale, from a prior release\n")
        r = run("relcore.py", "0.0.2", "--resume")
        ok(r.returncode == 0,
           "relcore re-run with a stale root-level MANIFEST.sha256 present",
           r.stdout + r.stderr)
        with zipfile.ZipFile(zips[0]) as z:
            names3 = z.namelist()
            ok(names3.count("MANIFEST.sha256") == 1,
               "exactly one root-level MANIFEST.sha256 entry, not "
               "duplicated by a pre-existing stale copy at archive root "
               "-- the actual bug found and fixed this session", names3)
        cfg2["release"]["archive"]["sources"] = orig_sources
        cfgf2.write_text(json.dumps(cfg2))
        (root / "MANIFEST.sha256").unlink()

        # 7. relcore: failing step halts; --resume skips the green build.
        cfgf = root / ".repoman.json"
        cfg = json.loads(cfgf.read_text())
        cfg["release"]["steps"].insert(
            2, {"name": "breaker", "run": "exit 3", "resumable": True})
        cfgf.write_text(json.dumps(cfg))
        r = run("relcore.py", "0.0.2")
        ok(r.returncode == 1 and "FAIL breaker" in r.stdout,
           "failure halts the run", r.stdout)
        cfg["release"]["steps"] = [s for s in cfg["release"]["steps"]
                                   if s["name"] != "breaker"]
        cfgf.write_text(json.dumps(cfg))
        r = run("relcore.py", "0.0.2", "--resume")
        ok(r.returncode == 0 and "build: journaled green, skipped"
           in r.stdout, "resume skips journaled step", r.stdout)

        # 8. Adversarial: each of these targets a REAL bug found this
        # session, not a speculative one -- the whole point of testing
        # it is that it actually bit something once.

        # 8a. syncver: check() is a genuine bool, and a real mismatch
        # is correctly detected via the natural `if not check():`
        # idiom -- the exact footgun found 2026-08-17 (a bare tuple is
        # always truthy regardless of content, so this idiom against
        # the OLD check() would never have caught a real mismatch).
        (root / "app.py").write_text('VERSION = "9.9.9"\n')  # deliberately desynced
        r = subprocess.run(
            [sys.executable, "-c",
             "import sys; sys.path.insert(0, sys.argv[1]); import syncver; "
             "v = syncver.check(); "
             "print('TYPE_OK' if isinstance(v, bool) else 'TYPE_BAD', v); "
             "sys.exit(0 if (not v) else 1)",
             str(PKG)],
            capture_output=True, text=True, cwd=root)
        ok(r.returncode == 0 and "TYPE_OK False" in r.stdout,
           "syncver check() is a real bool, and a real mismatch is falsy",
           r.stdout + r.stderr)
        r = run("syncver.py", "set", "0.0.2")  # resync for later checks
        ok(r.returncode == 0, "syncver resynced after adversarial mismatch",
           r.stderr)

        # 8b. register: a closure narrative that QUOTES a much higher,
        # foreign-looking id in free-form prose must not corrupt
        # next_id() -- the exact bug (xolu's own CHANGELOG mentioning
        # "T-160"/"T-161" in prose jumped a different project's
        # sequence from T-70 to T-161) this project's own anchored
        # RESOLVED.md scan exists to prevent.
        resolved_path = root / "docs" / "RESOLVED.md"
        original_resolved = resolved_path.read_text()
        resolved_path.write_text(
            original_resolved
            + "\n## [0.0.2] Q-50 — narrative quoting a foreign id "
              "(v0.0.2, 2026-01-03)\n\n"
              "As described in another project's own Q-999 report, "
              "this closure is unrelated to that number.\n")
        r = run("register.py", "add", "--summary", "third", "--theme",
                "core", "--priority", "P3", "--body", "- **Trigger:** t.",
                "--dry-run")
        ok(r.returncode == 0 and "Q-51" in r.stdout and "Q-1000" not in r.stdout,
           "next_id honours a real structured closure header (Q-50) but "
           "ignores a foreign-looking id quoted in free-form prose (Q-999)",
           r.stdout + r.stderr)
        resolved_path.write_text(original_resolved)  # restore

        # 8c. register: legacy and primary id formats coexist without
        # collision -- xolu's own real, permanent need (T-1..T-163
        # frozen forever, T-164 onward in a new unhyphenated shape).
        cfgf = root / ".repoman.json"
        cfg = json.loads(cfgf.read_text())
        cfg["legacy_id_prefix"] = "L"
        cfg["legacy_id_separator"] = "-"
        cfgf.write_text(json.dumps(cfg))
        tpath = root / "docs" / "TRACKING.md"
        tpath.write_text(tpath.read_text().replace(
            "| Q-01 | seed item | core | P2 | ☐ | — |\n",
            "| Q-01 | seed item | core | P2 | ☐ | — |\n"
            "| L-3 | legacy-format item | core | P3 | ☐ | — |\n"
        ).replace(
            "### Q-01. seed item",
            "### L-3. legacy-format item\n\n"
            "Theme: core · Priority: P3 · Status: ☐\n\n"
            "- **Trigger:** fixture, legacy format.\n\n---\n\n"
            "### Q-01. seed item"))
        r = run("register.py", "check")
        ok(r.returncode == 0 and "2 open" in r.stdout,
           "legacy-format id (L-3) parses alongside primary (Q-01)",
           r.stdout + r.stderr)
        r = run("register.py", "add", "--summary", "fourth", "--theme",
                "core", "--priority", "P3", "--body", "- **Trigger:** t.",
                "--dry-run")
        ok(r.returncode == 0 and "Q-04" in r.stdout,
           "next_id computes from the union of both formats but issues "
           "only the primary shape", r.stdout + r.stderr)
        cfg.pop("legacy_id_prefix")
        cfg.pop("legacy_id_separator")
        cfgf.write_text(json.dumps(cfg))
        tpath.write_text(tpath.read_text().replace(
            "| L-3 | legacy-format item | core | P3 | ☐ | — |\n", ""
        ).replace(
            "### L-3. legacy-format item\n\n"
            "Theme: core · Priority: P3 · Status: ☐\n\n"
            "- **Trigger:** fixture, legacy format.\n\n---\n\n", ""))

        # 8d. register: check() genuinely catches drift, not just
        # passes on well-formed data -- deliberately desync the status
        # table's priority column from the detail block's own field
        # line, confirm A3 fires, then restore.
        original_tracking = tpath.read_text()
        tpath.write_text(original_tracking.replace(
            "| Q-01 | seed item | core | P2 | ☐ | — |",
            "| Q-01 | seed item | core | P4 | ☐ | — |"))  # P2 -> P4, drifted
        r = run("register.py", "check")
        ok(r.returncode == 1 and "A3" in r.stdout,
           "register check catches a real table-vs-detail mismatch",
           r.stdout + r.stderr)
        tpath.write_text(original_tracking)  # restore
        r = run("register.py", "check")
        ok(r.returncode == 0, "register check clean again after restore",
           r.stdout)

        # 8e. guards: a non-default guard_id_prefix works end to end,
        # not just at import time.
        kpath = root / "docs" / "KNOWN_ISSUES.md"
        original_known_issues = kpath.read_text()
        cfg["guard_id_prefix"] = "GRD-"
        cfgf.write_text(json.dumps(cfg))
        kpath.write_text(original_known_issues.replace(
            "### G-01. fixture guard", "### GRD-01. fixture guard"))
        r = run("guards.py", "list")
        ok(r.returncode == 0 and "GRD-01" in r.stdout,
           "non-default guard_id_prefix (GRD-) parses", r.stdout + r.stderr)
        r = run("guards.py", "record", "GRD-01", "--date", "2026-01-04",
                "--env", "ci")
        ok(r.returncode == 0, "record works against the custom prefix",
           r.stderr)
        cfg.pop("guard_id_prefix")
        cfgf.write_text(json.dumps(cfg))
        kpath.write_text(original_known_issues)  # restore default-prefix fixture

        # 8f. guards: a hand-written bullet with two dates NOT in
        # append order -- max() must pick the later one regardless of
        # its position in the text. Backported from xolu's own real
        # fix; this proves it, doesn't just assert it.
        kpath.write_text(re.sub(
            r"^- \*\*Last exercised:\*\*.*$",
            "- **Last exercised:** 2025-12-01 env:m1, following up on the "
            "2026-01-10 finding described above.",
            original_known_issues, count=1, flags=re.M))
        r = run("guards.py", "list")
        ok(r.returncode == 0 and "2026-01-10" in r.stdout,
           "last_date picks the later date even when it appears "
           "earlier in a hand-written, non-append-order bullet",
           r.stdout + r.stderr)
        kpath.write_text(original_known_issues)  # restore

        # 8g. waves: debt cross-referencing actually fires for a real
        # synthetic case, not just on xolu's own already-correct data
        # (which proves nothing about the LOGIC in isolation). Add an
        # open register item whose theme is mapped via wave_themes but
        # is not yet any wave's own planned item.
        tpath.write_text(tpath.read_text().replace(
            "| Q-01 | seed item | core | P2 | ☐ | — |\n",
            "| Q-01 | seed item | core | P2 | ☐ | — |\n"
            "| Q-05 | orphaned debt item | extra | P3 | ☐ | — |\n"
        ).replace(
            "### Q-01. seed item",
            "### Q-05. orphaned debt item\n\n"
            "Theme: extra · Priority: P3 · Status: ☐\n\n"
            "- **Trigger:** fixture.\n\n---\n\n"
            "### Q-01. seed item"))
        cfg["wave_themes"] = {"1": ["extra"]}
        cfgf.write_text(json.dumps(cfg))
        r = run("wave_progress.py", "--show")
        ok(r.returncode == 0 and "debt: Q-05" in r.stdout,
           "an open item's theme correctly surfaces as wave debt",
           r.stdout + r.stderr)
        cfg.pop("wave_themes")
        cfgf.write_text(json.dumps(cfg))
        tpath.write_text(original_tracking)  # restore (drops Q-05 too)

        # 8i. waves: a heading with zero table rows underneath it must
        # warn and skip, never crash the whole regeneration.
        wtpath = root / "docs" / "WAVE_TRACKING.md"
        original_wave_tracking = wtpath.read_text()
        wtpath.write_text(
            original_wave_tracking
            + "\n### Wave 99 — malformed, no rows (0 items, ideal 0d, "
              "added 2026-01-01)\n\n(nothing here)\n")
        r = run("wave_progress.py", "--show")
        ok(r.returncode == 0 and "WARNING" in r.stderr and "Wave 99" not in r.stdout,
           "a malformed wave heading (no rows) warns and is skipped, "
           "not a crash", r.stdout + r.stderr)
        wtpath.write_text(original_wave_tracking)  # restore

        # 8j. wave_progress.py --html: a fresh, dedicated fixture (not
        # reusing the evolved state above) with one done wave, one
        # 0%-complete wave, and one wave whose name contains HTML-
        # special characters -- checked for real escaping, not just
        # that the function exists, since wave names and debt/blocker
        # ids come from documents this tool does not control the
        # content of.
        html_fixture_tracking = (
            "# Waves\n\n## 1. Progress at a glance\n\n```\n(placeholder)\n```\n\n"
            "## 2. Wave-by-wave detail\n\n"
            "### Wave 20 — done (1 items, ideal 1d)\n\n"
            "| # | Summary | Status | Register item |\n|---|---|---|---|\n"
            "| 90 | x | ✓ | Q-90 |\n\n**Wave 20: 1/1, complete.**\n\n"
            "### Wave 21 — <script>evil</script> & \"quotes\" (1 items, ideal 1d)\n\n"
            "| # | Summary | Status | Register item |\n|---|---|---|---|\n"
            "| 91 | y | ☐ | Q-91 |\n\n**Wave 21: 0/1, not started.**\n\n"
            "Overall by item count: 1 of 2 items \u2248 **50%**\n")
        wtpath.write_text(html_fixture_tracking)
        original_short_names = dict(cfg.get("wave_short_names", {}))
        cfg["wave_short_names"] = {"20": "done", "21": '<script>evil</script> & "quotes"'}
        cfgf.write_text(json.dumps(cfg))
        html_out = root / "waves.html"
        r = run("wave_progress.py", "--html", str(html_out))
        ok(r.returncode == 0 and html_out.is_file(),
           "wave_progress --html writes a file", r.stdout + r.stderr)
        html_content = html_out.read_text(encoding="utf-8")
        ok(html_content.startswith("<!DOCTYPE html>") and "<style>" in html_content,
           "output is a genuinely standalone HTML document (DOCTYPE + inline "
           "style, no dependency on an external stylesheet or host CSS "
           "variables)", html_content[:200])
        ok('class="fill success"' in html_content and 'class="fill muted"' in html_content,
           "a complete wave gets the success colour class, an unstarted one "
           "gets muted", html_content)
        ok("<script>evil</script>" not in html_content
           and "&lt;script&gt;evil&lt;/script&gt;" in html_content,
           "a wave name containing HTML-special characters is actually "
           "escaped in the output, not injected raw", html_content)
        wtpath.write_text(original_wave_tracking)  # restore for anything after
        cfg["wave_short_names"] = original_short_names
        cfgf.write_text(json.dumps(cfg))

        # 8k. wave visibility: persisted DATA, not a rendering
        # concern -- --hide/--unhide mutate .repoman.json directly,
        # both render_table (ASCII) and render_html read the SAME
        # dict via is_visible(), and Overall always reflects every
        # wave regardless of what's currently hidden (visibility is a
        # display concern, not a "this doesn't count" concern).
        r = run("wave_progress.py", "--show")
        ok(r.returncode == 0 and "Wave 2" in r.stdout,
           "wave 2 visible by default before any --hide", r.stdout + r.stderr)
        overall_before = [ln for ln in r.stdout.splitlines() if "Overall" in ln][0]

        r = run("wave_progress.py", "--hide", "2")
        ok(r.returncode == 0 and "hidden" in r.stdout and "persisted" in r.stdout,
           "--hide persists and confirms", r.stdout + r.stderr)
        visibility_cfg = json.loads(cfgf.read_text()).get("wave_visibility", {})
        ok(visibility_cfg.get("2") is False,
           "hidden state actually persisted to .repoman.json, not just printed",
           str(visibility_cfg))

        r = run("wave_progress.py", "--show")
        ok(r.returncode == 0 and "Wave 2" not in r.stdout and "Wave 1" in r.stdout,
           "hidden wave omitted from ASCII output, others unaffected",
           r.stdout + r.stderr)
        overall_after_hide = [ln for ln in r.stdout.splitlines() if "Overall" in ln][0]
        ok(overall_after_hide == overall_before,
           "Overall line is byte-identical whether or not a wave is hidden "
           "-- hiding is display-only, the real total never changes",
           f"before={overall_before!r} after={overall_after_hide!r}")

        html_hidden = root / "hidden.html"
        r = run("wave_progress.py", "--html", str(html_hidden))
        ok(r.returncode == 0 and "1 hidden" in r.stdout,
           "--html reports the hidden count", r.stdout + r.stderr)
        ok("Wave 2 " not in html_hidden.read_text(encoding="utf-8"),
           "HTML output respects the SAME persisted visibility state as "
           "ASCII -- one source of truth, not two independently-tracked "
           "notions of what's shown", html_hidden.read_text(encoding="utf-8"))

        html_all = root / "all.html"
        r = run("wave_progress.py", "--html", str(html_all), "--include-hidden")
        ok(r.returncode == 0 and "Wave 2 " in html_all.read_text(encoding="utf-8"),
           "--include-hidden overrides persisted visibility for one render, "
           "without changing the persisted state itself", r.stdout + r.stderr)

        r = run("wave_progress.py", "--unhide", "2")
        ok(r.returncode == 0 and "visible" in r.stdout, "--unhide persists and confirms",
           r.stdout + r.stderr)
        r = run("wave_progress.py", "--show")
        ok(r.returncode == 0 and "Wave 2" in r.stdout,
           "wave 2 visible again in ASCII after --unhide", r.stdout + r.stderr)

        r = run("wave_progress.py", "--hide", "999")
        ok(r.returncode == 0 and "warning" in r.stderr.lower() and "not a wave id" in r.stderr,
           "hiding an unrecognised wave id warns but still proceeds "
           "(the document may not be regenerated yet)", r.stdout + r.stderr)
        r = run("wave_progress.py", "--unhide", "999")  # clean up the stray entry
        ok(r.returncode == 0, "cleanup: unhide the stray test entry", r.stderr)

        # 8l. add_wave.py automatically records visible=True for a
        # newly created wave -- not left to the absent-means-visible
        # fallback alone, matching wave_short_names's own precedent of
        # an explicit, discoverable record for every wave.
        r = run("add_wave.py", "--name", "vis test", "--ideal-days", "1.0",
                "--items-json", '[{"summary": "z", "register_item": "Q-90"}]',
                "--plan-note", "t")
        ok(r.returncode == 0, "add_wave for the visibility-default check succeeds",
           r.stdout + r.stderr)
        new_wave_num = re.search(r"Wave number: (\d+)", r.stdout).group(1)
        visibility_cfg = json.loads(cfgf.read_text()).get("wave_visibility", {})
        ok(visibility_cfg.get(new_wave_num) is True,
           "a newly created wave gets an explicit visible=True entry "
           "written automatically, not left implicit", str(visibility_cfg))

        # 9. str_replace_extended.py: its own embedded 18-path selftest,
        # run as a subprocess the same way `ed.py selftest` is checked
        # above, then a real (non-dry-run) apply against this fixture's
        # own TRACKING.md, proving two things the tool's own isolated
        # selftest cannot: it interoperates with a real repoman-managed
        # register document, and the shared-journal claim (edits from
        # either tool visible to both) holds in the full repoman
        # context, not just the tool's own synthetic temp dir.
        r = run("str_replace_extended.py", "selftest")
        ok(r.returncode == 0 and "ALL GREEN" in r.stdout and "FAIL" not in r.stdout,
           "str_replace_extended selftest", r.stdout + r.stderr)

        sre_payload = json.dumps({
            "v": 1,
            "ops": [{
                "file": "docs/TRACKING.md",
                "search_b64": base64.b64encode(
                    b"# Register\n").decode("ascii"),
                "replace_b64": base64.b64encode(
                    b"# Register (str_replace_extended-edited)\n"
                ).decode("ascii"),
                "expect": 1,
                "roles": ["md-heading"],
            }],
        })
        r = subprocess.run(
            [sys.executable, str(PKG / "str_replace_extended.py"), "apply", "-"],
            input=sre_payload, capture_output=True, text=True, cwd=root)
        ok(r.returncode == 0 and "str_replace_extended-edited" in
           (root / "docs" / "TRACKING.md").read_text(),
           "str_replace_extended applies a real edit against a repoman fixture",
           r.stdout + r.stderr)
        r = run("ed.py", "log")
        ok(r.returncode == 0 and "str_replace_extended" in r.stdout,
           "ed.py log sees the str_replace_extended edit -- shared journal confirmed",
           r.stdout + r.stderr)
        r = run("ed.py", "undo")
        ok(r.returncode == 0 and (root / "docs" / "TRACKING.md").read_text()
           .startswith("# Register\n"),
           "ed.py undo reverts the str_replace_extended edit", r.stdout + r.stderr)

        # 10. doctor.py: environment diagnostic, checked for real
        # rather than just "it imports". Every real tool this sandbox
        # actually has must be correctly detected (a false "not found"
        # here would be worse than the tool never existing -- it would
        # tell a real user their real gofmt/bash/node/PyYAML doesn't
        # work when it does); genuinely hiding a tool via an emptied
        # PATH must correctly flip it to "found: False" with a
        # fallback stated. --quiet must never change what's true, only
        # how much detail is printed. Never affects this gate's own
        # pass/fail (see doctor.py's own docstring for why) -- exit 0
        # is asserted here even with every optional tool hidden.
        report = doctor.check()
        ok(report["python_ok"] and report["platform"], "doctor.check() basic shape",
           str(report))
        real_bash = subprocess.run([sys.executable, "-c",
                                    "import shutil; print(shutil.which('bash'))"],
                                    capture_output=True, text=True).stdout.strip()
        ok((real_bash != "None") == report["bash"]["found"],
           "doctor correctly detects a real, present tool", str(report["bash"]))
        r = subprocess.run([sys.executable, str(PKG / "doctor.py")],
                            capture_output=True, text=True,
                            env={**os.environ, "PATH": "/nonexistent-empty-path"})
        ok(r.returncode == 0 and "not found" in r.stdout,
           "doctor.py under an emptied PATH reports missing tools, "
           "still exits 0 (absence is informational, not a failure)",
           r.stdout + r.stderr)
        r = subprocess.run([sys.executable, str(PKG / "doctor.py"), "--quiet"],
                            capture_output=True, text=True,
                            env={**os.environ, "PATH": "/nonexistent-empty-path"})
        ok(r.returncode == 0 and "enables:" not in r.stdout,
           "doctor.py --quiet omits per-tool detail but still exits 0",
           r.stdout + r.stderr)

        # 11. gomod.py: go.mod/go.sum sanity gate. The replace-directive
        # checks are fully offline and deterministic (go mod edit -json
        # parses the file; nothing needs to resolve over the network),
        # so they run unconditionally. The go.sum-completeness check is
        # inherently network-adjacent -- its entire job is confirming a
        # real dependency resolves -- so that one sub-test uses a real,
        # tiny, stable dependency (zen80) and is skipped, not failed, if
        # this environment genuinely has no outbound network access; a
        # missing network is not this tool's own defect to report as one.
        gomod_dir = root / "gomod-fixture"
        gomod_dir.mkdir()
        (gomod_dir / "go.mod").write_text(
            'module example.com/fixture\n\n'
            'go 1.21\n\n'
            'require github.com/foo/bar v1.2.3\n\n'
            'replace github.com/foo/bar => /root/go/pkg/mod/'
            'github.com/foo/bar@v1.2.3\n'
            'replace github.com/baz/qux => ../local-qux\n'
            'replace github.com/legit/thing => github.com/legit/thing v1.9.9\n')
        r = run("gomod.py", "check", str(gomod_dir))
        ok(r.returncode == 1 and "replace-absolute-path" in r.stdout,
           "gomod.py fails on an absolute-path replace directive "
           "(the exact shape of the real Seam incident)", r.stdout + r.stderr)
        ok("replace-relative-path" in r.stdout and "ERROR replace-relative-path"
           not in r.stdout,
           "a relative-path replace warns, not fails, by default", r.stdout)
        ok("github.com/legit/thing" not in r.stdout,
           "a versioned replace (a real registry redirect) is never flagged",
           r.stdout)
        r = run("gomod.py", "check", "--strict-relative-replace", str(gomod_dir))
        ok(r.returncode == 1 and "ERROR replace-relative-path" in r.stdout,
           "--strict-relative-replace promotes the relative case to a "
           "failure too", r.stdout + r.stderr)

        clean_dir = root / "gomod-clean"
        clean_dir.mkdir()
        (clean_dir / "go.mod").write_text(
            'module example.com/clean\n\ngo 1.21\n')
        r = run("gomod.py", "check", str(clean_dir))
        ok(r.returncode == 0 and r.stdout.strip() == "GOMOD CHECK OK",
           "a clean go.mod with no replace directives and no dependencies "
           "passes cleanly", r.stdout + r.stderr)

        r = subprocess.run([sys.executable, str(PKG / "gomod.py"), "check",
                            str(clean_dir)], capture_output=True, text=True,
                            env={**os.environ, "PATH": "/nonexistent-empty-path"})
        ok(r.returncode == 1 and "go-tooling" in r.stdout,
           "gomod.py fails clearly (not silently) when go itself is not "
           "on PATH -- this check cannot degrade gracefully the way an "
           "optional tool can, since there is no fallback for reading a "
           "Go module file without Go", r.stdout + r.stderr)

        network_dir = root / "gomod-network"
        network_dir.mkdir()
        (network_dir / "go.mod").write_text(
            'module example.com/network\n\ngo 1.21\n\n'
            'require github.com/ha1tch/zen80 v0.1.0\n')
        (network_dir / "main.go").write_text(
            'package main\n\nimport _ "github.com/ha1tch/zen80/z80"\n\n'
            'func main() {}\n')
        try:
            probe = subprocess.run(
                ["go", "list", "-m", "-versions", "github.com/ha1tch/zen80"],
                capture_output=True, text=True, timeout=15, cwd=network_dir)
            network_up = probe.returncode == 0
        except Exception:
            network_up = False
        if network_up:
            r = run("gomod.py", "check", str(network_dir))
            ok(r.returncode == 1 and "gosum-incomplete" in r.stdout,
               "a real dependency with no go.sum entry at all is caught "
               "via go list's own \"missing go.sum entry\" text, not "
               "confused with an unrelated build failure (this is the "
               "same detection path that stays silent for a CGO package "
               "missing system libraries, since go list never compiles "
               "anything)", r.stdout + r.stderr)
        else:
            print("skip  gosum-incomplete detection (no outbound network "
                  "in this environment -- not this tool's own defect)")

        print(f"selftest: all {checks} checks green")
    return 0


if __name__ == "__main__":
    sys.exit(main())
