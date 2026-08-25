# Changelog

## [0.8.0] - 2026-08-21

- **New: `gomod.py`, a go.mod/go.sum sanity gate for Go projects.**
  Built after a real incident: a different project's team hit hardcoded
  Claude-sandbox paths that had reached a committed go.mod via a
  `replace` directive, while trying to get the project running
  themselves. Two checks:
  - **`replace` directive sanity** (always enforced, fully offline,
    deterministic). Uses `go mod edit -json` -- Go's own real parser,
    not a hand-rolled regex -- to read every replace directive. A
    replace entry's `New.Version` being absent is Go's own unambiguous
    signal that the target is a local filesystem path rather than a
    module-registry redirect. An ABSOLUTE local-path target always
    fails: it cannot exist on any machine but the one that wrote it,
    sandbox or not -- exactly the reported failure shape. A RELATIVE
    local-path target (`./foo`, `../foo`) is a legitimate, if
    uncommon, monorepo pattern, so it warns rather than fails by
    default; `--strict-relative-replace` promotes that to a failure
    too for projects that want the stricter rule.
  - **go.sum completeness** (best-effort; degrades to a warning, never
    a false failure, when the environment itself can't finish the
    check). Uses `go list -deps -mod=readonly ./...` rather than `go
    build`: it resolves the full import and module graph without
    compiling or linking anything, so a CGO package missing system
    libraries -- a real, common, and entirely unrelated environment
    gap -- can never be mistaken for a go.sum problem. An incomplete
    go.sum surfaces as an explicit `missing go.sum entry for module
    providing package ...` line, matched verbatim rather than inferred
    from exit code alone (a plain network outage also exits non-zero,
    and must not be reported as the same thing).

  Wire into a project's own release gate as an ordinary `run` step:
  `{"name": "go-sanity", "run": "python3 repoman/gomod.py check",
  "always": true}`. Six new `selftest.py` checks, all verified
  red-before/green-after by hand against real fixtures before being
  written, not inferred from the implementation: an absolute-path
  replace (the exact incident shape), a relative-path replace (warns,
  not fails; fails under `--strict-relative-replace`), a versioned
  replace (never flagged), a clean pass, and `go` itself missing from
  `PATH`. A seventh check (real go.sum-incompleteness detection against
  a live dependency) is network-dependent by nature and correctly
  skips rather than fails when this environment has no outbound
  access -- confirmed manually against a real fixture instead
  (`/tmp/gomod_test2`-style: a genuine missing-entry case, exact error
  text matched) since the hermetic suite couldn't exercise it live
  here.

## [0.7.1] - 2026-08-21

- **Removed: a dead, actively-misleading `repoman/repoman/` duplicate of
  every tool, still present despite being documented as removed back in
  0.2.0.** Found while packaging this release: the CHANGELOG's own 0.2.0
  entry describes deleting this exact subdirectory after discovering
  `selftest.py` had been silently validating it instead of the real,
  current top-level scripts -- but it was still on disk, unchanged since
  the initial commit, meaning that fix never actually made it into this
  repository's committed history despite being documented as done.
  Confirmed nothing references it (`README.md` already described the
  post-removal state correctly; no script imports from it) before
  deleting. `selftest.py`: 68/68 green, unaffected by its removal,
  confirming it really was dead weight.

- **Added `.gitignore`** (this repo never had one): `__pycache__/`,
  `*.pyc`, `.ed-journal.json`, `MANIFEST.sha256`, `.release-state.json`,
  `release-*.log`, `*-checkpoint.zip`. Found the gap because it let two
  real accidents into git history: a stale, pre-existing committed
  `MANIFEST.sha256` (generated packaging output from some earlier
  checkpoint session, of zero ongoing value, its paths still carrying an
  old `repoman/`-prefixed layout this project no longer uses), and a
  `.ed-journal.json` committed by this very session's own `git add -A`
  before the gap was noticed. Both removed; the ignore rules exist so
  neither recurs.

