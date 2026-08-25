#!/usr/bin/env python3
"""
repoman/roles.py — syntactic-role classification for text occurrences.

Mechanizes the mass-substitution rule: before any substitution,
classify every occurrence of the target by its syntactic role; a single
pass is safe only when all occurrences share one role and one correct
treatment. This module answers "what roles does this text appear in?"
so that judgment starts from facts.

Importable (ed.py uses classify()) and a CLI auditor:

    python3 repoman/roles.py <term> [path ...]

prints every occurrence with its role. Roles are HEURISTIC and
advisory — they inform the classification step, they do not replace it.

Role vocabulary:
  go-backtick-string | go-dquote-string | go-comment | go-code
  md-fence | md-inline-code | md-table | md-heading | md-prose
  python-string | python-comment | python-code
  json-string | json-code
  yaml-string | yaml-comment | yaml-code
  shell-squote-string | shell-dquote-string | shell-backtick-string
  shell-comment | shell-code
  js-string | js-template-string | js-comment | js-code
  ts-string | ts-template-string | ts-comment | ts-code
  css-string | css-comment | css-code
  html-comment | html-tag | html-attr-dquote | html-attr-squote | html-text
  text

Known classifier limitations (heuristic by design, documented rather
than silently wrong -- see each _*_role's own docstring for detail):
YAML block scalars (|, >) misclassify as yaml-code; shell heredocs
misclassify as shell-code; Python f-string {expr} interiors classify
as python-string, not a separate role; JS/TS regex literals are not
specially recognised (the classic regex-vs-division ambiguity is not
resolved) and JSX tag/expression structure is opaque, both documented
on _js_scan; HTML's <script>/<style> BODY content delegates to
js-*/css-* roles, but inline onclick=/style= attribute content does
not, classifying as plain html-attr-*.

Not yet supported at all -- deferred, not forgotten (2026-08-17):
SQL (standalone .sql files -- the dangerous case, SQL built via Go
string concatenation, is already covered by go-code's own delimiter-
integrity awareness; a standalone-file classifier is lower priority
absent a real consumer with checked-in .sql files), Z80/x86 assembly
(.asm -- a real, maintained project (zenzx/zenas) exists in this
ecosystem, just not this tool's priority yet), ual (a first-party,
still-evolving language -- its own string/comment/quoting rules are
not settled enough yet that a classifier wouldn't need reworking every
time the language does; revisit once the syntax stabilises). Add
support for any of these when a project actively needs it, the same
evidence bar JS/TS/CSS/HTML were held to (Vikinga's real, in-progress
HTML+JS work, not a speculative future need).
"""

import re
import sys
from pathlib import Path


def _go_role(text: str, offset: int) -> str:
    """Scan the line-local context; delimiter state tracked from the
    start of the enclosing line (Go string literals cannot span lines
    except backticks — for those, scan back to the opening backtick)."""
    # Backtick strings can span lines: count backticks before offset.
    if text[:offset].count("`") % 2 == 1:
        return "go-backtick-string"
    line_start = text.rfind("\n", 0, offset) + 1
    line = text[line_start:offset]
    # Comment?
    if "//" in _strip_go_strings(line):
        return "go-comment"
    # Double-quoted string state within the line.
    in_dq = False
    i = 0
    while i < len(line):
        c = line[i]
        if c == "\\" and in_dq:
            i += 2
            continue
        if c == '"':
            in_dq = not in_dq
        i += 1
    if in_dq:
        return "go-dquote-string"
    # Block comments: crude but honest — count openers/closers.
    before = text[:offset]
    if before.count("/*") > before.count("*/"):
        return "go-comment"
    return "go-code"


def _strip_go_strings(line: str) -> str:
    out, in_dq, i = [], False, 0
    while i < len(line):
        c = line[i]
        if c == "\\" and in_dq:
            i += 2
            continue
        if c == '"':
            in_dq = not in_dq
            i += 1
            continue
        if not in_dq:
            out.append(c)
        i += 1
    return "".join(out)


