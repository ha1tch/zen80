# repoman

Repository-discipline tooling in pure Python 3 (stdlib only). Built for
precise text editing, tracked-work registers, dormant-guard currency,
version sync, and interruptible release orchestration — in any
repository, driven by one optional `.repoman.json`.

## Requirements

- Python 3.10 or later. No third-party dependencies.

## Install

Clone the repository, or copy its files into yours (conventionally
under `scripts/` or as a sibling directory) -- the tools are plain
files at this repository's own root, not a package to import. A
repository opts in with `.repoman.json` at its root — an empty `{}` is
valid; every key has a documented default (`config.py`).

Run `python3 doctor.py` first -- an environment diagnostic, not a
pass/fail test: Python version, platform, and which of the four
optional external tools (`gofmt`, `bash`, `node`, PyYAML) this
environment has, with an install command for whatever's missing.
Tested, supported environments: Debian/Ubuntu, macOS, and Ubuntu under
WSL2 -- anything else is not refused, just unconfirmed, and `doctor.py`
says so plainly rather than guessing. None of this is required for
repoman to work: every optional tool has a documented fallback (a
heuristic check, or an honest "not independently verified") when
absent, and `selftest.py` passes cleanly either way -- `doctor.py`
exists so that's a visible, informed choice, not a silent one.

Then run `python3 selftest.py` -- the actual acceptance gate.
Sixty-four checks (plus `ed.py`'s own 9-path and
`str_replace_extended.py`'s own 24-path embedded selftests, each
runnable standalone, and a `doctor.py` environment summary printed
first purely for visibility); exit 0 is the acceptance gate. Do not
trust an installation whose selftest fails.

## Tools

| Tool | Purpose |
|------|---------|
| `doctor.py` | Environment diagnostic: Python version, platform, and which of `gofmt`/`bash`/`node`/PyYAML are available -- run this first, informational only, never a pass/fail gate |
| `ed.py` | Journaled handle-based editing: `find` / `apply` / `sub --expect` / bounded `undo` (`selftest` embedded) |
| `roles.py` | Syntactic-role auditor: classify every occurrence of a term before substituting -- go, markdown, python, json, yaml, shell, javascript, typescript, css, html |
| `str_replace_extended.py` | Format-aware, journaled, base64-payload substitution: `expect` (match count) and `roles` (asserted syntactic roles) mandatory on every op, refuses on a delimiter-integrity break or a post-apply syntax-validator failure, journals through `ed.py`'s own WAL (`selftest` embedded) |
| `register.py` | Work-register operations: `add` / `close` / `check` over a tracking document, closures recorded append-only |
| `guards.py` | Dormant-guard registry: `stale` / `handoff` / `record` for verification that does not run by default |
| `syncver.py` | Version sync: plain VERSION file plus regex-targeted stamps in code or docs |
| `relcore.py` | Manifest-driven release orchestration: durable journal, `--resume`, no-display-pipes logging, archive builtin with embedded SHA-256 manifest, contamination scan, and binary sniff |
| `wave_progress.py` | Staged-work ("wave") progress: regenerates a wave-tracking document's summary from its own per-wave tables, with debt/blocker cross-referencing against the register; `--html PATH` renders the same data as a standalone HTML file; `--hide`/`--unhide ID` persist per-wave visibility as data in `.repoman.json` (both ASCII and HTML respect it; `Overall` always reflects every wave regardless) |
| `add_wave.py` | Adds a new wave deterministically: collision-checked wave and item numbers, sections inserted into both the tracking and plan documents, bars regenerated -- one call, nothing left as a manual follow-up |
| `gomod.py` | go.mod/go.sum sanity gate for Go projects: fails on a `replace` directive pointing at an absolute local filesystem path (the exact shape of a real incident -- a hardcoded sandbox path reaching a downstream team's go.mod), warns on a relative one, and separately confirms go.sum is complete enough for a plain `go build -mod=readonly` to work with no extra magic, via `go list`'s own dependency-graph resolution rather than a full compile (so a CGO package missing system libraries is never mistaken for a go.sum problem) |

Every tool supports `--help`; every refusal names what to do next; the
process exit code is the only success signal. Full command output goes
to per-run log files — the tools are designed so no caller ever needs
to pipe their output to read it, because piping is how exit codes get
silently disarmed.

The suite is suitable for constrained or interruptible environments —
CI runners with execution ceilings, containers that reap processes,
laptops that sleep mid-run. Interrupted work resumes from its journal;
nothing completed is lost.

## Configuration

See `config.py` for defaults and `relcore.py` for the release-manifest
schema, including a worked example.

## Language support (`roles.py` / `str_replace_extended.py`)

Currently classified and syntax-validated: Go, Markdown, Python, JSON,
YAML, shell, JavaScript, TypeScript, CSS, HTML. See `roles.py`'s own
module docstring for the exact role vocabulary and each classifier's
own documented, honest limitations (heuristic by design, never silent
about it).

**Deliberately not supported yet -- deferred, not forgotten:** SQL
(standalone `.sql` files -- the dangerous case, SQL built via Go
string concatenation, is already covered by the Go classifier's own
delimiter-integrity awareness), Z80/x86 assembly (`.asm`), and `ual`
(a first-party, still-evolving language whose own string/comment
rules aren't settled enough yet for a classifier to be worth building
before they are). Add support for any of these the same way
JS/TS/CSS/HTML earned theirs: a real, current project actively needing
it, not a speculative future one. Full rationale on each in
`roles.py`'s own module docstring.

## License

GPLv3.0 - Copyright (c) 2026 haitch. https://ual.li
