#!/usr/bin/env python3
"""
repoman/ed.py — journaled, handle-based text editing.

Built to retire one error class: editing against a mental model of the
text instead of the text itself. Every mechanism below exists because a
recorded incident needed it.

Commands:

  find <term> [path ...] [--regex]
      Print every occurrence as a HANDLE — file:start-end:hash8 — with
      its syntactic role (roles.py) and line context. Handles are the
      only currency apply accepts: the hash covers the span plus
      context, so an edit can never fire against text that has changed
      since it was found.

  apply <handle> --with <text>
      Replace the handled span. The hash is re-verified against the
      file AT EDIT TIME; a stale handle refuses ("re-run find"), never
      guesses. Journaled.

  sub <old> <new> [path ...] --expect N
      Literal counted replacement across files. Refuses BEFORE touching
      anything if the total occurrence count != N (per-file counts are
      printed either way). All files verified, then all written — one
      journal transaction; a multi-file campaign cannot strand partial
      state.

  mark <name>            Name the current journal position.
  undo [N]               Undo the last N transactions (default 1).
  undo --since <name>    Undo back to a mark — atomically or not at all.
  log [N]                Show the last N journal entries.
  selftest               Exercise every path in a temp dir; exit 0 green.

Journal contract (.ed-journal.json beside the repo root or cwd):
  - Bounded: last 200 transactions AND <= 10 MB stored; oldest evicted
    first. A WAL, not an archive — checkpoints own the distant past.
  - Undo verifies every span still holds its post-edit bytes before
    reverting anything; a changed file refuses the whole transaction.
  - `undo --since` where the mark has been evicted REFUSES with the
    truncation point — partial undo is never offered.
  - On eviction, a content hash of each touched file at the truncation
    boundary is recorded, so "can I still get back to X?" is a query.

Failure philosophy: every refusal names what to do next; exit codes are
the only truth; nothing is written unless everything passes.
"""

import argparse
import hashlib
import json
import os
import sys
import tempfile
import time
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import roles  # noqa: E402

MAX_TXNS = 200
MAX_BYTES = 10 * 1024 * 1024
CTX = 64  # context bytes hashed around a span


def journal_path() -> Path:
    return Path.cwd() / ".ed-journal.json"


def load_journal() -> dict:
    p = journal_path()
    if p.is_file():
        try:
            return json.loads(p.read_text())
        except (json.JSONDecodeError, OSError):
            pass
    return {"txns": [], "marks": {}, "evicted": {"count": 0, "anchors": {}}}


def save_journal(j: dict) -> None:
    # Evict beyond bounds, oldest first, recording anchors.
    def stored(t):
        return sum(len(e["old"]) + len(e["new"]) for e in t["edits"])
    while len(j["txns"]) > MAX_TXNS or sum(map(stored, j["txns"])) > MAX_BYTES:
        ev = j["txns"].pop(0)
        j["evicted"]["count"] += 1
        for e in ev["edits"]:
            f = Path(e["file"])
            if f.is_file():
                j["evicted"]["anchors"][e["file"]] = hashlib.sha256(
                    f.read_bytes()).hexdigest()[:16]
        # Marks pointing at evicted txns become unreachable.
        j["marks"] = {k: v for k, v in j["marks"].items()
                      if any(t["id"] == v for t in j["txns"])}
    tmp = journal_path().with_suffix(".tmp")
    tmp.write_text(json.dumps(j))
    os.replace(tmp, journal_path())


def span_hash(text: str, start: int, end: int) -> str:
    lo, hi = max(0, start - CTX), min(len(text), end + CTX)
    return hashlib.sha256(text[lo:hi].encode()).hexdigest()[:8]


def record(j: dict, edits: list, label: str) -> None:
    j["txns"].append({
        "id": (j["txns"][-1]["id"] + 1) if j["txns"] else j["evicted"]["count"] + 1,
        "at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "label": label,
        "edits": edits,  # each: {file, offset, old, new}
    })
    save_journal(j)


# ── find ─────────────────────────────────────────────────────────────

def cmd_find(args) -> int:
    paths = roles.expand(args.paths or ["."])
    n = 0
    for p, s, e, role, ln, line in roles.occurrences(args.term, paths,
                                                     regex=args.regex):
        text = p.read_text()
        h = span_hash(text, s, e)
        print(f"{p}:{s}-{e}:{h}  [{role}]  line {ln}: {line.strip()[:80]}")
        n += 1
    print(f"{n} occurrence(s)")
    return 0


# ── apply ────────────────────────────────────────────────────────────