def _md_role(text: str, offset: int) -> str:
    before = text[:offset]
    # Fenced block: odd number of ``` fences before us.
    if len(re.findall(r"^```", before, re.M)) % 2 == 1:
        return "md-fence"
    line_start = before.rfind("\n") + 1
    line_end = text.find("\n", offset)
    line = text[line_start:line_end if line_end != -1 else len(text)]
    prefix = text[line_start:offset]
    if prefix.count("`") % 2 == 1:
        return "md-inline-code"
    if line.lstrip().startswith("|"):
        return "md-table"
    if line.lstrip().startswith("#"):
        return "md-heading"
    return "md-prose"


def _js_scan(text: str, offset: int, lang: str) -> str:
    """Whole-text forward scan with an explicit state STACK, not a
    single state variable -- needed because template literals nest
    arbitrarily (a `${...}` substitution can itself contain another
    template literal, which can contain another substitution, and so
    on; this is idiomatic in real JS/TS, not an edge case). A single-
    state scan, as used for Python/Go strings, would mis-terminate at
    the FIRST inner backtick and misclassify everything after it.

    Frames: ("str", delim) for '/" strings; ("tmpl",) for a template
    literal's own text; ("subst", depth) for the code inside a `${`
    substitution, tracking brace depth so nested braces (object
    literals, arrow-function bodies) inside the substitution don't
    close it early; ("lc",) / ("bc",) for // and /* */ comments.
    `lang` ("js" or "ts") only changes the returned role names --
    TypeScript's type-level syntax (annotations, generics, interfaces)
    introduces no new string/comment/template delimiter, so the scan
    itself is identical for both.

    KNOWN LIMITATION, documented rather than silently wrong: regex
    literals (/pattern/flags) are not specially recognised -- the
    classic regex-vs-division lexing ambiguity in JS is not resolved
    here. A regex body is scanned as ordinary code; if it contains an
    unescaped quote or comment-opening sequence, this can misclassify
    following text. Rare in practice (most regex bodies contain
    neither), and the delimiter-integrity check in
    str_replace_extended.py still catches most resulting damage via
    role-divergence -- the same mitigation already relied on for
    YAML's own block-scalar limitation and shell's own heredoc
    limitation. JSX (`<Component>...</Component>`, `{expr}`
    containers in .jsx/.tsx) is likewise not specially modelled: JSX
    attribute strings and any template literals inside a JSX
    expression container are still classified correctly (same quote
    delimiters as plain JS), but element/tag structure itself is
    opaque here, the same boundary _html_role draws around embedded
    script/style bodies.
    """
    i, n = 0, len(text)
    stack: list = []
    while i < offset:
        c = text[i]
        top = stack[-1] if stack else None
        kind = top[0] if top else None
        if kind is None:
            if text[i:i + 2] == "//":
                stack.append(("lc",)); i += 2; continue
            if text[i:i + 2] == "/*":
                stack.append(("bc",)); i += 2; continue
            if c in ("'", '"'):
                stack.append(("str", c)); i += 1; continue
            if c == "`":
                stack.append(("tmpl",)); i += 1; continue
            i += 1; continue
        if kind == "lc":
            if c == "\n":
                stack.pop()
            i += 1; continue
        if kind == "bc":
            if text[i:i + 2] == "*/":
                stack.pop(); i += 2; continue
            i += 1; continue
        if kind == "str":
            delim = top[1]
            if c == "\\":
                i += 2; continue
            if c == delim:
                stack.pop()
            i += 1; continue
        if kind == "tmpl":
            if c == "\\":
                i += 2; continue
            if c == "`":
                stack.pop(); i += 1; continue
            if text[i:i + 2] == "${":
                stack.append(("subst", 1)); i += 2; continue
            i += 1; continue
        if kind == "subst":
            depth = top[1]
            if text[i:i + 2] == "//":
                stack.append(("lc",)); i += 2; continue
            if text[i:i + 2] == "/*":
                stack.append(("bc",)); i += 2; continue
            if c in ("'", '"'):
                stack.append(("str", c)); i += 1; continue
            if c == "`":
                stack.append(("tmpl",)); i += 1; continue
            if c == "{":
                stack[-1] = ("subst", depth + 1); i += 1; continue
            if c == "}":
                if depth == 1:
                    stack.pop()
                else:
                    stack[-1] = ("subst", depth - 1)
                i += 1; continue
            i += 1; continue
    if not stack:
        return f"{lang}-code"
    kind = stack[-1][0]
    if kind == "subst":
        return f"{lang}-code"
    return {"str": f"{lang}-string", "tmpl": f"{lang}-template-string",
            "lc": f"{lang}-comment", "bc": f"{lang}-comment"}[kind]