- **Fixed: `relcore.py`'s archive builtin could write a duplicate
  `MANIFEST.sha256` entry.** Found in a downstream repo (zenzx) whose
  working tree had a stale `MANIFEST.sha256` sitting at archive root
  (left over from a prior checkpoint extracted back into a working
  copy) -- `archive.sources` including `"."` swept it in as an
  ordinary source file, then the builtin's own `z.writestr` overwrote
  it a second time, producing two same-named entries in one zip.
  `MANIFEST.sha256` is now in `step_archive`'s own hardcoded exclusion
  list alongside `.release-state.json`/`release-*.log`/
  `.ed-journal.json` -- the same category of self-generated packaging
  output those three already cover, just one entry that had been
  missing from it. Confirmed red-before/green-after by hand against a
  minimal reproduction before writing the regression test, not
  inferred from the diff alone.
- **New regression coverage in `selftest.py`** (checks 67-68): a
  same-named file in a *subdirectory* is correctly excluded too (the
  exclude list matches by basename throughout, same as `*.log`/
  `*.png`/etc. -- not a path-specific special case), and a stale
  `MANIFEST.sha256` at archive *root* no longer duplicates -- the
  actual shape of the bug as found.

## [0.7.0] - 2026-08-17

- **New: persisted per-wave visibility.** Direct correction of a real
  architectural mistake found in this project's own prior history: an
  earlier, in-chat-only version of "hide a wave" baked its toggle
  state into an ephemeral HTML widget's own JS (`display: none` on a
  checkbox), which is why it was gone the moment that chat session
  ended -- there was never anywhere for it to persist. Visibility is
  now wave STATUS, stored the same way `wave_short_names`/
  `wave_themes` already are (a plain dict in `.repoman.json`, absent
  entry = visible by default, zero behaviour change for a consumer
  that never touches this) -- both `render_table()` (ASCII) and
  `render_html()` read the exact same `is_visible()` check, so the two
  display forms cannot independently drift on what's shown.
- **`wave_progress.py --hide ID` / `--unhide ID`** -- single-purpose
  mutations, same shape as `guards.py`'s own `record` command: persist,
  confirm, exit. Warns (but still proceeds) when the given id isn't a
  wave found in the current document, in case the document hasn't been
  regenerated yet.
- **`--include-hidden`** -- renders every wave regardless of persisted
  state, for one invocation, without touching that state. Combinable
  with `--show`, `--html`, or the default write-back.
- **`Overall` always reflects every wave, hidden or not** -- a
  deliberate design choice, stated explicitly rather than left
  implicit: visibility is a display concern, and hiding a wave from
  view doesn't mean its work stops counting toward the real total.
  Verified directly: the `Overall` line is byte-identical before and
  after hiding a wave, not just "close."
- **`add_wave.py` now writes an explicit `visible: true` entry for
  every newly created wave**, matching `wave_short_names`'s own
  existing precedent of an explicit record per wave rather than
  relying on the absent-means-visible fallback alone.
- New config key `wave_visibility` (default `{}`).
- **Found and fixed during implementation, not shipped with it:** a
  leftover reference to a renamed variable in `main()`'s own write-back
  success message, caught immediately by running `add_wave.py`
  end-to-end (not just the isolated unit checks) before trusting the
  change -- the exact discipline this project's own selftest exists to
  enforce, working as intended.
- Selftest: 50 -> 64 checks. Real, not smoke: persisted state actually
  checked in `.repoman.json` (not just the printed confirmation),
  `Overall`'s byte-identical claim checked directly, ASCII and HTML
  checked against the SAME hidden wave, `--include-hidden` checked to
  restore it without mutating stored state, and the new-wave default-
  visibility record checked against a real `add_wave.py` run.
- Default behaviour (no `wave_visibility` ever set) re-verified
  byte-identical to xolu's own real, current output after this change
  -- nothing shifts for an existing consumer that hasn't opted in.

