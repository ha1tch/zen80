#!/bin/bash
# runtest.sh - Run the fast test suite plus the ROM-backed opcode-coverage test.
#
# This skips the ZEXDOC/ZEXALL conformance exercisers by default: they run to
# completion (billions of cycles, several minutes) and have their own runners,
# zexdoc.sh and zexall.sh. To include them here, pass --zex.

rompath=$(realpath "./rom/128-0.rom")

export Z80_ROM_PATH="$rompath"
export Z80_ROM_STEPS="50000000"

SKIP="-skip ZEX|ZEXALL"
for arg in "$@"; do
    case "$arg" in
        --zex) SKIP="" ;;  # include the ZEXDOC/ZEXALL exercisers
    esac
done

go test ./z80 -v -count=1 $SKIP