def _js_role(text: str, offset: int) -> str:
    return _js_scan(text, offset, "js")


def _ts_role(text: str, offset: int) -> str:
    return _js_scan(text, offset, "ts")


def _css_role(text: str, offset: int) -> str:
    """Whole-text forward scan: /* */ block comments (CSS has no line
    comments), '...'/"..." strings with backslash escapes. KNOWN
    LIMITATION: an unquoted url(...) value's content is classified as
    plain css-code, not distinguished from a selector/property/value
    -- this classifier draws the same string/comment/code boundary as
    the JSON classifier, no finer-grained than that."""
    i, n = 0, len(text)
    state = None  # None | "comment" | ("str", delim)
    while i < offset:
        c = text[i]
        if state is None:
            if text[i:i + 2] == "/*":
                state = "comment"; i += 2; continue
            if c in ("'", '"'):
                state = ("str", c); i += 1; continue
            i += 1; continue
        if state == "comment":
            if text[i:i + 2] == "*/":
                state = None; i += 2; continue
            i += 1; continue
        _, delim = state
        if c == "\\":
            i += 2; continue
        if c == delim:
            state = None
        i += 1; continue
    if state is None:
        return "css-code"
    if state == "comment":
        return "css-comment"
    return "css-string"


_HTML_EMBED_OPEN_RE = re.compile(r"<\s*(script|style)\b[^>]*>", re.I)


def _html_embedded_spans(text: str) -> list:
    """Every <script>...</script> / <style>...</style> BODY span (the
    text strictly between the opening tag's own closing '>' and the
    matching closing tag), as (body_start, body_end, lang) tuples in
    document order. A separate pre-pass rather than interleaving
    delegation into _html_role's own scan below -- finding matching
    spans first, then deciding whether `offset` falls in one, is far
    simpler to get right than threading a sub-scanner into a single
    linear walk, and this list is cheap (documents are not large
    enough for repeated regex scans here to matter at this tool's
    scale).

    KNOWN LIMITATION: the closing-tag search is a literal, case-
    insensitive text search for </script> or </style> -- it does not
    parse attributes on the closing tag (not valid HTML in practice),
    and like a real browser's own HTML parser, has no way to
    distinguish a literal "</script>" appearing unescaped inside a
    script body's own string content from a genuine closing tag. Real
    HTML has the identical hazard for exactly this reason (the
    documented escape is <\\/script>); this is not a rougher
    approximation than HTML's own parsing rules, it is the same one.
    """
    spans = []
    for m in _HTML_EMBED_OPEN_RE.finditer(text):
        tagname = m.group(1).lower()
        body_start = m.end()
        close_re = re.compile(r"</\s*" + tagname + r"\s*>", re.I)
        m2 = close_re.search(text, body_start)
        body_end = m2.start() if m2 else len(text)
        spans.append((body_start, body_end, "js" if tagname == "script" else "css"))
    return spans