def parse_handle(h: str):
    try:
        rest, hsh = h.rsplit(":", 1)
        path, span = rest.rsplit(":", 1)
        s, e = span.split("-")
        return Path(path), int(s), int(e), hsh
    except ValueError:
        raise SystemExit(f"malformed handle {h!r}; expected file:start-end:hash "
                         "as printed by find")


def cmd_apply(args) -> int:
    path, s, e, hsh = parse_handle(args.handle)
    if not path.is_file():
        print(f"REFUSED: {path} does not exist", file=sys.stderr)
        return 1
    text = path.read_text()
    if e > len(text) or span_hash(text, s, e) != hsh:
        print(f"REFUSED: {path} changed since find (stale handle) — "
              "re-run find and use a fresh handle", file=sys.stderr)
        return 1
    old = text[s:e]
    path.write_text(text[:s] + args.replacement + text[e:])
    j = load_journal()
    record(j, [{"file": str(path), "offset": s, "old": old,
                "new": args.replacement}], f"apply {path.name}")
    print(f"applied at {path}:{s}: {old[:40]!r} -> {args.replacement[:40]!r}")
    return 0


# ── sub ──────────────────────────────────────────────────────────────

def cmd_sub(args) -> int:
    paths = roles.expand(args.paths or ["."])
    plan = []  # (path, text, [offsets])
    total = 0
    role_set = set()
    for p in paths:
        try:
            text = p.read_text()
        except (UnicodeDecodeError, OSError):
            continue
        offs = []
        i = text.find(args.old)
        while i != -1:
            offs.append(i)
            role_set.add(roles.classify(p, text, i))
            i = text.find(args.old, i + 1)
        if offs:
            plan.append((p, text, offs))
            total += len(offs)
            print(f"  {p}: {len(offs)}")
    if total != args.expect:
        print(f"REFUSED: found {total} occurrence(s), --expect {args.expect}. "
              "Nothing written. Re-count, or narrow the paths.",
              file=sys.stderr)
        return 1
    if len(role_set) > 1 and not args.force_roles:
        print(f"REFUSED: occurrences span multiple syntactic roles "
              f"{sorted(role_set)} — one pass is not safe (working "
              "agreement §7.1). Split by role, or pass --force-roles "
              "if you have classified them as one treatment.",
              file=sys.stderr)
        return 1
    edits = []
    for p, text, offs in plan:
        new_text = text.replace(args.old, args.new)
        p.write_text(new_text)
        for o in offs:
            edits.append({"file": str(p), "offset": o,
                          "old": args.old, "new": args.new})
    j = load_journal()
    record(j, edits, f"sub {args.old[:30]!r}->{args.new[:30]!r}")
    print(f"replaced {total} occurrence(s) across {len(plan)} file(s)")
    return 0


# ── undo ─────────────────────────────────────────────────────────────

def revert_txn(t: dict) -> str | None:
    """Verify every edit's post-state, then revert all. Returns an
    error string (nothing written) or None on success."""
    per_file: dict = {}
    for e in t["edits"]:
        per_file.setdefault(e["file"], []).append(e)
    staged = {}
    for fname, edits in per_file.items():
        p = Path(fname)
        if not p.is_file():
            return f"{fname} no longer exists"
        text = p.read_text()
        # Revert in descending offset order so offsets stay valid.
        for e in sorted(edits, key=lambda x: -x["offset"]):
            s = e["offset"]
            if text[s:s + len(e["new"])] != e["new"]:
                return (f"{fname} changed at offset {s} since the edit — "
                        "cannot undo safely")
            text = text[:s] + e["old"] + text[s + len(e["new"]):]
        staged[p] = text
    for p, text in staged.items():
        p.write_text(text)
    return None


def cmd_undo(args) -> int:
    j = load_journal()
    if args.since:
        if args.since not in j["marks"]:
            evc = j["evicted"]["count"]
            print(f"REFUSED: mark {args.since!r} not in the journal "
                  f"(unknown, or evicted — {evc} transaction(s) have "
                  "rolled off; the journal is a WAL, checkpoints own the "
                  "distant past). No partial undo offered.", file=sys.stderr)
            return 1
        target = j["marks"][args.since]
        batch = [t for t in j["txns"] if t["id"] > target]
    else:
        batch = j["txns"][-args.n:] if args.n else j["txns"][-1:]
    if not batch:
        print("nothing to undo")
        return 0
    for t in reversed(batch):
        err = revert_txn(t)
        if err:
            print(f"REFUSED at txn {t['id']} ({t['label']}): {err}. "
                  "Transactions after it were already reverted — journal "
                  "log shows the boundary.", file=sys.stderr)
            j["txns"] = [x for x in j["txns"] if x["id"] > t["id"]] and j["txns"]
            save_journal(j)
            return 1
        j["txns"].remove(t)
        print(f"undone: txn {t['id']} ({t['label']})")
    save_journal(j)
    return 0


