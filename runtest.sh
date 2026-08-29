#!/bin/bash
# runtest.sh - Run the fast test suite plus the ROM-backed opcode-coverage test.
#
# ZEXDOC/ZEXALL live in tools/zex, not here, so this never picks them up --
# use their own runners, zexdoc.sh and zexall.sh, for those.

rompath=$(realpath "./rom/128-0.rom")

export Z80_ROM_PATH="$rompath"
export Z80_ROM_STEPS="50000000"

go test ./z80 -v -count=1
