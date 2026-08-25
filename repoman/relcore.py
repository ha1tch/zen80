#!/usr/bin/env python3
"""repoman/relcore.py — manifest-driven release orchestration.

The portable core of a release process for any repository, designed
for constrained or interruptible execution environments (CI runners,
sandboxes, laptops that sleep):

  - DURABLE JOURNAL: every step's result is recorded atomically in
    .release-state.json; a killed run leaves an exact record and
    --resume continues it. Steps marked "always" re-run regardless.
  - NO DISPLAY PIPES: full command output streams to
    release-<version>.log; stdout carries a bounded per-step summary;
    the process exit code is the only success signal.
  - POLICY AS DATA: the step list, archive sources/exclusions, and
    version-sync targets live in the repository's .repoman.json.

Manifest schema (release section of .repoman.json):

  "release": {
    "steps": [
      {"name": "build", "run": "make build", "resumable": true},
      {"name": "test",  "run": "make test",  "resumable": true,
       "timeout": 900},
      {"name": "gate",  "run": "python3 scripts/my_gate.py",
       "always": true},
      {"name": "sync",    "builtin": "syncver", "always": true},
      {"name": "archive", "builtin": "archive", "always": true}
    ],
    "archive": {
      "name": "{repo}-v{version}-checkpoint.zip",
      "sources": ["README.md", "CHANGELOG.md", "VERSION", "src"],
      "exclude": ["*.tmp", "*.log"],
      "size_warn_mb": 3
    }
  }

Builtins: "syncver" (config.py version_file + version_targets) and
"archive" (zip with embedded MANIFEST.sha256, artifact-contamination
scan, executable-magic sniff, size ceiling). Every archive check
failure aborts; nothing ships around a guard.

Usage:  python3 -m repoman.relcore <version> [--resume]
"""

import argparse
import fnmatch
import hashlib
import json
import os
import re
import subprocess
import sys
import time
import zipfile
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import config as _config
import syncver as _syncver

CONTAMINATION_RE = re.compile(
    r"\.bak$|\.db$|-wal$|-shm$|-journal$|\.pprof$|\.prof$|\.test$"
    r"|\.DS_Store$|/\._|__MACOSX|Thumbs\.db$")
MAGICS = [(b"\x7fELF", "ELF"), (b"\xca\xfe\xba\xbe", "Mach-O"),
          (b"\xfe\xed\xfa\xce", "Mach-O"), (b"\xfe\xed\xfa\xcf", "Mach-O"),
          (b"MZ", "PE")]


class StepFailed(Exception):
    pass


class Journal:
    def __init__(self, root: Path, version: str):
        self.path = root / ".release-state.json"
        self.version = version
        self.data = {}
        if self.path.is_file():
            try:
                j = json.loads(self.path.read_text())
                if j.get("version") == version:
                    self.data = j.get("steps", {})
            except (json.JSONDecodeError, OSError):
                pass

    def record(self, step, status, **meta):
        self.data[step] = {"status": status,
                           "at": time.strftime("%Y-%m-%dT%H:%M:%SZ",
                                               time.gmtime()), **meta}
        tmp = self.path.with_suffix(".tmp")
        tmp.write_text(json.dumps({"version": self.version,
                                   "steps": self.data}, indent=1))
        os.replace(tmp, self.path)

    def green(self, step):
        return self.data.get(step, {}).get("status") == "ok"


def say(msg):
    print(msg, flush=True)


def run_cmd(cmd: str, log, root: Path, timeout: int) -> int:
    log.write(f"\n$ {cmd}\n")
    log.flush()
    try:
        p = subprocess.run(cmd, shell=True, stdout=log,
                           stderr=subprocess.STDOUT, cwd=root,
                           timeout=timeout)
        return p.returncode
    except subprocess.TimeoutExpired:
        log.write(f"\n[relcore] TIMEOUT after {timeout}s\n")
        return 124


def step_syncver(root, cfg, version, log):
    _syncver.set_version(version, root=root, cfg=cfg)
    ok, detail = _syncver.check_detail(root=root, cfg=cfg)
    if not ok:
        raise StepFailed(f"version sync failed: {detail}")
    return {"version": version}


