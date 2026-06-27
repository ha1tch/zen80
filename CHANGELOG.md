# Changelog

All notable changes to zen80 are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

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
