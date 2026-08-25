# Changelog

All notable changes to zen80 are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [0.5.0] - 2026-08-25

### Added

- **ULA contention hooks**: `ContendedMemDelay` and `ContendedIODelay`,
  optional `func(x uint16, cyclesBefore uint64) int` fields called on
  every memory read/write and port access respectively. Returned delays
  accumulate in a private `pendingContention` folded into the
  instruction's cycle count exactly once by `finishStep`, at every
  `Step()` exit path. Both hooks nil by default: a complete no-op with
  zero overhead for existing callers.
- **Within-instruction access-position tracking** for the hooks:
  `cyclesBefore` is not the instruction-start cycle count but the
  access's estimated true T-state position -- instruction start plus a
  running offset built from per-access base costs and contention
  delays already applied within the same instruction. Base costs are
  opcode-aware via a new `firstMCycleCost` table (fetch 5 for
  DJNZ/PUSH/RST/RET cc/LD SP,HL, 6 for INC/DEC dd, prefixed second
  fetch 4). Measured against real execution (Speedlock loader
  workload, 4.2M instructions), residual position error bounds at
  ~0.33% of total memory-contention delay and ~0% for I/O -- versus
  ~6% and ~20% for the same hooks at plain instruction-start
  positions. Tracking is skipped entirely when both hooks are nil.

## [0.4.0] - 2026-08-24

### Added

- **`FastPortWriteOut`**: a further optional addition alongside
  `FastPort`/`FastPortReadIn`, for a caller whose memory model has state
  driven by a specific port write -- a memory paging register being the
  motivating case. Called on every `OUT` while `FastPort` is active
  (after `FastPort[port]` is already updated), so a caller can react to
  a paging write the instant it happens rather than only discovering it
  later. Mirrors `FastPortReadIn`'s existing design exactly; nil (the
  default) is a complete no-op.

  Motivating case, found via zenzx integration testing: a flat 64K
  `FastMem` snapshot has no way to represent banked memory on its own.
  Reconciling it only periodically (e.g. at a caller-chosen checkpoint)
  is provably insufficient for a program that pages banks *during
  ongoing execution*, not just between well-defined phases -- confirmed
  with a real 128K game (Cybernoid 2) that issues dozens of paging
  writes within a few thousand instructions, including rapid ROM-bank
  toggling. `FastPortWriteOut` lets the caller swap `FastMem`'s affected
  content the moment paging actually changes, which a purely periodic
  reconciliation structurally cannot do correctly.

## [0.3.0] - 2026-08-23

### Added

- **FastMem/FastPort**: an optional addition alongside the existing
  `Memory`/`IO` interface fields, not a replacement -- every existing
  caller is completely unaffected (both fields default to nil, a
  complete no-op). When a caller sets `Z80.FastMem` (a `*[65536]byte`)
  and/or `Z80.FastPort` (a `*[65536]byte`), every memory/IO access in
  the package routes through it directly instead of the `Memory`/`IO`
  interfaces, letting the Go compiler inline a plain array access
  instead of an interface's indirect call. Measured effect: roughly
  3.6x throughput on a representative memory-access workload (up from
  a raw interface-only baseline), closing most of the gap to a
  fully-concrete, no-indirection upper bound. Intended for a caller
  that needs to run a large, closed number of instructions fast and
  correctly with no bank-switching concerns (a full flat 64K address
  space) -- e.g. a tape-loading fast path -- and is willing to
  reconcile FastMem/FastPort against its own real memory model itself;
  zen80 does not manage that reconciliation.
- **FastPortReadIn**: a further optional addition alongside FastPort,
  for callers whose IO model is dynamic in a way a static array can't
  cheaply represent (e.g. a ULA-style port combining keyboard state and
  a frequently-toggling external signal from the port address's high
  byte). Checked first on every IN while FastPort is active, before
  falling back to the flat array; nil (the default) is a complete
  no-op.

### Fixed