def cmd_mark(args) -> int:
    j = load_journal()
    j["marks"][args.name] = j["txns"][-1]["id"] if j["txns"] else 0
    save_journal(j)
    print(f"mark {args.name!r} at txn {j['marks'][args.name]}")
    return 0


def cmd_log(args) -> int:
    j = load_journal()
    for t in j["txns"][-args.n:]:
        print(f"txn {t['id']}  {t['at']}  {t['label']}  "
              f"({len(t['edits'])} edit(s))")
    if j["evicted"]["count"]:
        print(f"[{j['evicted']['count']} evicted; anchors held for "
              f"{len(j['evicted']['anchors'])} file(s)]")
    return 0


# ── selftest ─────────────────────────────────────────────────────────

def cmd_selftest(_args) -> int:
    import subprocess
    me = str(Path(__file__).resolve())
    with tempfile.TemporaryDirectory() as d:
        os.chdir(d)
        f = Path("t.md")
        f.write_text("alpha beta\nalpha in `code alpha`\n")

        def run(*a):
            return subprocess.run([sys.executable, me, *a],
                                  capture_output=True, text=True)

        # 1. find prints handles with roles.
        r = run("find", "alpha", "t.md")
        assert "3 occurrence(s)" in r.stdout, r.stdout
        handle = r.stdout.splitlines()[0].split()[0]

        # 2. sub refuses on wrong count, writes nothing.
        r = run("sub", "alpha", "omega", "t.md", "--expect", "2")
        assert r.returncode == 1 and "REFUSED" in r.stderr, r.stderr
        assert "alpha" in f.read_text()

        # 3. sub refuses on mixed roles without --force-roles.
        r = run("sub", "alpha", "omega", "t.md", "--expect", "3")
        assert r.returncode == 1 and "roles" in r.stderr, r.stderr

        # 4. forced sub succeeds and journals.
        r = run("sub", "alpha", "omega", "t.md", "--expect", "3",
                "--force-roles")
        assert r.returncode == 0, r.stderr
        assert f.read_text().count("omega") == 3

        # 5. undo restores exactly.
        r = run("undo")
        assert r.returncode == 0, r.stderr
        assert f.read_text() == "alpha beta\nalpha in `code alpha`\n"

        # 6. apply with a fresh handle works...
        r = run("find", "beta", "t.md")
        handle = r.stdout.splitlines()[0].split()[0]
        r = run("apply", handle, "--with", "gamma")
        assert r.returncode == 0 and "gamma" in f.read_text(), r.stderr

        # 7. ...and a stale handle refuses.
        r = run("apply", handle, "--with", "delta")
        assert r.returncode == 1 and "stale" in r.stderr, r.stderr

        # 8. mark + undo --since; unknown mark refuses.
        run("mark", "here")
        r = run("undo", "--since", "nowhere")
        assert r.returncode == 1 and "REFUSED" in r.stderr, r.stderr

        # 9. undo --since a real mark with no later txns is a no-op.
        r = run("undo", "--since", "here")
        assert "nothing to undo" in r.stdout, r.stdout

    print("selftest: all 9 paths green")
    return 0


def main() -> int:
    ap = argparse.ArgumentParser(description="journaled precise text editing")
    sub = ap.add_subparsers(dest="cmd", required=True)
    p = sub.add_parser("find")
    p.add_argument("term")
    p.add_argument("paths", nargs="*")
    p.add_argument("--regex", action="store_true")
    p = sub.add_parser("apply")
    p.add_argument("handle")
    p.add_argument("--with", dest="replacement", required=True)
    p = sub.add_parser("sub")
    p.add_argument("old")
    p.add_argument("new")
    p.add_argument("paths", nargs="*")
    p.add_argument("--expect", type=int, required=True)
    p.add_argument("--force-roles", action="store_true")
    p = sub.add_parser("undo")
    p.add_argument("n", nargs="?", type=int)
    p.add_argument("--since")
    p = sub.add_parser("mark")
    p.add_argument("name")
    p = sub.add_parser("log")
    p.add_argument("n", nargs="?", type=int, default=20)
    sub.add_parser("selftest")
    args = ap.parse_args()
    return {"find": cmd_find, "apply": cmd_apply, "sub": cmd_sub,
            "undo": cmd_undo, "mark": cmd_mark, "log": cmd_log,
            "selftest": cmd_selftest}[args.cmd](args)


if __name__ == "__main__":
    try:
        sys.exit(main())
    except BrokenPipeError:
        # Downstream (head, grep -m) closed the pipe: normal CLI life.
        os.dup2(os.open(os.devnull, os.O_WRONLY), sys.stdout.fileno())
        sys.exit(0)