def _html_role(text: str, offset: int) -> str:
    """Delegates to _js_role/_css_role for any offset falling inside an
    embedded <script>/<style> body (see _html_embedded_spans) -- an
    editor working inside embedded script needs JS-aware roles, not a
    single undifferentiated "html-text". Otherwise, a whole-text
    forward scan over plain HTML: <!-- --> comments, < > tags with
    attribute values tracked in their own roles (single- vs double-
    quoted, matching the same multi-quote-type granularity
    shell-squote-string/shell-dquote-string already established), and
    plain text content between tags.

    KNOWN LIMITATION, named rather than silently unhandled: inline
    event-handler attributes (onclick="...") and inline style
    attributes (style="...") are NOT delegated to _js_role/_css_role
    -- their content classifies as plain html-attr-dquote/squote, the
    same as any other attribute value. Delegating there too would need
    tracking the attribute NAME while scanning, not just quote state;
    left out deliberately rather than half-built, since the two
    embedding forms this tool exists to serve (external <script src>
    boilerplate aside, real <script>/<style> BODY content) are already
    covered.
    """
    for body_start, body_end, lang in _html_embedded_spans(text):
        if body_start <= offset < body_end:
            sub = text[body_start:body_end]
            rel = offset - body_start
            return _js_role(sub, rel) if lang == "js" else _css_role(sub, rel)
        if offset < body_start:
            break  # spans are in document order; none later can match either
    i, n = 0, len(text)
    state = "text"
    while i < offset:
        c = text[i]
        if state == "text":
            if text[i:i + 4] == "<!--":
                state = "comment"; i += 4; continue
            if c == "<":
                state = "tag"; i += 1; continue
            i += 1; continue
        if state == "comment":
            if text[i:i + 3] == "-->":
                state = "text"; i += 3; continue
            i += 1; continue
        if state == "tag":
            if c == '"':
                state = "attr-dq"; i += 1; continue
            if c == "'":
                state = "attr-sq"; i += 1; continue
            if c == ">":
                state = "text"; i += 1; continue
            i += 1; continue
        if state == "attr-dq":
            if c == '"':
                state = "tag"
            i += 1; continue
        if state == "attr-sq":
            if c == "'":
                state = "tag"
            i += 1; continue
    return {"text": "html-text", "comment": "html-comment", "tag": "html-tag",
            "attr-dq": "html-attr-dquote", "attr-sq": "html-attr-squote"}[state]


def _python_role(text: str, offset: int) -> str:
    """Whole-text forward scan tracking string/comment state. Triple-
    quoted strings can span lines, so (unlike Go's line-local scan)
    this must walk from the start of the file -- O(offset) per call,
    acceptable at source-file scale. f-strings are classified as
    ordinary python-string (no separate role): the {expr} interior of
    an f-string is still Python code, but distinguishing it would need
    real brace-depth tracking inside the string, which the delimiter-
    integrity check in str_replace_extended.py's own precheck already
    covers structurally -- this classifier stays a role heuristic."""
    i, n = 0, len(text)
    state = None  # None | ("str", delimiter)
    while i < offset:
        c = text[i]
        if state is None:
            if text[i:i + 3] in ('"""', "'''"):
                state = ("str", text[i:i + 3])
                i += 3
                continue
            if c in ('"', "'"):
                state = ("str", c)
                i += 1
                continue
            if c == "#":
                nl = text.find("\n", i)
                if nl == -1 or offset <= nl:
                    return "python-comment"
                i = nl + 1
                continue
            i += 1
        else:
            _, delim = state
            if c == "\\" and len(delim) == 1:
                i += 2
                continue
            if text[i:i + len(delim)] == delim:
                i += len(delim)
                state = None
                continue
            i += 1
    return "python-string" if state is not None else "python-code"


def _json_role(text: str, offset: int) -> str:
    """Whole-text forward scan. JSON has exactly one string delimiter
    ("), one escape char, no comments -- the simplest of these."""
    i, in_str = 0, False
    while i < offset:
        c = text[i]
        if in_str:
            if c == "\\":
                i += 2
                continue
            if c == '"':
                in_str = False
            i += 1
            continue
        if c == '"':
            in_str = True
        i += 1
    return "json-string" if in_str else "json-code"


def _yaml_role(text: str, offset: int) -> str:
    """Line-local scan: flow-style quoted scalars ('...'/"...") don't
    span lines, so line-local is correct for them. KNOWN LIMITATION,
    documented rather than silently wrong: block scalars (| and >) are
    NOT modeled -- their body lines classify as yaml-code even though
    they are technically string content. str_replace_extended.py's own
    delimiter-integrity precheck (role-divergence, not delimiter
    tables) still catches most damage from an edit landing inside one,
    since introducing new YAML structure mid-block-scalar changes
    indentation/role for everything after it."""
    line_start = text.rfind("\n", 0, offset) + 1
    line = text[line_start:offset]
    in_sq = in_dq = False
    i = 0
    while i < len(line):
        c = line[i]
        if in_dq:
            if c == "\\":
                i += 2
                continue
            if c == '"':
                in_dq = False
            i += 1
            continue
        if in_sq:
            if c == "'" and line[i:i + 2] == "''":
                i += 2
                continue
            if c == "'":
                in_sq = False
            i += 1
            continue
        if c == '"':
            in_dq = True
            i += 1
            continue
        if c == "'":
            in_sq = True
            i += 1
            continue
        if c == "#":
            return "yaml-comment"
        i += 1
    if in_dq or in_sq:
        return "yaml-string"
    return "yaml-code"