## [0.6.0] - 2026-08-17

- **New: `wave_progress.py --html PATH`** -- renders the same wave/
  debt/blocker data the terminal summary already computes as a
  standalone, portable HTML file. Genuinely standalone, not a port of
  an in-chat prototype: real hex colours with a `prefers-color-scheme`
  dark-mode variant, no dependency on any host page's own CSS
  variables -- the design was first iterated live in a chat session
  using that session's own token system, then rebuilt from scratch
  with literal colour values so the output opens correctly in any
  browser handed the file directly, not just inside the context it was
  prototyped in.
- **Layout matches this project's own established terminal convention
  exactly, not a redesign:** wave name (or a `debt`/`blocked by`
  sublabel, for waves that have either) right-aligned in a fixed left
  column, the bar and its percentage/count -- or the debt/blocker
  pills -- left-aligned in the column beside it, starting at the same
  point the bar itself starts. Debt renders as amber pills, blockers
  as blue `X → Y` pills, both empty unless a wave actually has one.
- **All user-supplied text is escaped, checked directly rather than
  assumed** -- wave names and debt/blocker ids come from documents
  this tool does not control the content of. Verified with a wave name
  containing `<script>`, `&`, and `"` in the same string, confirmed
  absent unescaped and present correctly escaped in the output.
- New config key `wave_html_title` (default `"wave progress"`) --
  cosmetic heading text only.
- Selftest: 46 -> 50 checks. Real coverage, not a smoke test: the
  output file is genuinely written, is a real standalone document
  (`<!DOCTYPE html>` + inline `<style>`, no external stylesheet), a
  complete wave gets the success colour class and an unstarted one
  gets muted, and the escaping claim above is checked against actual
  output, not trusted from the function's own docstring.

## [0.5.1] - 2026-08-17

- **Fixed two real bugs in `add_wave.py`'s `insert_tracking_section`,
  found while backporting this function into a downstream consumer
  (xolu), not found here first.** The version shipped in 0.2.0 fixed
  the original fragile `"## 3."` anchor correctly, but:
  - **Duplicate separator on the second insertion onward.** Always
    added a trailing `---` regardless of whether a previous
    insertion's own `---` was already sitting at the same spot --
    correct on the very first wave added to a document, wrong on
    every one after it. The single-insertion coverage this project's
    own selftest had could never have caught this; it only manifests
    starting on the second insertion.
  - **Missing blank line when nothing at all follows the last wave.**
    The branch handling "this is the literal end of the document"
    inserted a new wave directly against the previous wave's own
    closing line, no blank line between them.
  - Both fixed: an existing trailing separator is now reused, not
    duplicated; a genuinely absent one gets an explicit leading blank
    line added, not assumed present. Verified directly with multiple
    sequential insertions -- four, against an isolated fixture; two,
    against a real downstream consumer's own actual tracking document
    -- before either fix was trusted.
  - **One visual consequence worth stating plainly:** a document whose
    waves were added under the OLD, anchor-based logic has each wave
    individually separated by its own `---` (an accident of that
    logic always re-targeting a fixed anchor, never a deliberate
    convention); waves added under this fix reuse a single separator
    marking the wave-list/next-section boundary instead. Existing
    separators in already-shipped documents are untouched.
- Selftest: a second real (non-dry-run) wave insertion added
  specifically to catch this class of bug -- checks for proper
  blank-line spacing between two adjacent waves and exactly one
  trailing separator, distinguished from a markdown table's own
  `|---|---|` row rather than a naive substring count (an early draft
  of this same check falsely failed on exactly that confusion, caught
  before being trusted).

## [0.5.0] - 2026-08-17

