#!/usr/bin/env python3
"""repoman/doctor.py — environment diagnostic, run once after install
alongside (before, ideally) `selftest.py`.

Different job from selftest.py, deliberately: selftest.py asks "is the
CODE correct" and is a pass/fail gate. This asks "what does THIS
environment provide", and most of its answers are not failures at all
-- str_replace_extended.py's whole design already treats every
external tool it can use (gofmt, bash, node, PyYAML) as an optional
enhancement with a documented, working fallback when absent (a
heuristic check, or an honest "not independently verified" instead of
a real one). Missing gofmt is not broken; it is one of this project's
own supported operating modes. What doctor.py adds is visibility: a
newcomer to a repository this project's tools are installed in should
not have to read str_replace_extended.py's own source to learn which
of their edits get a real syntax check and which get a heuristic one,
or spend time discovering a platform-specific install command by
trial and error.

Supported, tested environments (per project convention -- anything
else is not unsupported out of malice, just untested, and this tool
says so plainly rather than guessing): Debian/Ubuntu, macOS, and
Ubuntu-under-WSL2. Install hints below are for these three only.

Exit code: 0 unless Python's own version is below the floor this
project's own code already assumes (f-strings with `=`, `match`,
`list[...]` generics -- 3.10). Every other finding is informational;
this tool never fails a build over an optional tool's absence, since
absence is a supported mode, not a defect.

Usage:
    python3 repoman/doctor.py
    python3 repoman/doctor.py --quiet   # summary line only, still exit 0/1
"""

import importlib.util
import os
import platform
import re
import shutil
import subprocess
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import str_replace_extended as _sre  # noqa: E402 -- reuse _which_gofmt() so this
                                       # report can never drift from what the
                                       # real validator actually does


def _detect_platform() -> tuple:
    """Returns (label, supported: bool). WSL2 is detected via
    /proc/version's own kernel string (Microsoft's WSL kernels self-
    identify there) rather than any environment variable, since env
    vars are easy to leave unset in a script's own invocation context
    but the kernel string cannot lie about what it is."""
    system = platform.system()
    if system == "Darwin":
        ver = platform.mac_ver()[0] or "unknown version"
        return f"macOS {ver}", True
    if system == "Linux":
        is_wsl = False
        try:
            proc_version = Path("/proc/version").read_text().lower()
            is_wsl = "microsoft" in proc_version or "wsl" in proc_version
        except OSError:
            pass
        distro = _linux_distro()
        if is_wsl:
            return f"WSL2 / {distro}" if distro else "WSL2 (distro unknown)", True
        if distro and "ubuntu" in distro.lower():
            return distro, True
        if distro and "debian" in distro.lower():
            return distro, True
        return distro or "Linux (distro unknown)", False
    return f"{system} {platform.release()}", False


def _linux_distro() -> str:
    try:
        text = Path("/etc/os-release").read_text()
    except OSError:
        return ""
    m = re.search(r'^PRETTY_NAME="?([^"\n]+)"?', text, re.M)
    return m.group(1) if m else ""


def _pkg_hints(system: str) -> dict:
    """Install commands, keyed by tool name -- only shown for tools
    actually found missing, never printed unprompted for tools already
    present."""
    if system == "Darwin":
        return {
            "go": "brew install go",
            "node": "brew install node",
            "pyyaml": "pip3 install pyyaml  (add --break-system-packages, or use a "
                      "venv, if this Python is Homebrew's own and refuses global "
                      "installs)",
            # bash: no hint -- macOS ships it unconditionally as part of the OS,
            # this case should not be reachable there.
        }
    # Debian/Ubuntu, native or under WSL2 -- same package manager either way.
    return {
        "go": "sudo apt install golang-go   (ships gofmt; for a newer Go than "
              "Ubuntu's own package, see https://go.dev/dl/)",
        "node": "sudo apt install nodejs npm",
        "bash": "sudo apt install bash   (genuinely absent on some minimal/"
                "Alpine-derived images -- not a concern on a standard "
                "Debian/Ubuntu install, which always includes it)",
        "pyyaml": "pip3 install pyyaml --break-system-packages   (Debian/Ubuntu's "
                  "system Python refuses a bare global install since Python "
                  "3.11 -- this project's own sandbox needed exactly this flag)",
    }