- **`decode.go` bypassed FastMem/FastPort entirely for 16 call sites**
  covering some of the most common opcodes in the instruction set
  (`LD A,(HL)`, `LD (HL),n`, `INC (HL)`, `DEC (HL)`, `LD A,(BC)`,
  `LD A,(DE)`, `LD (BC),A`, `LD (DE),A`, `IN A,(n)`, `OUT (n),A`, and
  related indexed forms). Root cause: this file uses `cpu` as its
  receiver variable name where every other file in the package uses
  `z`; the original mechanical substitution introducing `memRead`/
  `memWrite`/`ioIn`/`ioOut` searched specifically for the `z.`-prefixed
  form and silently missed every site in this file. A caller using
  FastMem/FastPort would see memory silently split into two diverging
  views -- most instructions correctly using the fast array, but these
  16 forms always reading and writing the real, interface-backed
  memory regardless. Found via zenzx integration testing: a BASIC
  program's own interpreter loop (which leans on exactly these opcodes
  for variable and stack handling) would silently read stale values,
  eventually stalling program flow entirely. All four helper functions
  (`memRead`/`memWrite`/`ioIn`/`ioOut`) are now confirmed as the sole
  indirection point for every memory/IO access in the package, verified
  by an exhaustive grep across every file for any remaining direct
  `<recv>.Memory.`/`<recv>.IO.` call regardless of receiver name.

### Documentation

- Corrected README's stated ZEXDOC/ZEXALL runtime from "several
  minutes"/"minutes" to the actual 30-60+ minutes, and added the same
  timing note directly on both test functions (`zexdoc_test.go`,
  `zexall_test.go`) plus a note on `TestZEXALL_QuickCheck` that it
  currently reports 0 passing tests when actually run -- flagged, not
  investigated, since it wasn't in scope this session.

## [0.2.0] - 2026-08-21

### Added

- **Z80N (ZX Spectrum Next) extended instruction set**, all 29 opcodes,
  gated behind a new `Z80N bool` field on `Z80` that defaults to false --
  a plain Z80 core's behaviour is completely unaffected unless a caller
  opts in explicitly. Implemented in three stages:
  - **Dispatch surgery**: the 29 opcodes were carved out of their
    previous NOP-catch-all case blocks in `prefix_ed.go` into their own
    named dispatch entries, gated on `z.Z80N`, calling into new bodies in
    `z80n.go`. Verified both directions: `TestZ80N_OffByDefault_ByteFor
    ByteUnchanged` confirms a Z80N=false core's behaviour is genuinely
    identical to before the surgery, and `TestZ80N_On_DispatchReaches
    CorrectHandler` confirms each opcode routes to its own handler, not
    a neighbour's.
  - **~25 mechanical opcodes**: `SWAPNIB`, `MIRROR A`, `TEST $im8`, the
    barrel shift/rotate set (`BSLA`/`BSRA`/`BSRL`/`BSRF`/`BRLC DE,B`),
    `MUL D,E`, `ADD HL/DE/BC,A` and `,$im16`, `OUTINB`, `PIXELDN`,
    `PIXELAD`, `SETAE`, and the extended block-copy set (`LDIX`, `LDWS`,
    `LDDX`, `LDIRX`, `LDPIRX`, `LDDRX`), each implemented directly from
    the SpecNext wiki's documented formulas.
  - **3 genuine edge cases**: `PUSH $im16` (the only operand in the whole
    Z80/Z80N set encoded big-endian in the instruction stream -- fetched
    explicitly in that order rather than through the package's own
    little-endian `fetchWord()`, with a test that would catch a
    byte-swap regression, confirmed by deliberately introducing one and
    watching it fail before restoring the fix); `NEXTREG $im8,$im8` /
    `NEXTREG $im8,A` (implemented as the two port writes -- register
    select via `0x243B`, data via `0x253B` -- real hardware's own
    observable equivalent, cross-checked against four independent
    sources before trusting the port numbers); `JP (C)` (the one
    instruction with no classic-Z80 shape at all, an I/O read feeding
    directly into PC -- `PC := (PC & $C000) | (IN(C) << 6)`, confirmed
    from the wiki's own two independently-phrased statements of the
    formula, tested specifically for reading the full 16-bit `BC` as the
    port address rather than just `C`, and for preserving PC's top bits
    correctly at a non-zero starting address).

### Fixed