- **New: `doctor.py`, an environment diagnostic.** Prompted directly
  by this project now being used by more than one person, in more
  than one environment this session's own sandbox can't personally
  verify -- `node`'s presence here was an accident of this particular
  container, not evidence of anything about anyone else's machine.
  Checks Python version (the one genuinely blocking condition -- below
  3.10, this project will not run correctly), platform (Debian/Ubuntu,
  macOS, or Ubuntu under WSL2 -- the three tested, supported
  environments; anything else reported as unconfirmed, not refused),
  and each of the four optional external tools (`gofmt`, `bash`,
  `node`, PyYAML), reporting exactly what each one enables when
  present and exactly what fallback applies when it isn't, plus a
  platform-appropriate install command for whatever's missing.
  Deliberately a SEPARATE tool from `selftest.py`, not folded into its
  pass/fail logic: an absent optional tool is one of this project's
  own supported operating modes (a documented heuristic fallback, or
  an honest "not independently verified"), never a defect -- `doctor.py`
  never fails a build over one. `selftest.py` prints `doctor.py`'s own
  summary at the start of its own run, informational only, so the
  standard install ritual surfaces this without a second command
  needing to be remembered; `doctor.py` also stands alone, runnable
  anytime.
- **Verified against genuine tool absence, not simulated in the
  abstract:** every claim above about fallback behaviour was checked
  under a real, restricted `PATH` with `gofmt`/`bash`/`node` actually
  unreachable -- both `str_replace_extended.py`'s own 24-path selftest
  and the outer `selftest.py`'s full 39 (at the time) checks re-run
  clean under it. `doctor.py`'s own detection was checked the same
  way: correctly reports "not found" for a genuinely hidden tool,
  correctly reports "found" for a genuinely present one -- a false
  negative here would be worse than not checking at all, since it
  would tell a real user their real toolchain doesn't work when it
  does.
- Selftest: 39 -> 43 checks. Four added for `doctor.py`: basic report
  shape, correct detection of a real present tool (checked against an
  independent `shutil.which` call, not just internal consistency),
  and both an emptied-`PATH` run and its `--quiet` variant still
  exiting 0 with the right level of detail.
- README: `doctor.py` positioned as the first step of Install, ahead
  of `selftest.py`, with the reasoning stated plainly rather than just
  asserted -- this is visibility into a choice already being made
  silently either way, not a new requirement.

## [0.4.0] - 2026-08-17

- **New language support: JavaScript, TypeScript, CSS, HTML** --
  `roles.py` classification and `str_replace_extended.py` syntax
  validation for all four, prioritized on real, current evidence (an
  active client project, Vikinga, with real in-progress HTML+JS work),
  the same evidence bar every language addition here is held to.
  - **JS/TS (`_js_scan`, shared by both since TypeScript's own type-
    level syntax introduces no new string/comment/template
    delimiter):** a state-STACK scan, not the single-state scan every
    other classifier here uses -- template literals nest arbitrarily
    (a `${...}` substitution can itself contain another template
    literal, idiomatic in real code, not an edge case), and a single-
    state scan mis-terminates at the first inner backtick. Proven
    correct on exactly that case: a triple-nested template literal
    with its own inner substitution, verified character-by-character
    before being trusted. Named limitations: regex-literal boundaries
    are not specially recognised (the classic regex-vs-division
    lexing ambiguity), and JSX tag/expression structure is opaque
    (attribute strings and template literals inside a JSX expression
    container still classify correctly, since they use the same
    delimiters as plain JS/TS).
  - **CSS:** string/comment/code, the same granularity as the existing
    JSON classifier.
  - **HTML:** delegates to JS/CSS roles for the BODY of any
    `<script>`/`<style>` element -- an editor working inside embedded
    script needs JS-aware roles, not a single undifferentiated
    "html-text". Named limitation: inline `onclick=`/`style=`
    attribute content is not delegated, classifying as plain
    attribute-value text.
  - **Syntax validation:** `.js`/`.mjs`/`.cjs` try `node --check`
    first when Node is on PATH (the closest equivalent here to
    `gofmt -e`/`bash -n`), falling back to a role-aware bracket-
    balance heuristic otherwise. `.jsx` is deliberately excluded from
    the `node --check` path even when Node is present -- confirmed
    empirically, not just reasoned about, that plain Node rejects
    valid JSX and valid TypeScript syntax alike, so `.jsx`/`.ts`/`.tsx`
    always use the heuristic path; a "real" check that confidently
    rejects valid input is worse than an honestly-labelled heuristic.
    CSS and HTML are always heuristic (no standard parser reliably
    available to shell out to for either).