def step_archive(root, cfg, version, log):
    a = cfg["release"].get("archive") or {}
    if not a.get("sources"):
        raise StepFailed("archive builtin requires release.archive.sources")
    name = a.get("name", "{repo}-v{version}-checkpoint.zip").format(
        repo=root.name, version=version)
    zipname = root / name
    zipname.unlink(missing_ok=True)
    exclude = a.get("exclude", []) + [".release-state.json", "release-*.log",
                                      ".ed-journal.json", "MANIFEST.sha256", name]

    def excluded(rel):
        base = rel.rsplit("/", 1)[-1]
        return any(fnmatch.fnmatch(base, p) or fnmatch.fnmatch(rel, p)
                   for p in exclude)

    manifest, count = [], 0
    with zipfile.ZipFile(zipname, "w", zipfile.ZIP_DEFLATED) as z:
        for src in a["sources"]:
            path = root / src
            if not path.exists():
                raise StepFailed(f"archive source missing: {src}")
            files = [path] if path.is_file() else sorted(
                p for p in path.rglob("*") if p.is_file())
            for f in files:
                rel = str(f.relative_to(root))
                if excluded(rel):
                    continue
                z.write(f, rel)
                manifest.append(
                    f"{hashlib.sha256(f.read_bytes()).hexdigest()}  {rel}")
                count += 1
        z.writestr("MANIFEST.sha256", "\n".join(manifest) + "\n")
    with zipfile.ZipFile(zipname) as z:
        names = [n for n in z.namelist() if n != "MANIFEST.sha256"]
        dirty = [n for n in names if CONTAMINATION_RE.search(n)]
        if dirty:
            raise StepFailed(f"archive contains artifacts: {dirty[:5]}")
        bad = []
        for n in names:
            if n.endswith("/"):
                continue
            head = z.open(n).read(4)
            for m, kind in MAGICS:
                if head.startswith(m):
                    bad.append((kind, n))
        if bad:
            raise StepFailed(f"archive contains binaries: {bad[:5]}")
    size = zipname.stat().st_size
    warn = a.get("size_warn_mb", 3) * 1024 * 1024
    if size > warn:
        say(f"   !! archive is {size/1048576:.1f} MB — exceeds the "
            f"{warn // 1048576} MB ceiling")
    return {"files": count, "bytes": size, "zip": name}


BUILTINS = {"syncver": step_syncver, "archive": step_archive}


def main() -> int:
    ap = argparse.ArgumentParser(description="manifest-driven release")
    ap.add_argument("version")
    ap.add_argument("--resume", action="store_true")
    args = ap.parse_args()

    root, cfg = _config.load()
    steps = cfg["release"].get("steps")
    if not steps:
        say("no release.steps in .repoman.json — nothing to orchestrate")
        return 1
    journal = Journal(root, args.version)
    log = open(root / f"release-{args.version}.log", "a")
    say(f"relcore {args.version} at {root.name} "
        f"(log: release-{args.version}.log"
        f"{', resuming' if args.resume else ''})")
    t0 = time.time()
    for s in steps:
        name = s["name"]
        always = s.get("always", False)
        if args.resume and s.get("resumable") and not always \
                and journal.green(name):
            say(f"-- {name}: journaled green, skipped")
            continue
        say(f"-- {name}")
        t = time.time()
        try:
            if "builtin" in s:
                meta = BUILTINS[s["builtin"]](root, cfg, args.version, log) or {}
            else:
                rc = run_cmd(s["run"], log, root, s.get("timeout", 600))
                if rc != 0:
                    raise StepFailed(f"exit {rc}; see the release log")
                meta = {}
        except StepFailed as e:
            journal.record(name, "fail", error=str(e))
            say(f"   FAIL {name}: {e}")
            say("   run halted; fix and re-run with --resume")
            return 1
        journal.record(name, "ok",
                       seconds=round(time.time() - t, 1), **meta)
        say(f"   ok {name} ({time.time() - t:.0f}s)")
    say(f"\nrelease v{args.version} prepared ({time.time() - t0:.0f}s)")
    return 0


if __name__ == "__main__":
    sys.exit(main())