- **`ADD HL/DE/BC,A` (`ED 31`/`32`/`33`) no longer clears the carry
  flag.** The three opcodes' first implementation set `C:=0`
  unconditionally, following the SpecNext wiki's 2025-01-25 hardware-test
  note ("most probably always reset"), which had itself superseded an
  older 2021-09-16 wiki note. Both turned out to be wrong: a real
  ground-truth test (`SCF` then `ADD HL,A`, run to completion on genuine
  CSpect via NextZXOS with the result read back by screen OCR, and
  separately on this package's own core embedded in ZenZX headless, with
  the result confirmed stable across multiple different frame counts
  after the same load rather than read once) showed carry left
  unchanged in both cases. `zesarux` (`instruccion_ed_49`,
  `z80_codpred.c`) had implemented it as unchanged all along --
  unmodified since that file's first commit (2022-03-04), nearly three
  years before either wiki note existed to contradict it -- and turns
  out to have been right, not merely first. See `z80n.go`'s own comment
  on `z80nAddHLA` for the full trail, and `Z80N_ZESARUX_CROSSCHECK.md`
  for the wider cross-checking write-up this fix came out of.

### Testing

- `z80n_test.go`: dispatch-correctness harness (both directions --
  off-by-default and on-routes-correctly).
- `z80n_semantics_test.go`: one test per opcode against documented
  behaviour, including the specific corrections noted above.
- `z80n_adversarial_test.go`: edge cases picked to distinguish a correct
  implementation from a plausible-looking wrong one where two
  possible bugs could otherwise hide behind each other (wraparound
  arithmetic, maximum-magnitude multiply, R-register increments
  interleaved with classic instructions, zero-shift-count no-ops, shift
  mask boundaries, BC-starts-at-zero block copies).
- `Z80N_ZESARUX_CROSSCHECK.md`: the cross-checking methodology and
  findings for every opcode where the wiki's own documentation left
  genuine doubt, not a routine per-opcode audit.

## [0.1.0] - 2026-06-26

First versioned release. Establishes the release-hygiene baseline (VERSION
file, `pkg/version` package, `syncver.sh`, `release.sh`, this changelog) for
the existing zen80 codebase.

### Emulator core

- Instruction-stepped Z80 CPU core with full documented instruction set:
  main, CB-prefixed, ED-prefixed, DD/FD-prefixed (IX/IY), and DDCB/FDCB
  indexed bit operations.
- Accurate flag handling including the undocumented X and Y flags, and the
  internal WZ (MEMPTR) register.
- Interrupt support: NMI (edge-triggered) and maskable interrupts in modes
  0, 1, and 2, with a two-phase Mode 0 instruction-injection buffer and an
  optional `InterruptController` interface for vector and Mode 0 supply.
- Correct R-register refresh increment on the M1 cycle.

### Memory and I/O

- `memory` package: `RAM` (64 KB), `ROM`, and `MappedMemory` (ROM-low /
  RAM-high with power-of-two ROM mirroring).
- `io` package: `NullIO`, `SimpleIO` (256-port array), and `MappedIO`
  (per-port and 8-bit-decode fallback handlers).

### System layer

- `system` package providing a ZX Spectrum system: 48 KB memory map, ULA
  keyboard/border/speaker I/O, frame timing, and vertical-blank interrupt
  generation.

### Tooling

- `pkg/version` package exposing the `Version` constant, kept in sync with
  the root `VERSION` file via `syncver.sh`.
- `release.sh` single-pass release preparation: version validation,
  CHANGELOG check, version sync, build, single test pass with coverage, a
  version-consistency check, and a binary-free, artifact-free checkpoint zip.

### Documentation

- `runtest.sh` now skips the ZEXDOC/ZEXALL exercisers by default (they have
  their own runners and take minutes); pass `--zex` to include them.
- Expanded the README testing section to document the three test tiers (fast
  unit tests, the ROM-backed opcode-coverage test, and the ZEXDOC/ZEXALL
  conformance exercisers), including the environment variables that gate the
  exercisers and why `go test` alone triggers a long run.

[0.1.0]: https://github.com/ha1tch/zen80/releases/tag/v0.1.0