- **Documented, not just deferred:** SQL (standalone files), Z80/x86
  assembly, and `ual` explicitly named as candidates left for later in
  both `roles.py`'s own module docstring and a new README section --
  with the reasoning for each, so the decision doesn't need
  re-litigating from scratch when one of them does become a real,
  current need.
- Selftest: `str_replace_extended.py`'s own embedded selftest 18 -> 24
  paths (one per new format, plus two dedicated to the nested-
  template-literal case -- a correct substitution inside the
  innermost substitution, and a correct role-mismatch refusal for text
  in the outer template that isn't code). The outer `selftest.py`'s
  own assertion on this subprocess's output was hardcoded to the old
  path count (`"ALL GREEN (18 paths)"`) -- exactly the kind of stale-
  count trap the project's own README/CHANGELOG counts have already
  been caught by twice this session; fixed to check for "ALL GREEN"
  generically instead of a number that will go stale the next time a
  check is added.

## [0.3.0] - 2026-08-17

- **New: `str_replace_extended.py`** -- format-aware, journaled,
  base64-payload text substitution. Ported from a consumer's own local
  tool of the same name (built to retire the failure class direct
  `str_replace`/`sed`/`awk` substitution invites: content passing
  through a shell/argv quoting layer, and edits landing with no check
  on whether they broke the file's own syntax). The payload is JSON,
  read from stdin or a file, with all search/replace content carried
  as base64 of raw UTF-8 bytes -- a backtick, an apostrophe, a
  newline, or any other character with shell meaning can never be
  reinterpreted before it reaches this tool. Every op requires an
  explicit `expect` (match count) and `roles` (asserted syntactic
  roles) -- no silent defaults -- and refuses, writing nothing, on a
  count mismatch, a role census outside the asserted roles, a
  delimiter-integrity break (including a *balanced* escape-and-reenter
  injection a naive before/after-only check would miss -- found and
  fixed during the tool's own original construction, traced
  character-by-character), or a post-apply syntax-validator failure
  for the file's own format (`gofmt -e`, `ast.parse`, `json.loads`, a
  YAML/Markdown structural lint, `bash -n`). Journals through `ed.py`'s
  own WAL -- one undo history for both tools, confirmed by this
  release's own selftest (`ed.py log`/`undo` correctly see and revert
  a `str_replace_extended` edit). No xolu-specific paths or
  dependencies found anywhere in the 921-line source when ported --
  the only changes needed were two header/doc-comment lines.
- **`roles.py` extended** with python/json/yaml/shell syntactic-role
  classifiers alongside the existing go/md ones (four new whole-text
  or line-local scan functions, wired into the same `classify()`
  dispatch `ed.py` already used) -- required by `str_replace_extended`
  above, ported as a single unit with it since the tool depends on
  these classifiers directly.
- Selftest: 35 -> 39 checks. Four added: the tool's own embedded
  18-path selftest run as a subprocess (matching how `ed.py selftest`
  is already checked), a real end-to-end apply against this project's
  own synthetic register fixture, and confirmation that `ed.py log`
  and `ed.py undo` correctly see and revert the edit -- proving the
  shared-journal claim in the full repoman context, not just the
  tool's own isolated temp-dir selftest.
- Every changed/added file (`register.py`, `wave_progress.py`,
  `roles.py`, `str_replace_extended.py`) re-verified against real,
  current xolu production data after this change, byte-identical to
  xolu's own local scripts' output on the same data.