def _shell_role(text: str, offset: int) -> str:
    """Whole-text forward scan: single quotes (no escapes inside),
    double quotes (backslash-escaped), backtick command substitution,
    and # comments (only where # starts a token, not mid-word like
    a literal '#' in an argument). KNOWN LIMITATION: heredocs
    (<<EOF ... EOF) are NOT modeled -- their body misclassifies as
    shell-code. Flag heredoc-containing files for manual review rather
    than trust this classifier there."""
    i, n = 0, len(text)
    state = None  # None | "sq" | "dq" | "bq"
    while i < offset:
        c = text[i]
        if state == "sq":
            if c == "'":
                state = None
            i += 1
            continue
        if state == "dq":
            if c == "\\":
                i += 2
                continue
            if c == '"':
                state = None
            i += 1
            continue
        if state == "bq":
            if c == "\\":
                i += 2
                continue
            if c == "`":
                state = None
            i += 1
            continue
        if c == "'":
            state = "sq"
            i += 1
            continue
        if c == '"':
            state = "dq"
            i += 1
            continue
        if c == "`":
            state = "bq"
            i += 1
            continue
        if c == "#" and (i == 0 or text[i - 1] in " \t\n;|&("):
            nl = text.find("\n", i)
            if nl == -1 or offset <= nl:
                return "shell-comment"
            i = nl + 1
            continue
        i += 1
    return {"sq": "shell-squote-string", "dq": "shell-dquote-string",
            "bq": "shell-backtick-string", None: "shell-code"}[state]


def classify(path: Path, text: str, offset: int) -> str:
    """Role of the occurrence starting at byte offset in text."""
    suffix = path.suffix.lower()
    if suffix == ".go":
        return _go_role(text, offset)
    if suffix in (".md", ".markdown"):
        return _md_role(text, offset)
    if suffix == ".py":
        return _python_role(text, offset)
    if suffix == ".json":
        return _json_role(text, offset)
    if suffix in (".yml", ".yaml"):
        return _yaml_role(text, offset)
    if suffix in (".sh", ".bash"):
        return _shell_role(text, offset)
    if suffix in (".js", ".mjs", ".cjs", ".jsx"):
        return _js_role(text, offset)
    if suffix in (".ts", ".tsx"):
        return _ts_role(text, offset)
    if suffix == ".css":
        return _css_role(text, offset)
    if suffix in (".html", ".htm"):
        return _html_role(text, offset)
    return "text"


def occurrences(term: str, paths, regex: bool = False):
    """Yield (path, offset, end, role, line_no, line_text) for every
    occurrence of term in the given files."""
    pat = re.compile(term if regex else re.escape(term))
    for p in paths:
        p = Path(p)
        try:
            text = p.read_text()
        except (UnicodeDecodeError, OSError):
            continue
        for m in pat.finditer(text):
            line_no = text.count("\n", 0, m.start()) + 1
            ls = text.rfind("\n", 0, m.start()) + 1
            le = text.find("\n", m.start())
            line = text[ls:le if le != -1 else len(text)]
            yield p, m.start(), m.end(), classify(p, text, m.start()), line_no, line


def expand(paths):
    out = []
    for p in paths:
        p = Path(p)
        if p.is_dir():
            out += [f for f in sorted(p.rglob("*"))
                    if f.is_file() and ".git" not in f.parts]
        elif p.is_file():
            out.append(p)
    return out


def main(argv) -> int:
    if len(argv) < 1:
        print(__doc__)
        return 1
    term, paths = argv[0], expand(argv[1:] or ["."])
    by_role = {}
    for p, s, e, role, ln, line in occurrences(term, paths):
        by_role.setdefault(role, []).append((p, ln, line))
        print(f"{p}:{ln}: [{role}] {line.strip()[:90]}")
    if by_role:
        print(f"\nroles present: {sorted(by_role)} "
              f"({sum(len(v) for v in by_role.values())} occurrence(s))")
        if len(by_role) > 1:
            print("MULTIPLE ROLES: a single substitution pass is NOT safe; "
                  "write one targeted pass per role.")
    else:
        print("no occurrences")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