def _tool_version(cmd: list) -> str:
    try:
        r = subprocess.run(cmd, capture_output=True, text=True, timeout=10)
        line = (r.stdout or r.stderr or "").strip().splitlines()
        return line[0] if line else "(version unknown)"
    except (OSError, subprocess.TimeoutExpired):
        return "(version unknown)"


def check() -> dict:
    """Returns a structured report; used by both main() below and by
    selftest.py, which prints a short summary of this at the start of
    its own run without treating any of it as pass/fail."""
    report: dict = {"python_ok": sys.version_info >= (3, 10),
                     "python_version": platform.python_version()}

    plat_label, plat_supported = _detect_platform()
    report["platform"] = plat_label
    report["platform_supported"] = plat_supported

    gofmt = _sre._which_gofmt()
    report["gofmt"] = {
        "found": gofmt is not None, "path": gofmt,
        "version": _tool_version([gofmt, "-h"]) if gofmt else None,
        "enables": "real gofmt -e syntax validation for .go substitutions",
        "fallback": "\"validated\": None (not independently verified) -- "
                    "no heuristic exists for Go yet",
    }

    bash = shutil.which("bash")
    report["bash"] = {
        "found": bash is not None, "path": bash,
        "version": _tool_version([bash, "--version"]) if bash else None,
        "enables": "real bash -n syntax validation for .sh/.bash substitutions",
        "fallback": "\"validated\": None (not independently verified) -- "
                    "no heuristic exists for shell yet",
    }

    node = shutil.which("node")
    report["node"] = {
        "found": node is not None, "path": node,
        "version": _tool_version([node, "--version"]) if node else None,
        "enables": "real node --check syntax validation for .js/.mjs/.cjs "
                   "substitutions (.jsx/.ts/.tsx always use the heuristic "
                   "path regardless -- see _validate_js's own docstring)",
        "fallback": "\"validated\": \"heuristic\" -- role-aware bracket/string "
                    "balance check, real but reduced-confidence",
    }

    yaml_spec = importlib.util.find_spec("yaml")
    yaml_version = None
    if yaml_spec is not None:
        try:
            import yaml  # type: ignore
            yaml_version = getattr(yaml, "__version__", "(version unknown)")
        except ImportError:
            yaml_spec = None
    report["pyyaml"] = {
        "found": yaml_spec is not None, "path": None,
        "version": yaml_version,
        "enables": "real yaml.safe_load validation for .yaml/.yml substitutions",
        "fallback": "\"validated\": \"heuristic\" -- tab/bracket structural "
                    "check, real but reduced-confidence",
    }

    return report


def _print_report(report: dict, quiet: bool) -> None:
    py_mark = "OK" if report["python_ok"] else "FAIL"
    print(f"[{py_mark}] Python {report['python_version']} "
          f"({'>= 3.10, fine' if report['python_ok'] else 'BELOW 3.10 -- this '
             'project will not run correctly'})")

    plat_mark = "OK" if report["platform_supported"] else "??"
    print(f"[{plat_mark}] Platform: {report['platform']}"
          + ("" if report["platform_supported"] else
             " -- not one of this project's tested environments "
             "(Debian/Ubuntu, macOS, Ubuntu/WSL2). May still work; untested, "
             "not unsupported out of malice."))

    if quiet:
        missing = [k for k in ("gofmt", "bash", "node", "pyyaml")
                   if not report[k]["found"]]
        if missing:
            print(f"optional tools not found: {', '.join(missing)} "
                  f"(fallbacks apply -- run without --quiet for detail)")
        return

    print()
    hints = _pkg_hints(platform.system())
    hint_key = {"gofmt": "go", "bash": "bash", "node": "node", "pyyaml": "pyyaml"}
    for name in ("gofmt", "bash", "node", "pyyaml"):
        info = report[name]
        if info["found"]:
            where = f" ({info['path']})" if info.get("path") else ""
            ver = f" -- {info['version']}" if info.get("version") else ""
            print(f"[OK] {name}{where}{ver}")
            print(f"     enables: {info['enables']}")
        else:
            print(f"[--] {name}: not found")
            print(f"     without it: {info['fallback']}")
            key = hint_key.get(name)
            if key and key in hints:
                print(f"     install: {hints[key]}")
        print()


def main(argv: list) -> int:
    quiet = "--quiet" in argv
    report = check()
    _print_report(report, quiet)
    return 0 if report["python_ok"] else 1


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