## [0.2.0] - 2026-08-17

- **New: wave management** (`wave_progress.py`, `add_wave.py`) --
  staged-work ("wave") tracking with mechanically-generated progress
  bars, debt/blocker cross-referencing against the register, and
  collision-safe wave creation. Ported from a consumer's own local
  tooling of the same shape; config-driven paths (`wave_tracking`,
  `wave_plan`) and data (`wave_short_names`, `wave_themes`), both
  defaulting empty/absent so an existing consumer sees no behaviour
  change until it opts in.
- **`register.py`: generalized id-format handling.** New config keys
  `id_separator`, `legacy_id_prefix`, `legacy_id_separator` support a
  mid-project id-prefix migration (a project's own ids changing shape
  partway through its life, with the old shape frozen permanently and
  the new shape forward-only) -- additive and opt-in, byte-identical
  default behaviour for any consumer that doesn't set the new keys.
  Also corrected two stale header claims about a downstream consumer
  still carrying defects this project fixed at extraction; that
  consumer's own later work had already fixed both independently,
  which this project's own header never caught up to until now.
- **`guards.py`: two fixes.** Backported a downstream consumer's own
  fix for multiline "Last exercised" bullets and
  `max()`-over-multiple-dates correctness (a hand-written bullet's
  dates aren't always in append order). Also generalized the
  previously-hardcoded `"G-"` guard-id prefix to a new
  `guard_id_prefix` config key, default `"G-"` for exact backward
  compatibility -- found on review that this project's own id
  generalization (above) hadn't been applied consistently to guards.
- **`syncver.py`: fixed a real API footgun, found tracing a downstream
  consumer's own release orchestration.** `check()` used to return
  `(bool, str)` -- a non-empty tuple, always truthy in Python
  regardless of its content, so the natural `if not check():` idiom
  silently never triggers no matter what the actual sync state is.
  Split into `check() -> bool` (safe for that idiom) and
  `check_detail() -> (bool, str)` for callers that want the message;
  `relcore.py`'s own syncver builtin updated to use `check_detail()`.
- **`config.py`: new `save_key()`** -- writes a single top-level key
  back to `.repoman.json` without re-serializing the full
  defaults-merged view, used by `add_wave.py`'s own short-name
  persistence (moved off injecting a dict-literal line into Python
  source at runtime, a fragile pattern this replaces with a mechanical
  JSON write).
- **Fixed: `selftest.py` had been silently exercising a stale, never-
  synced copy of every tool** in a `repoman/repoman/` subdirectory
  since the initial commit -- confirmed via git history (that
  subdirectory carries exactly one commit, ever, while the top-level
  scripts have since been fixed at least once). The acceptance gate
  has apparently never actually validated the real, current scripts.
  Fixed by pointing `selftest.py` at the top-level scripts directly;
  the dead, actively-misleading duplicate removed. `README.md`'s own
  Install section, which described copying the now-removed
  subdirectory, corrected to match.
- **Selftest: eighteen checks -> thirty-five.** Twelve added for wave
  creation, section placement, config persistence, staleness
  detection, and collision refusal. A further section (§8) targets
  specific real bugs found in this project's own history rather than
  speculative edge cases -- see `selftest.py`'s own module docstring
  for the full list. Every one of these adversarial checks reproduces
  a failure mode that actually occurred (this project's own or a
  consumer's), not a hypothetical.
- **Not done, found and deliberately deferred:** `syncver.py`'s own
  `version_targets` generalization exists but the release-
  orchestration side of at least one downstream consumer needed its
  own adaptation beyond the `check()` fix above -- flagged, not folded
  in, since it's a separate, correctly-scoped piece of work belonging
  to that consumer's own release code, not this project.

## [0.1.0] - 2026-07-22

- Initial release: ed, roles, register, guards, syncver, relcore, and
  the eighteen-check selftest acceptance gate. Pure stdlib Python.
